package vl1

import (
	"fmt"
	"log/slog"
	"net"
	"sync"
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

// LocalAddr returns the local address of the UDP socket.
func (t *Transport) LocalAddr() net.Addr {
	return t.conn.LocalAddr()
}
