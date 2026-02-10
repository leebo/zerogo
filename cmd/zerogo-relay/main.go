package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/unicornultrafoundation/zerogo/internal/relay"
)

var version = "dev"

func main() {
	var (
		listen      = flag.String("listen", "0.0.0.0:3478", "STUN/TURN listen address")
		realm       = flag.String("realm", "zerogo", "TURN realm")
		publicIP    = flag.String("public-ip", "", "public IP for TURN relay")
		user        = flag.String("user", "zerogo", "TURN username")
		password    = flag.String("password", "zerogo", "TURN password")
		logLevel    = flag.String("log-level", "info", "log level")
		showVersion = flag.Bool("version", false, "show version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("zerogo-relay %s\n", version)
		os.Exit(0)
	}

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

	cfg := relay.Config{
		STUNEnabled: true,
		TURNEnabled: true,
		ListenAddr:  *listen,
		Realm:       *realm,
		PublicIP:    *publicIP,
		Credentials: map[string]string{
			*user: *password,
		},
	}

	srv := relay.New(cfg, log)
	if err := srv.Start(); err != nil {
		log.Error("start relay", "err", err)
		os.Exit(1)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Info("shutting down relay server")
	srv.Stop()
}
