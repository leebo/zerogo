package agent

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/unicornultrafoundation/zerogo/internal/identity"
	"github.com/unicornultrafoundation/zerogo/internal/tap"
	"github.com/unicornultrafoundation/zerogo/internal/vl1"
	"github.com/unicornultrafoundation/zerogo/internal/vl2"
)

// Agent is the main client daemon orchestrating VL1 transport, VL2 switching, and TAP devices.
type Agent struct {
	config    Config
	identity  *identity.Identity
	transport *vl1.Transport
	peers     *vl1.PeerManager
	network   *vl2.Network
	tapDev    tap.Device
	ctrlCli   *ControllerClient
	log       *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a new Agent instance.
func New(cfg Config, log *slog.Logger) (*Agent, error) {
	id, err := identity.LoadOrGenerate(cfg.IdentityPath)
	if err != nil {
		return nil, fmt.Errorf("load identity: %w", err)
	}
	log.Info("identity loaded", "address", id.Address, "pubkey", id.PublicKeyHex()[:16]+"...")

	ctx, cancel := context.WithCancel(context.Background())
	return &Agent{
		config:   cfg,
		identity: id,
		peers:    vl1.NewPeerManager(log),
		log:      log,
		ctx:      ctx,
		cancel:   cancel,
	}, nil
}

// Start initializes all subsystems and begins processing.
func (a *Agent) Start() error {
	// 1. Start VL1 UDP transport
	transport, err := vl1.NewTransport(a.config.ListenPort, a.log)
	if err != nil {
		return fmt.Errorf("start transport: %w", err)
	}
	a.transport = transport

	// Controller mode: connect to controller, TAP will be created on NetworkConfig
	if a.config.ControllerURL != "" {
		a.ctrlCli = NewControllerClient(a.config.ControllerURL, a, a.log)

		// Start goroutines (no TAP read loop yet, will start on network config)
		a.wg.Add(2)
		go a.udpReadLoop()
		go a.maintenanceLoop()

		// Start controller connection in background
		a.wg.Add(1)
		go func() {
			defer a.wg.Done()
			a.ctrlCli.Run(a.ctx)
		}()

		a.log.Info("agent started (controller mode)",
			"address", a.identity.Address,
			"port", a.transport.Port(),
			"controller", a.config.ControllerURL,
		)
		return nil
	}

	// Static peer mode: create TAP immediately
	// 2. Create TAP device
	tapDev, err := tap.NewLinuxTAP(a.config.TAPName)
	if err != nil {
		a.transport.Close()
		return fmt.Errorf("create TAP device: %w", err)
	}
	a.tapDev = tapDev
	a.log.Info("TAP device created", "name", tapDev.Name())

	// 3. Configure TAP: set MTU, MAC, IP
	mtu := a.config.TAPMTU
	if mtu == 0 {
		mtu = 2800
	}
	if err := tapDev.SetMTU(mtu); err != nil {
		a.log.Warn("set TAP MTU failed", "err", err)
	}

	// 4. Create VL2 network with virtual switch
	netConfig := vl2.NetworkConfig{
		ID:        a.config.NetworkID,
		Name:      "default",
		MTU:       mtu,
		Multicast: true,
	}
	a.network = vl2.NewNetwork(netConfig, a.identity.Address, a, a.log)

	// Set MAC address on TAP
	mac := vl2.GenerateMAC(a.config.NetworkID, a.identity.Address)
	if err := tapDev.SetMACAddress(mac); err != nil {
		a.log.Warn("set TAP MAC failed", "err", err)
	}

	// Set IP address on TAP
	if a.config.TAPIPv4 != "" {
		ip, ipNet, err := net.ParseCIDR(a.config.TAPIPv4)
		if err != nil {
			a.log.Warn("invalid TAP IP", "err", err)
		} else {
			if err := tapDev.AddIPAddress(ip, ipNet.Mask); err != nil {
				a.log.Warn("add TAP IP failed", "err", err)
			}
		}
	}

	// Bring interface up
	if err := tapDev.SetUp(); err != nil {
		a.log.Warn("bring TAP up failed", "err", err)
	}

	// 5. Add static peers and initiate handshakes
	for _, sp := range a.config.StaticPeers {
		endpoint, err := net.ResolveUDPAddr("udp", sp.Address)
		if err != nil {
			a.log.Error("resolve peer endpoint", "addr", sp.Address, "err", err)
			continue
		}
		pubKeyBytes, err := hex.DecodeString(sp.PublicKey)
		if err != nil {
			a.log.Error("decode peer public key", "err", err)
			continue
		}
		var pubKey [32]byte
		copy(pubKey[:], pubKeyBytes)
		peerAddr := identity.AddressFromPublicKey(pubKey[:])

		peer := a.peers.AddPeer(peerAddr, pubKey, endpoint)
		a.initiateHandshake(peer)
	}

	// 6. Start goroutines
	a.wg.Add(3)
	go a.tapReadLoop()
	go a.udpReadLoop()
	go a.maintenanceLoop()

	a.log.Info("agent started",
		"address", a.identity.Address,
		"port", a.transport.Port(),
		"tap", tapDev.Name(),
		"peers", len(a.config.StaticPeers),
	)
	return nil
}

// Stop gracefully shuts down the agent.
func (a *Agent) Stop() {
	a.log.Info("agent stopping...")
	a.cancel()
	if a.transport != nil {
		a.transport.Close()
	}
	if a.tapDev != nil {
		a.tapDev.Close()
	}
	a.wg.Wait()
	a.log.Info("agent stopped")
}

// Identity returns the agent's identity.
func (a *Agent) Identity() *identity.Identity {
	return a.identity
}

// --- Goroutine loops ---

// tapReadLoop reads Ethernet frames from the TAP device and forwards via VL2 switch.
func (a *Agent) tapReadLoop() {
	defer a.wg.Done()
	buf := make([]byte, vl2.MaxFrameSize)
	for {
		select {
		case <-a.ctx.Done():
			return
		default:
		}
		n, err := a.tapDev.Read(buf)
		if err != nil {
			if a.ctx.Err() != nil {
				return
			}
			a.log.Error("TAP read error", "err", err)
			continue
		}
		if n < vl2.MinFrameSize {
			continue
		}
		// Process through ARP proxy first
		frame, err := vl2.ParseEthernetFrame(buf[:n])
		if err != nil {
			continue
		}
		if frame.IsARP() {
			if reply := a.network.ARP.HandleARP(frame); reply != nil {
				// Inject ARP reply directly into TAP (no need to send to network)
				a.tapDev.Write(reply)
				continue
			}
		}

		// Forward through virtual switch
		frameCopy := make([]byte, n)
		copy(frameCopy, buf[:n])
		a.log.Debug("TAP frame read", "len", n, "dst", frame.DstMAC, "src", frame.SrcMAC, "type", fmt.Sprintf("0x%04x", frame.EtherType))
		if err := a.network.Switch.HandleLocalFrame(frameCopy); err != nil {
			a.log.Debug("switch handle local frame", "err", err)
		}
	}
}

// udpReadLoop reads VL1 packets from the UDP transport.
func (a *Agent) udpReadLoop() {
	defer a.wg.Done()
	buf := make([]byte, vl1.MaxPacketSize)
	for {
		select {
		case <-a.ctx.Done():
			return
		default:
		}
		n, remoteAddr, err := a.transport.ReadFrom(buf)
		if err != nil {
			if a.ctx.Err() != nil {
				return
			}
			a.log.Error("UDP read error", "err", err)
			continue
		}
		a.handleUDPPacket(buf[:n], remoteAddr)
	}
}

// handleUDPPacket processes an incoming VL1 packet.
func (a *Agent) handleUDPPacket(data []byte, from *net.UDPAddr) {
	pkt, err := vl1.DecodePacket(data)
	if err != nil {
		a.log.Debug("decode packet", "err", err, "from", from)
		return
	}

	switch pkt.Header.Type {
	case vl1.PacketTypeHandshake:
		a.handleHandshake(pkt.Payload, from)

	case vl1.PacketTypeData:
		a.handleDataPacket(pkt, from)

	case vl1.PacketTypeKeepalive:
		// Find peer and touch
		if peer := a.peers.GetPeerByEndpoint(from); peer != nil {
			peer.Touch()
		}

	default:
		a.log.Debug("unknown packet type", "type", pkt.Header.Type, "from", from)
	}
}

// handleHandshake processes a handshake/hello message from a peer.
func (a *Agent) handleHandshake(payload []byte, from *net.UDPAddr) {
	// Hello format: [pubkey(32 bytes)]
	if len(payload) < 32 {
		a.log.Debug("handshake too short", "len", len(payload), "from", from)
		return
	}

	var remotePubKey [32]byte
	copy(remotePubKey[:], payload[:32])

	remoteAddr := identity.AddressFromPublicKey(remotePubKey[:])

	// Find existing peer
	peer := a.peers.GetPeer(remoteAddr)
	if peer != nil {
		// Update endpoint and touch — keys are already derived
		peer.Endpoint = from
		peer.Touch()

		// If not yet connected, derive keys now
		if !peer.IsConnected() {
			sendKey, recvKey := vl1.DeriveKeysFromPSK(a.config.PSK, a.identity.PublicKey, remotePubKey)
			cipher := vl1.NewNoiseCipher(sendKey, recvKey)
			peer.SetCipher(cipher)
			a.log.Info("peer connected via PSK handshake", "peer", peer.Address, "endpoint", from)
		}
		return
	}

	// Unknown peer sending hello — create and connect
	peer = a.peers.AddPeer(remoteAddr, remotePubKey, from)
	sendKey, recvKey := vl1.DeriveKeysFromPSK(a.config.PSK, a.identity.PublicKey, remotePubKey)
	cipher := vl1.NewNoiseCipher(sendKey, recvKey)
	peer.SetCipher(cipher)
	a.log.Info("new peer connected via PSK handshake", "peer", peer.Address, "endpoint", from)

	// Send hello back so the remote side learns our endpoint
	a.sendHello(peer)
}

// handleDataPacket processes an encrypted data packet.
func (a *Agent) handleDataPacket(pkt *vl1.Packet, from *net.UDPAddr) {
	peer := a.peers.GetPeerByEndpoint(from)
	if peer == nil {
		a.log.Debug("data from unknown peer", "from", from)
		return
	}
	peer.Touch()

	// Decrypt payload
	plaintext, err := peer.Decrypt(pkt.Payload)
	if err != nil {
		a.log.Debug("decrypt failed", "peer", peer.Address, "err", err, "payload_len", len(pkt.Payload))
		return
	}

	a.log.Debug("received encrypted frame", "peer", peer.Address, "frame_len", len(plaintext))

	// Check if network is ready
	if a.network == nil {
		a.log.Debug("network not ready, dropping frame")
		return
	}

	// Process through VL2 switch
	frameToInject, err := a.network.Switch.HandleRemoteFrame(peer.Address, plaintext)
	if err != nil {
		a.log.Debug("switch handle remote frame", "err", err)
		return
	}

	// Inject into TAP device
	if frameToInject != nil && a.tapDev != nil {
		if _, err := a.tapDev.Write(frameToInject); err != nil {
			a.log.Error("TAP write error", "err", err)
		}
		a.log.Debug("injected frame into TAP", "len", len(frameToInject))
	}
}

// sendHello sends a hello handshake packet carrying our public key.
func (a *Agent) sendHello(peer *vl1.Peer) {
	// Hello payload: our public key (32 bytes)
	pkt := vl1.NewHandshakePacket(a.identity.PublicKey[:])
	if err := a.transport.SendPacket(pkt, peer.Endpoint); err != nil {
		a.log.Debug("send hello failed", "peer", peer.Address, "err", err)
		return
	}
	peer.LastSend = time.Now()
	a.log.Info("hello sent", "peer", peer.Address, "endpoint", peer.Endpoint)
}

// initiateHandshake starts the PSK key exchange with a peer.
func (a *Agent) initiateHandshake(peer *vl1.Peer) {
	// Derive keys immediately from PSK (deterministic, no round-trip needed)
	sendKey, recvKey := vl1.DeriveKeysFromPSK(a.config.PSK, a.identity.PublicKey, peer.PublicKey)
	cipher := vl1.NewNoiseCipher(sendKey, recvKey)
	peer.SetCipher(cipher)

	// Send hello so remote side knows our endpoint and can derive matching keys
	a.sendHello(peer)
}

// maintenanceLoop runs periodic maintenance tasks.
func (a *Agent) maintenanceLoop() {
	defer a.wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			// Send keepalives
			for _, peer := range a.peers.ConnectedPeers() {
				if peer.NeedsKeepalive() {
					pkt := vl1.NewKeepalivePacket()
					if err := a.transport.SendPacket(pkt, peer.Endpoint); err != nil {
						a.log.Debug("keepalive send failed", "peer", peer.Address, "err", err)
					}
					peer.LastSend = time.Now()
				}
			}

			// Re-send hello for peers that aren't connected yet
			for _, peer := range a.peers.AllPeers() {
				if !peer.IsConnected() {
					a.sendHello(peer)
				}
			}

			// Clean expired MAC entries
			if a.network != nil {
				a.network.Switch.CleanExpired()
				a.network.ARP.CleanExpired()
			}

			// Send status to controller
			if a.ctrlCli != nil {
				a.ctrlCli.SendStatus()
			}
		}
	}
}

