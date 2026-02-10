package protocol

const (
	// DefaultAgentPort is the default UDP port for VL1 transport.
	DefaultAgentPort = 9993
	// DefaultControllerPort is the default controller API port.
	DefaultControllerPort = 9394
	// DefaultSTUNPort is the default STUN/TURN port.
	DefaultSTUNPort = 3478

	// MaxFrameSize is the maximum Ethernet frame size supported.
	MaxFrameSize = 9000
	// DefaultMTU is the default overlay MTU.
	DefaultMTU = 2800

	// ProtocolVersion is the current protocol version.
	ProtocolVersion = 1
)
