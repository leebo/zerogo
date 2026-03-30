package agent

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"runtime"

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
	localIPv4 [4]byte    // our assigned IPv4, used to detect TUN bounce-back
	localNet  *net.IPNet // VPN subnet, used to distinguish bounce-back from forwarded traffic

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

	// Protect UDP socket from VPN routing (Android)
	if a.config.SocketProtect != nil {
		transport.SocketProtect = a.config.SocketProtect
		if err := transport.ProtectSocket(); err != nil {
			a.log.Warn("protect socket failed", "err", err)
		}
	}

	// Apply socket buffer tuning
	if a.config.RcvBuf > 0 || a.config.SndBuf > 0 {
		if err := transport.SetSocketBuffers(a.config.RcvBuf, a.config.SndBuf); err != nil {
			a.log.Warn("set socket buffers failed", "err", err)
		} else {
			a.log.Info("socket buffers configured", "rcvbuf", a.config.RcvBuf, "sndbuf", a.config.SndBuf)
		}
	}

	// Apply DSCP marking
	if a.config.DSCP > 0 {
		if err := transport.SetDSCP(a.config.DSCP); err != nil {
			a.log.Warn("set DSCP failed", "err", err)
		} else {
			a.log.Info("DSCP marking configured", "dscp", a.config.DSCP)
		}
	}

	if a.config.Gaming {
		a.log.Info("gaming optimization mode enabled",
			"dscp", a.config.DSCP,
			"sndbuf", a.config.SndBuf,
			"rcvbuf", a.config.RcvBuf,
		)
	}

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

	// Static peer mode: create TAP/TUN device
	var tapDev tap.Device
	switch runtime.GOOS {
	case "darwin":
		tapDev, err = tap.NewTUN(a.config.TAPName)
	case "android":
		tapDev, err = tap.NewTUNFromFD(a.config.TUNFD, a.config.TAPName)
	default:
		tapDev, err = tap.NewTAP(a.config.TAPName)
	}
	if err != nil {
		a.transport.Close()
		return fmt.Errorf("create network device: %w", err)
	}
	a.tapDev = tapDev
	a.log.Info("network device created", "name", tapDev.Name(), "tun", tapDev.IsTUN())

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

			// Save local IPv4 and subnet for TUN bounce-back detection
			if ip4 := ip.To4(); ip4 != nil {
				copy(a.localIPv4[:], ip4)
				a.localNet = ipNet
			}

			// For TUN devices (macOS), seed ARP cache with our own IP→MAC so we
			// can respond to ARP requests from remote peers.  Without this, Linux
			// peers will never learn our MAC and their ICMP replies will be dropped.
			if tapDev.IsTUN() {
				a.network.ARP.Learn(ip, mac)
				a.log.Info("TUN ARP cache seeded", "ip", ip, "mac", mac)
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
		if a.config.Gaming {
			peer.KeepaliveInterval = vl1.GamingKeepaliveInterval
		}
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

	// Clean managed routes before closing the device
	if a.ctrlCli != nil {
		a.ctrlCli.cleanupRoutes()
	}

	// Close TAP/TUN first to unblock tapReadLoop
	if a.tapDev != nil {
		a.tapDev.Close()
	}
	// Close all ICE connections
	for _, peer := range a.peers.AllPeers() {
		if peer.HasICE() {
			peer.CloseICE()
		}
	}
	if a.transport != nil {
		a.transport.Close()
	}

	// Wait for goroutines with timeout
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		a.log.Info("agent stopped")
	case <-time.After(5 * time.Second):
		a.log.Warn("agent stop timeout, some goroutines may not have terminated")
	}
}

// Identity returns the agent's identity.
func (a *Agent) Identity() *identity.Identity {
	return a.identity
}

// Peers returns the agent's peer manager.
func (a *Agent) Peers() *vl1.PeerManager {
	return a.peers
}

// Config returns the agent's configuration.
func (a *Agent) Config() Config {
	return a.config
}

