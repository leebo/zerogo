package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/unicornultrafoundation/zerogo/internal/agent"
)

var version = "dev"

func main() {
	// CLI flags
	var (
		identityPath = flag.String("identity", "/etc/zerogo/identity.key", "path to identity key file")
		listenPort   = flag.Int("port", 9993, "UDP listen port for VL1 transport")
		tapName      = flag.String("tap", "zt0", "TAP device name")
		tapIP        = flag.String("tap-ip", "", "IP/mask to assign to TAP (e.g., 10.147.17.1/24)")
		tapMTU       = flag.Int("mtu", 2800, "TAP device MTU")
		networkID    = flag.Int("network", 1, "network ID (for static mode)")
		networks     = flag.String("networks", "", "comma-separated network IDs to join via controller")
		peers        = flag.String("peer", "", "static peer(s): pubkey@host:port,pubkey@host:port")
		pskHex       = flag.String("psk", "", "pre-shared key (hex, 64 chars)")
		controller   = flag.String("controller", "", "controller URL (ws://host:port or http://host:port)")
		stunServers  = flag.String("stun", "", "comma-separated STUN server URIs (e.g., stun:stun.l.google.com:19302)")
		logLevel     = flag.String("log-level", "info", "log level: debug, info, warn, error")
		gaming       = flag.Bool("gaming", false, "enable gaming optimization mode (large socket buffers, DSCP EF, fast keepalive)")
		dscp         = flag.Int("dscp", 0, "DSCP marking value (0=default, 46=EF; gaming mode defaults to 46)")
		sndBuf       = flag.Int("sndbuf", 0, "UDP send buffer size in bytes (0=OS default; gaming mode defaults to 4MB)")
		rcvBuf       = flag.Int("rcvbuf", 0, "UDP receive buffer size in bytes (0=OS default; gaming mode defaults to 4MB)")
		showVersion  = flag.Bool("version", false, "show version and exit")
		showIdentity = flag.Bool("show-identity", false, "show identity and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("zerogo-agent %s\n", version)
		os.Exit(0)
	}

	// Setup logging
	var level slog.Level
	switch strings.ToLower(*logLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	// Parse PSK
	var psk [32]byte
	if *pskHex != "" {
		b, err := hex.DecodeString(*pskHex)
		if err != nil || len(b) != 32 {
			log.Error("invalid PSK: must be 64 hex characters (32 bytes)")
			os.Exit(1)
		}
		copy(psk[:], b)
	}

	// Build config
	cfg := agent.Config{
		IdentityPath:  *identityPath,
		ListenPort:    *listenPort,
		TAPName:       *tapName,
		TAPIPv4:       *tapIP,
		TAPMTU:        *tapMTU,
		NetworkID:     uint32(*networkID),
		PSK:           psk,
		ControllerURL: *controller,
		Gaming:        *gaming,
		DSCP:          *dscp,
		SndBuf:        *sndBuf,
		RcvBuf:        *rcvBuf,
		LogLevel:      *logLevel,
	}

	// Gaming mode defaults
	if cfg.Gaming {
		if cfg.DSCP == 0 {
			cfg.DSCP = 46 // EF (Expedited Forwarding)
		}
		if cfg.SndBuf == 0 {
			cfg.SndBuf = 4 * 1024 * 1024 // 4MB
		}
		if cfg.RcvBuf == 0 {
			cfg.RcvBuf = 4 * 1024 * 1024 // 4MB
		}
		if cfg.LogLevel == "" || cfg.LogLevel == "debug" {
			cfg.LogLevel = "info" // suppress debug noise in gaming mode
			level = slog.LevelInfo
			log = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
		}
	}

	// Parse STUN servers
	if *stunServers != "" {
		for _, s := range strings.Split(*stunServers, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				cfg.STUNServers = append(cfg.STUNServers, s)
			}
		}
	}

	// Parse network IDs for controller mode
	if *networks != "" {
		cfg.Networks = strings.Split(*networks, ",")
		for i := range cfg.Networks {
			cfg.Networks[i] = strings.TrimSpace(cfg.Networks[i])
		}
	}

	// Convert http:// to ws:// for controller URL
	if cfg.ControllerURL != "" && strings.HasPrefix(cfg.ControllerURL, "http://") {
		cfg.ControllerURL = "ws://" + cfg.ControllerURL[7:]
	} else if cfg.ControllerURL != "" && strings.HasPrefix(cfg.ControllerURL, "https://") {
		cfg.ControllerURL = "wss://" + cfg.ControllerURL[8:]
	}

	// Parse static peers
	if *peers != "" {
		for _, peerStr := range strings.Split(*peers, ",") {
			parts := strings.SplitN(peerStr, "@", 2)
			if len(parts) != 2 {
				log.Error("invalid peer format, expected pubkey@host:port", "peer", peerStr)
				os.Exit(1)
			}
			cfg.StaticPeers = append(cfg.StaticPeers, agent.PeerEndpoint{
				PublicKey: parts[0],
				Address:   parts[1],
			})
		}
	}

	// Create and start agent
	a, err := agent.New(cfg, log)
	if err != nil {
		log.Error("create agent failed", "err", err)
		os.Exit(1)
	}

	if *showIdentity {
		fmt.Printf("Address:    %s\n", a.Identity().Address)
		fmt.Printf("Public Key: %s\n", a.Identity().PublicKeyHex())
		os.Exit(0)
	}

	if err := a.Start(); err != nil {
		log.Error("start agent failed", "err", err)
		os.Exit(1)
	}

	// Wait for signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Info("received signal, shutting down", "signal", sig)

	a.Stop()
}
