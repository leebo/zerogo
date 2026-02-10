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
		networkID    = flag.Int("network", 1, "network ID")
		peers        = flag.String("peer", "", "static peer(s): pubkey@host:port,pubkey@host:port")
		pskHex       = flag.String("psk", "", "pre-shared key (hex, 64 chars)")
		controller   = flag.String("controller", "", "controller URL (ws://host:port)")
		logLevel     = flag.String("log-level", "info", "log level: debug, info, warn, error")
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
		LogLevel:      *logLevel,
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