// SetTUNFD replaces the underlying TUN file descriptor at runtime.
// This is used on Android when the VPN session is rebuilt (e.g. after a
// route change) and VpnService.Builder.establish() returns a new fd.
// The old fd is closed automatically.
func (a *Agent) SetTUNFD(fd int) error {
	if a.tapDev == nil {
		return fmt.Errorf("no TUN device to update")
	}
	type fdUpdater interface {
		UpdateFD(fd int) error
	}
	if u, ok := a.tapDev.(fdUpdater); ok {
		return u.UpdateFD(fd)
	}
	return fmt.Errorf("TUN device does not support fd replacement")
}

// injectFrame writes a frame into the local TAP/TUN device.
// For TUN devices, ARP frames are intercepted and replied to via the switch
// since TUN devices cannot handle Layer 2 ARP.
func (a *Agent) injectFrame(frame []byte) {
	if a.tapDev == nil || a.network == nil {
		return
	}

	if a.tapDev.IsTUN() {
		parsed, err := vl2.ParseEthernetFrame(frame)
		if err == nil && parsed.IsARP() {
			// Handle ARP locally: if we have the answer, send reply back through switch
			if reply := a.network.ARP.HandleARP(parsed); reply != nil {
				if err := a.network.Switch.HandleLocalFrame(reply); err != nil {
					a.log.Debug("ARP reply via switch", "err", err)
				}
			}
			return // Don't inject ARP into TUN
		}
	}

	if _, err := a.tapDev.Write(frame); err != nil {
		a.log.Error("TAP write error", "err", err)
	}
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
			// Brief sleep to prevent 100% CPU spin if the device returns
			// persistent errors (e.g. misconfigured utun on macOS).
			time.Sleep(time.Millisecond)
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
			// Extract peer IP→MAC from the ARP frame so we can proactively
			// populate the kernel ARP table below.
			peerIP, peerMAC := a.network.ARP.PeerFromARP(frame)
			if reply := a.network.ARP.HandleARP(frame); reply != nil {
				// Inject ARP reply directly into TAP (no need to send to network)
				a.tapDev.Write(reply)
				continue
			}
			// On Linux the kernel does not reliably learn MAC addresses from
			// ARP replies written to the TAP fd (TUN/TAP socket behavior).
			// Proactively populate the kernel ARP table so the kernel can send
			// IP packets to this peer without ARPING again.
			if peerIP != nil && peerMAC != nil {
				_ = a.tapDev.SetPeerARP(peerIP, peerMAC)
			}
		}

		if a.tapDev.IsTUN() {
			// Drop kernel bounce-back packets: when we inject a remote peer's packet
			// into TUN and the kernel routes it back through the same TUN interface.
			// Only drop packets whose src IP is within the VPN subnet but not our own —
			// those are true bounce-backs. Packets from outside the VPN subnet
			// (e.g. 192.168.1.x from a LAN behind a gateway node) are legitimate
			// forwarded traffic and must NOT be dropped.
			// Minimum: 14 Ethernet + 20 IPv4 = 34 bytes for complete IPv4 header
			if frame.EtherType == vl2.EtherTypeIPv4 && n >= 34 {
				srcIP := net.IP(buf[26:30]) // IPv4 src at offset 12 in IP header + 14 Ethernet
				var srcArr [4]byte
				copy(srcArr[:], srcIP)
				if a.localIPv4 != [4]byte{} && srcArr != a.localIPv4 {
					if a.localNet != nil && a.localNet.Contains(srcIP) {
						continue // bounce-back: src is in VPN subnet but not us
					}
					// src is outside VPN subnet → forwarded traffic from gateway, allow
				}
			}

			// Resolve broadcast dst MAC to unicast using ARP cache.
			// TUN wraps all packets as broadcast, but we should send unicast when possible
			// to avoid flooding and duplicate replies.
			if frame.IsBroadcast() && n >= 34 {
				if frame.EtherType == vl2.EtherTypeIPv4 {
					dstIP := net.IP(buf[30:34]) // IPv4 dst at offset 16 in IP header + 14 Ethernet
					if mac := a.network.ARP.Lookup(dstIP); mac != nil {
						copy(buf[0:6], mac) // Rewrite dst MAC to unicast
					} else if a.ctrlCli != nil {
						// Destination not in ARP cache — check managed routes.
						// If the destination falls within a managed route, use
						// the gateway peer's MAC for unicast delivery.
						if mac := a.ctrlCli.LookupGatewayMAC(dstIP); mac != nil {
							copy(buf[0:6], mac)
						}
					}
				}
			}
		}

		// Forward through virtual switch
		frameBuf := vl2.GetFrameBuf()
		frameCopy := (*frameBuf)[:n]
		copy(frameCopy, buf[:n])
		if a.log.Enabled(a.ctx, slog.LevelDebug) {
			a.log.Debug("TAP frame read", "len", n, "dst", frame.DstMAC, "src", frame.SrcMAC, "type", fmt.Sprintf("0x%04x", frame.EtherType))
		}
		// Ensure buffer is returned even on error
		if err := a.network.Switch.HandleLocalFrame(frameCopy); err != nil {
			if a.log.Enabled(a.ctx, slog.LevelDebug) {
				a.log.Debug("switch handle local frame", "err", err)
			}
		}
		vl2.PutFrameBuf(frameBuf)
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
			time.Sleep(time.Millisecond)
			continue
		}
		a.handleUDPPacket(buf[:n], remoteAddr)
	}
}

