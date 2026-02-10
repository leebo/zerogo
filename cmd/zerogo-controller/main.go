package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/unicornultrafoundation/zerogo/internal/config"
	"github.com/unicornultrafoundation/zerogo/internal/controller"
)

var version = "dev"

func main() {
	var (
		configPath  = flag.String("config", "", "path to controller config file")
		listen      = flag.String("listen", "", "override listen address (e.g., 0.0.0.0:9394)")
		database    = flag.String("database", "", "override database DSN")
		jwtSecret   = flag.String("jwt-secret", "", "override JWT secret")
		logLevel    = flag.String("log-level", "info", "log level: debug, info, warn, error")
		showVersion = flag.Bool("version", false, "show version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("zerogo-controller %s\n", version)
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

	// Load config
	var cfg *config.ControllerConfig
	if *configPath != "" {
		var err error
		cfg, err = config.LoadControllerConfig(*configPath)
		if err != nil {
			log.Error("load config", "err", err)
			os.Exit(1)
		}
	} else {
		cfg = config.DefaultControllerConfig()
	}

	// Apply CLI overrides
	if *listen != "" {
		cfg.Listen = *listen
	}
	if *database != "" {
		cfg.Database = *database
	}
	if *jwtSecret != "" {
		cfg.JWTSecret = *jwtSecret
	}
	cfg.LogLevel = *logLevel

	// Create and run controller
	ctrl, err := controller.New(cfg, log)
	if err != nil {
		log.Error("create controller", "err", err)
		os.Exit(1)
	}

	if err := ctrl.Run(); err != nil {
		log.Error("controller stopped", "err", err)
		os.Exit(1)
	}
}
