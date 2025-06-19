package cat

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"net"
	"net/netip"

	quic "github.com/quic-go/quic-go"
)

const (
	DefaultPort = 7001
)

type HostOption func(*Host) error

// NewHost returns a new Host object.
func NewHost(ctx context.Context, opts ...HostOption) (*Host, error) {
	h := &Host{
		ep: net.UDPAddrFromAddrPort(netip.AddrPortFrom(netip.IPv4Unspecified(), DefaultPort)),
	}

	for _, opt := range opts {
		err := opt(h)
		if err != nil {
			return nil, err
		}
	}

	// If the private key is not specified, generate a new one
	if h.sk == nil {
		_, sk, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
		h.sk = sk
	}

	// Listen for the specified endpoint
	conn, err := net.ListenUDP("udp", h.ep)
	if err != nil {
		return nil, err
	}
	h.tr = &quic.Transport{
		Conn: conn,
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{tls.Certificate{}},
	}
	h.ln, err = h.tr.Listen(tlsConfig, &quic.Config{})
	if err != nil {
		return nil, err
	}

	go h.processLoop(ctx)

	return h, nil
}

func (h *Host) Connect(ctx context.Context, addr net.Addr) error {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true, // TODO: Verify the certifcate properly when it's implemented
	}
	conn, err := h.tr.Dial(ctx, addr, tlsConfig, &quic.Config{})
	if err != nil {
		return err
	}

	// TODO
	_ = conn
	return nil
}

// processLoop handles all incoming connections
func (h *Host) processLoop(ctx context.Context) {
	for {
		conn, err := h.ln.Accept(ctx)
		if err != nil {
			// If there is an error, it probably means the context has been cancelled
			log.Warnf("quic-go listener returns an error in the Host processLoop: %v", err)
			return
		}

		// TODO
		_ = conn
	}
}

func WithAddrPort(ep netip.AddrPort) HostOption {
	return func(h *Host) error {
		h.ep = net.UDPAddrFromAddrPort(ep)
		return nil
	}
}

func WithIdentity(sk crypto.PrivateKey) HostOption {
	return func(h *Host) error {
		h.sk = sk
		return nil
	}
}

type Host struct {
	ep *net.UDPAddr
	sk crypto.PrivateKey

	tr *quic.Transport
	ln *quic.Listener
}