// --- PeerSender interface implementation ---

// SendToPeer sends an encrypted Ethernet frame to a specific peer.
func (a *Agent) SendToPeer(peerAddr identity.Address, networkID uint32, frame []byte) error {
	peer := a.peers.GetPeer(peerAddr)
	if peer == nil {
		return fmt.Errorf("unknown peer: %s", peerAddr)
	}
	if !peer.IsConnected() {
		return fmt.Errorf("peer not connected: %s", peerAddr)
	}

	encrypted, err := peer.Encrypt(frame)
	if err != nil {
		return err
	}

	pkt := vl1.NewDataPacket(networkID, encrypted)
	return a.transport.SendPacket(pkt, peer.Endpoint)
}

// BroadcastToPeers sends an encrypted Ethernet frame to all connected peers in the network.
func (a *Agent) BroadcastToPeers(networkID uint32, frame []byte, excludePeer identity.Address) error {
	for _, peer := range a.peers.ConnectedPeers() {
		if peer.Address == excludePeer {
			continue
		}
		encrypted, err := peer.Encrypt(frame)
		if err != nil {
			a.log.Debug("encrypt for broadcast", "peer", peer.Address, "err", err)
			continue
		}
		pkt := vl1.NewDataPacket(networkID, encrypted)
		if err := a.transport.SendPacket(pkt, peer.Endpoint); err != nil {
			a.log.Debug("broadcast send", "peer", peer.Address, "err", err)
		}
	}
	return nil
}
