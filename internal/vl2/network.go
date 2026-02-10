package vl2

import (
	"log/slog"

	"github.com/unicornultrafoundation/zerogo/internal/identity"
)

// NetworkConfig holds the configuration for a virtual network.
type NetworkConfig struct {
	ID        uint32
	Name      string
	IPRange   string // CIDR notation, e.g. "10.147.17.0/24"
	IP6Range  string // optional IPv6 CIDR
	MTU       int
	Multicast bool
}

// Network represents a virtual L2 network instance on a node.
type Network struct {
	Config   NetworkConfig
	Switch   *Switch
	ARP      *ARPProxy
	LocalMAC [6]byte
	log      *slog.Logger
}

// NewNetwork creates a new virtual network instance.
func NewNetwork(config NetworkConfig, nodeAddr identity.Address, sender PeerSender, log *slog.Logger) *Network {
	netLog := log.With("network", config.ID, "name", config.Name)
	mac := GenerateMAC(config.ID, nodeAddr)
	var macArr [6]byte
	copy(macArr[:], mac)
	return &Network{
		Config:   config,
		Switch:   NewSwitch(config.ID, sender, netLog),
		ARP:      NewARPProxy(netLog),
		LocalMAC: macArr,
		log:      netLog,
	}
}