// handleUDPPacket processes an incoming VL1 packet.
func (a *Agent) handleUDPPacket(data []byte, from *net.UDPAddr) {
	var pkt vl1.Packet
	if err := vl1.DecodePacketInto(&pkt, data); err != nil {
		a.log.Debug("decode packet", "err", err, "from", from)
		return
	}

	switch pkt.Header.Type {
	case vl1.PacketTypeHandshake:
		a.handleHandshake(pkt.Payload, from)

	case vl1.PacketTypeData:
		a.handleDataPacket(&pkt, from)

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
		a.peers.UpdatePeerEndpoint(remoteAddr, from)
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
	if a.config.Gaming {
		peer.KeepaliveInterval = vl1.GamingKeepaliveInterval
	}
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

	// Decrypt payload into a pool buffer
	bufp := vl1.GetPacketBuf()
	defer vl1.PutPacketBuf(bufp)
	plaintext, err := peer.DecryptTo(*bufp, pkt.Payload)
	if err != nil {
		a.log.Debug("decrypt failed", "peer", peer.Address, "err", err, "payload_len", len(pkt.Payload))
		return
	}

	if a.log.Enabled(a.ctx, slog.LevelDebug) {
		a.log.Debug("received encrypted frame", "peer", peer.Address, "frame_len", len(plaintext))
	}

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

	// Populate kernel ARP table for this peer so the kernel can send
	// IP packets back without ARPING. Extract IP+MAC from the Ethernet frame.
	if len(plaintext) >= 34 {
		srcMAC := net.HardwareAddr(plaintext[6:12])
		srcIP := net.IP(plaintext[26:30])
		if srcIP.To4() != nil {
			_ = a.tapDev.SetPeerARP(srcIP, srcMAC)
		}
	}

	// Inject into TAP/TUN device
	if frameToInject != nil {
		a.injectFrame(frameToInject)
		if a.log.Enabled(a.ctx, slog.LevelDebug) {
			a.log.Debug("injected frame into TAP", "len", len(frameToInject))
		}
	}
}

// sendHello sends a hello handshake packet carrying our public key.
func (a *Agent) sendHello(peer *vl1.Peer) {
	// Hello payload: our public key (32 bytes)
	pkt := vl1.NewHandshakePacket(a.identity.PublicKey[:])
	encoded := pkt.Encode()

	// Prefer ICE connection if available
	if iceConn := peer.ICEConn(); iceConn != nil {
		if _, err := iceConn.Write(encoded); err != nil {
			a.log.Debug("send hello via ICE failed", "peer", peer.Address, "err", err)
			return
		}
		peer.LastSend = time.Now()
		a.log.Info("hello sent via ICE", "peer", peer.Address)
		return
	}

	if peer.Endpoint == nil {
		a.log.Debug("send hello skipped: no endpoint", "peer", peer.Address)
		return
	}

	if err := a.transport.SendTo(encoded, peer.Endpoint); err != nil {
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
					encoded := pkt.Encode()
					if iceConn := peer.ICEConn(); iceConn != nil {
						if _, err := iceConn.Write(encoded); err != nil {
							a.log.Debug("ICE keepalive failed", "peer", peer.Address, "err", err)
						}
					} else if peer.Endpoint != nil {
						if err := a.transport.SendTo(encoded, peer.Endpoint); err != nil {
							a.log.Debug("keepalive send failed", "peer", peer.Address, "err", err)
						}
					}
					peer.LastSend = time.Now()
				}
			}

			// Re-send hello for peers that aren't connected yet
			for _, peer := range a.peers.AllPeers() {
				if !peer.IsConnected() && !peer.HasICE() {
					a.sendHello(peer)
				}
			}

			// Detect and handle dead peers
			for _, peer := range a.peers.AllPeers() {
				if peer.IsConnected() && !peer.IsAlive() {
					a.log.Warn("peer timed out", "peer", peer.Address,
						"last_seen", time.Since(peer.LastSeen).Round(time.Second))
					wasICE := peer.HasICE()
					peer.MarkDead()
					// If using ICE, close the connection so it can be re-established
					if wasICE {
						peer.CloseICE()
					}
				}
			}

			// Re-initiate ICE for peers that lost their connection (before CleanDead removes them)
			if a.ctrlCli != nil && a.ctrlCli.nat != nil {
				for _, peer := range a.peers.AllPeers() {
					if !peer.IsConnected() && !peer.HasICE() && peer.PublicKey != [32]byte{} {
						remoteNodeAddr := peer.Address.String()
						if _, pending := a.ctrlCli.pendingICE.Load(remoteNodeAddr); !pending {
							a.log.Info("re-initiating ICE for disconnected peer", "peer", remoteNodeAddr)
							a.ctrlCli.initiateICE(peer.Address, remoteNodeAddr, a.config.PSK)
						}
					}
				}
			}

			a.peers.CleanDead()

			// Clean expired MAC entries
			if a.network != nil {
				a.network.Switch.CleanExpired()
				a.network.ARP.CleanExpired()
			}

			// Clean stale ICE sessions
			if a.ctrlCli != nil {
				a.ctrlCli.CleanStaleICE()
			}

			// Send status to controller
			if a.ctrlCli != nil {
				a.ctrlCli.SendStatus()
			}
		}
	}
}

