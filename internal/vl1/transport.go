package vl1

import (
	"fmt"
	"log/slog"
	"net"
	"sync"
	"syscall"
)

// Transport manages the UDP socket for VL1 communication.
type Transport struct {
	conn   *net.UDPConn
	port   int
	mu     sync.RWMutex
	closed bool
	log    *slog.Logger
}

// NewTransport creates and binds a UDP socket on the given port.
func NewTransport(port int, log *slog.Logger) (*Transport, error) {
	addr := &net.UDPAddr{Port: port}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("bind UDP port %d: %w", port, err)
	}
	// Get the actual port (useful if port was 0)
	actualPort := conn.LocalAddr().(*net.UDPAddr).Port
	log.Info("VL1 transport listening", "port", actualPort)
	return &Transport{
		conn: conn,
		port: actualPort,
		log:  log,
	}, nil
}

// Port returns the bound port number.
func (t *Transport) Port() int {
	return t.port
}

// ReadFrom reads a raw UDP packet. Returns the data, sender address, and error.
func (t *Transport) ReadFrom(buf []byte) (int, *net.UDPAddr, error) {
	n, addr, err := t.conn.ReadFromUDP(buf)
	return n, addr, err
}

// SendTo sends raw data to a specific UDP address.
func (t *Transport) SendTo(data []byte, addr *net.UDPAddr) error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed {
		return fmt.Errorf("transport closed")
	}
	_, err := t.conn.WriteToUDP(data, addr)
	return err
}

// SendPacket encodes and sends a VL1 packet to a specific address.
func (t *Transport) SendPacket(pkt *Packet, addr *net.UDPAddr) error {
	return t.SendTo(pkt.Encode(), addr)
}

// Close shuts down the transport.
func (t *Transport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
	return t.conn.Close()
}

// SetSocketBuffers sets the send and receive buffer sizes on the UDP socket.
func (t *Transport) SetSocketBuffers(rcvBuf, sndBuf int) error {
	rawConn, err := t.conn.SyscallConn()
	if err != nil {
		return fmt.Errorf("get raw conn: %w", err)
	}
	var setErr error
	err = rawConn.Control(func(fd uintptr) {
		if rcvBuf > 0 {
			if e := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF, rcvBuf); e != nil {
				setErr = fmt.Errorf("set SO_RCVBUF=%d: %w", rcvBuf, e)
				return
			}
		}
		if sndBuf > 0 {
			if e := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_SNDBUF, sndBuf); e != nil {
				setErr = fmt.Errorf("set SO_SNDBUF=%d: %w", sndBuf, e)
				return
			}
		}
	})
	if err != nil {
		return err
	}
	return setErr
}

// SetDSCP sets the DSCP value (Differentiated Services Code Point) on the UDP socket.
// The dscp value is shifted into the TOS byte (dscp << 2).
func (t *Transport) SetDSCP(dscp int) error {
	rawConn, err := t.conn.SyscallConn()
	if err != nil {
		return fmt.Errorf("get raw conn: %w", err)
	}
	tos := dscp << 2
	var setErr error
	err = rawConn.Control(func(fd uintptr) {
		if e := syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_TOS, tos); e != nil {
			setErr = fmt.Errorf("set IP_TOS=%d (DSCP %d): %w", tos, dscp, e)
		}
	})
	if err != nil {
		return err
	}
	return setErr
}

// LocalAddr returns the local address of the UDP socket.
func (t *Transport) LocalAddr() net.Addr {
	return t.conn.LocalAddr()
}
