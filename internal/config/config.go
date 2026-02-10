package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// AgentConfig is the configuration for the zerogo-agent.
type AgentConfig struct {
	IdentityPath string   `yaml:"identity_path"`
	Controller   string   `yaml:"controller"`
	Networks     []NetworkRef `yaml:"networks"`
	STUNServers  []string `yaml:"stun_servers"`
	ListenPort   int      `yaml:"listen_port"`
	LogLevel     string   `yaml:"log_level"`
}

// NetworkRef is a reference to a network in the agent config.
type NetworkRef struct {
	ID string `yaml:"id"`
}

// ControllerConfig is the configuration for the zerogo-controller.
type ControllerConfig struct {
	Listen    string        `yaml:"listen"`
	Database  string        `yaml:"database"`
	JWTSecret string        `yaml:"jwt_secret"`
	STUN      STUNConfig    `yaml:"stun"`
	TURN      TURNConfig    `yaml:"turn"`
	Admin     AdminConfig   `yaml:"admin"`
	LogLevel  string        `yaml:"log_level"`
}

// STUNConfig configures the built-in STUN server.
type STUNConfig struct {
	Enabled bool   `yaml:"enabled"`
	Listen  string `yaml:"listen"`
}

// TURNConfig configures the built-in TURN server.
type TURNConfig struct {
	Enabled     bool              `yaml:"enabled"`
	Listen      string            `yaml:"listen"`
	Realm       string            `yaml:"realm"`
	Credentials map[string]string `yaml:"credentials"`
}

// AdminConfig is the default admin account.
type AdminConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// DefaultAgentConfig returns a config with sensible defaults.
func DefaultAgentConfig() *AgentConfig {
	return &AgentConfig{
		IdentityPath: "/etc/zerogo/identity.key",
		ListenPort:   9993,
		STUNServers: []string{
			"stun:stun.l.google.com:19302",
		},
		LogLevel: "info",
	}
}

// DefaultControllerConfig returns a config with sensible defaults.
func DefaultControllerConfig() *ControllerConfig {
	return &ControllerConfig{
		Listen:    "0.0.0.0:9394",
		Database:  "sqlite:///var/lib/zerogo/controller.db",
		JWTSecret: "change-me-in-production",
		STUN: STUNConfig{
			Enabled: true,
			Listen:  "0.0.0.0:3478",
		},
		TURN: TURNConfig{
			Enabled: false,
			Listen:  "0.0.0.0:3478",
			Realm:   "zerogo",
		},
		Admin: AdminConfig{
			Username: "admin",
			Password: "admin",
		},
		LogLevel: "info",
	}
}

// LoadAgentConfig loads agent config from a YAML file.
func LoadAgentConfig(path string) (*AgentConfig, error) {
	cfg := DefaultAgentConfig()
	if err := loadYAML(path, cfg); err != nil {
		return nil, fmt.Errorf("load agent config: %w", err)
	}
	return cfg, nil
}

// LoadControllerConfig loads controller config from a YAML file.
func LoadControllerConfig(path string) (*ControllerConfig, error) {
	cfg := DefaultControllerConfig()
	if err := loadYAML(path, cfg); err != nil {
		return nil, fmt.Errorf("load controller config: %w", err)
	}
	return cfg, nil
}

func loadYAML(path string, out interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, out)
}