// iceReadLoop reads VL1 packets from an ICE connection for a specific peer.
func (a *Agent) iceReadLoop(peer *vl1.Peer, conn net.Conn) {
	defer a.wg.Done()
	buf := make([]byte, vl1.MaxPacketSize)
	for {
		select {
		case <-a.ctx.Done():
			return
		default:
		}
		n, err := conn.Read(buf)
		if err != nil {
			if a.ctx.Err() != nil {
				return
			}
			a.log.Debug("ICE read error", "peer", peer.Address, "err", err)
			// Only close the peer's ICE if our conn is still the active one.
			// If the peer reconnected, a new iceReadLoop is running on a new conn,
			// and we must not close that new connection.
			if peer.ICEConn() == conn {
				peer.CloseICE()
			}
			return
		}
		a.log.Debug("ICE read packet", "peer", peer.Address, "len", n, "connected", peer.IsConnected())
		a.handleICEPacket(buf[:n], peer)
	}
}

// handleICEPacket processes a VL1 packet received over an ICE connection.
func (a *Agent) handleICEPacket(data []byte, peer *vl1.Peer) {
	var pkt vl1.Packet
	if err := vl1.DecodePacketInto(&pkt, data); err != nil {
		a.log.Debug("ICE decode packet", "peer", peer.Address, "err", err, "raw_len", len(data))
		return
	}

	peer.Touch()

	a.log.Debug("ICE packet type", "peer", peer.Address, "type", pkt.Header.Type, "payload_len", len(pkt.Payload))

	switch pkt.Header.Type {
	case vl1.PacketTypeHandshake:
		// Hello from peer via ICE — derive keys if needed
		if len(pkt.Payload) >= 32 && !peer.IsConnected() {
			var remotePubKey [32]byte
			copy(remotePubKey[:], pkt.Payload[:32])
			sendKey, recvKey := vl1.DeriveKeysFromPSK(a.config.PSK, a.identity.PublicKey, remotePubKey)
			cipher := vl1.NewNoiseCipher(sendKey, recvKey)
			peer.SetCipher(cipher)
			a.log.Info("peer connected via ICE handshake", "peer", peer.Address)
		}

	case vl1.PacketTypeData:
		bufp := vl1.GetPacketBuf()
		defer vl1.PutPacketBuf(bufp)
		plaintext, err := peer.DecryptTo(*bufp, pkt.Payload)
		if err != nil {
			a.log.Debug("ICE decrypt failed", "peer", peer.Address, "err", err)
			return
		}

		if a.network == nil {
			a.log.Debug("ICE data: no network", "peer", peer.Address)
			return
		}

		frameToInject, err := a.network.Switch.HandleRemoteFrame(peer.Address, plaintext)
		if err != nil {
			a.log.Debug("ICE switch handle remote frame", "err", err)
			return
		}

		if frameToInject != nil {
			a.log.Debug("ICE injecting frame into TAP", "peer", peer.Address, "len", len(frameToInject))
			a.injectFrame(frameToInject)
		}

	case vl1.PacketTypeKeepalive:
		// Already touched above

	default:
		a.log.Debug("ICE unknown packet type", "type", pkt.Header.Type, "peer", peer.Address)
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

	bufp := vl1.GetPacketBuf()
	defer vl1.PutPacketBuf(bufp)
	buf := *bufp

	// Write header into buf[0:HeaderSize]
	hdr := vl1.Header{Version: vl1.Version, Type: vl1.PacketTypeData, NetworkID: networkID}
	hdr.Encode(buf[:vl1.HeaderSize])

	// Encrypt directly into buf[HeaderSize:]
	n, err := peer.EncryptTo(buf[vl1.HeaderSize:], frame)
	if err != nil {
		return err
	}
	total := vl1.HeaderSize + n

	// Prefer ICE connection if available
	if iceConn := peer.ICEConn(); iceConn != nil {
		_, err := iceConn.Write(buf[:total])
		peer.LastSend = time.Now()
		a.log.Debug("sent data via ICE", "peer", peerAddr, "frame_len", len(frame), "total", total)
		return err
	}

	if peer.Endpoint == nil {
		return fmt.Errorf("peer %s: no endpoint and no ICE connection", peerAddr)
	}
	err = a.transport.SendTo(buf[:total], peer.Endpoint)
	peer.LastSend = time.Now()
	return err
}

// BroadcastToPeers sends an encrypted Ethernet frame to all connected peers in the network.
func (a *Agent) BroadcastToPeers(networkID uint32, frame []byte, excludePeer identity.Address) error {
	bufp := vl1.GetPacketBuf()
	defer vl1.PutPacketBuf(bufp)
	buf := *bufp

	// Write header once (same for all peers)
	hdr := vl1.Header{Version: vl1.Version, Type: vl1.PacketTypeData, NetworkID: networkID}
	hdr.Encode(buf[:vl1.HeaderSize])

	for _, peer := range a.peers.ConnectedPeers() {
		if peer.Address == excludePeer {
			continue
		}

		// Encrypt directly into buf[HeaderSize:] (each peer has different cipher)
		n, err := peer.EncryptTo(buf[vl1.HeaderSize:], frame)
		if err != nil {
			a.log.Debug("encrypt for broadcast", "peer", peer.Address, "err", err)
			continue
		}
		total := vl1.HeaderSize + n

		if iceConn := peer.ICEConn(); iceConn != nil {
			if _, err := iceConn.Write(buf[:total]); err != nil {
				a.log.Debug("broadcast send via ICE", "peer", peer.Address, "err", err)
			}
		} else if peer.Endpoint != nil {
			if err := a.transport.SendTo(buf[:total], peer.Endpoint); err != nil {
				a.log.Debug("broadcast send", "peer", peer.Address, "err", err)
			}
		}
	}
	return nil
}
