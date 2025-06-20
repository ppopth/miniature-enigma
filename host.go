package cat

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/netip"
	"sync"
	"time"

	quic "github.com/quic-go/quic-go"

	ic "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
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
		err = WithIdentity(sk)(h)
		if err != nil {
			return nil, err
		}
	}

	// Listen for the specified endpoint
	conn, err := net.ListenUDP("udp", h.ep)
	if err != nil {
		return nil, err
	}
	h.tr = &quic.Transport{
		Conn: conn,
	}

	// Create a self-signed certifiate from the identity private key
	if h.crt, err = createTLSCertFromKey(h.sk); err != nil {
		return nil, err
	}
	// Assign the certificate to the TLS config and require client authentication
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*h.crt},
		ClientAuth:   tls.RequireAnyClientCert,
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
		Certificates:       []tls.Certificate{*h.crt}, // Put a certificate to do client authentication
		InsecureSkipVerify: true,                      // TODO: Verify the certifcate properly when it's implemented
	}
	conn, err := h.tr.Dial(ctx, addr, tlsConfig, &quic.Config{})
	if err != nil {
		return err
	}

	// Derive the peer id from the certificate
	peerCrt := conn.ConnectionState().TLS.PeerCertificates[0]
	pid, err := parsePeerIDFromCerticate(peerCrt)
	if err != nil {
		return fmt.Errorf("failed parsing for a peer ID from the TLS certificate: %v", err)
	}

	log.Infof("connected to %s at %s", pid, addr)

	go h.handler(pid, conn)
	return nil
}

type DgramConnection interface {
	SendDatagram(payload []byte) error
	ReceiveDatagram(context.Context) ([]byte, error)

	LocalAddr() net.Addr
	RemoteAddr() net.Addr
}

// TODO: The connection handler is supposed to be able to handle any type of connection, but
// we support only a datagram connection for now.
type ConnectionHandler func(peer.ID, DgramConnection)

func (h *Host) SetConnectionHandler(handler ConnectionHandler) {
	h.handlerLk.Lock()
	defer h.handlerLk.Unlock()

	h.handler = handler
}

// processLoop handles all incoming connections
func (h *Host) processLoop(ctx context.Context) {
	log.Infof("listening for incoming connections at %s", h.ep)
	log.Infof("my peer id is %s", h.pid)

	for {
		conn, err := h.ln.Accept(ctx)
		if err != nil {
			// If there is an error, it probably means the context has been cancelled
			log.Warnf("quic-go listener returned an error in the Host processLoop: %v", err)
			return
		}

		// Derive the peer id from the certificate
		peerCrt := conn.ConnectionState().TLS.PeerCertificates[0]
		pid, err := parsePeerIDFromCerticate(peerCrt)
		if err != nil {
			log.Warnf("failed parsing for a peer ID from the TLS certificate: %v", err)
			continue
		}

		log.Infof("accepted a connection from %s at %s", pid, conn.RemoteAddr())

		go h.handler(pid, conn)
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
		var pubkey ic.PubKey
		var privkey ic.PrivKey
		var err error

		// Currently only ed25519 is supported in this function
		switch k := sk.(type) {
		case ed25519.PrivateKey:
			privkey, err = ic.UnmarshalEd25519PrivateKey(k)
			pubkey = privkey.GetPublic()
		default:
			return fmt.Errorf("unsupported key type: %T", sk)
		}
		if err != nil {
			return err
		}
		p, err := peer.IDFromPublicKey(pubkey)
		if err != nil {
			return err
		}

		h.sk = sk
		h.pid = p
		return nil
	}
}

type Host struct {
	crt *tls.Certificate
	ep  *net.UDPAddr
	pid peer.ID
	sk  crypto.PrivateKey

	tr *quic.Transport
	ln *quic.Listener

	handlerLk sync.RWMutex
	handler   ConnectionHandler
}

// createTLSCertFromKey creates a self-signed certificate from a private key
func createTLSCertFromKey(key crypto.PrivateKey) (*tls.Certificate, error) {
	var pub crypto.PublicKey

	// Currently only ed25519 is supported in this function
	switch k := key.(type) {
	case ed25519.PrivateKey:
		pub = k.Public()
	default:
		return nil, fmt.Errorf("unsupported key type: %T", key)
	}

	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),                      // This can be anything
		Subject:      pkix.Name{CommonName: "localhost"}, // This can be anything
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, pub, key)
	if err != nil {
		return nil, err
	}

	keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

// parsePeerIDFromCerticate derive the peer ID from a TLS certifiate
func parsePeerIDFromCerticate(crt *x509.Certificate) (peer.ID, error) {
	crtPubkey := crt.PublicKey

	var pubkey ic.PubKey
	var err error

	// Currently only ed25519 is supported in this function
	switch k := crtPubkey.(type) {
	case ed25519.PublicKey:
		pubkey, err = ic.UnmarshalEd25519PublicKey(k)
	default:
		return "", fmt.Errorf("unsupported public key type: %T", crtPubkey)
	}
	if err != nil {
		return "", err
	}

	pid, err := peer.IDFromPublicKey(pubkey)
	if err != nil {
		return "", err
	}
	return pid, nil
}
