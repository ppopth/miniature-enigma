package host

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

	logging "github.com/ipfs/go-log/v2"
	ic "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

var log = logging.Logger("host")

const (
	DefaultPort = 7001
)

type HostOption func(*Host) error

// NewHost returns a new Host object.
func NewHost(ctx context.Context, opts ...HostOption) (*Host, error) {
	h := &Host{
		ep:    net.UDPAddrFromAddrPort(netip.AddrPortFrom(netip.IPv4Unspecified(), DefaultPort)),
		peers: make(map[peer.ID]quic.Connection),
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
	quicConfig := &quic.Config{
		EnableDatagrams: true,
	}
	h.ln, err = h.tr.Listen(tlsConfig, quicConfig)
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
	quicConfig := &quic.Config{
		EnableDatagrams: true,
	}
	conn, err := h.tr.Dial(ctx, addr, tlsConfig, quicConfig)
	if err != nil {
		return err
	}

	p, err := h.handleConnection(conn)
	if err != nil {
		return err
	}
	log.Infof("connected to %s at %s", p, addr)
	return nil
}

func (h *Host) LocalAddr() net.Addr {
	return h.tr.Conn.LocalAddr()
}

func (h *Host) ID() peer.ID {
	return h.pid
}

type DgramConnection interface {
	SendDatagram(payload []byte) error
	ReceiveDatagram(context.Context) ([]byte, error)

	LocalAddr() net.Addr
	RemoteAddr() net.Addr
}

// TODO: The handlers are supposed to be able to handle any type of connection, but we
// support only a datagram connection for now.
type AddPeerHandler func(peer.ID, DgramConnection)
type RemovePeerHandler func(peer.ID)

func (h *Host) SetPeerHandlers(addHandler AddPeerHandler, removeHandler RemovePeerHandler) {
	h.lk.Lock()
	defer h.lk.Unlock()

	h.addHandler = addHandler
	h.removeHandler = removeHandler

	// Retrospectively add all exiting peers
	for p, conn := range h.peers {
		h.addHandler(p, conn)
	}
}

// handleConnection handles the connection regardless of whether it's incoming or outgoing
func (h *Host) handleConnection(conn quic.Connection) (peer.ID, error) {
	// Validate the connection

	// Derive the peer id from the certificate
	peerCrt := conn.ConnectionState().TLS.PeerCertificates[0]
	p, err := parsePeerIDFromCerticate(peerCrt)
	if err != nil {
		return "", fmt.Errorf("failed parsing for a peer ID from the TLS certificate: %v", err)
	}

	// Lock to read/write the peer set
	h.lk.Lock()
	defer h.lk.Unlock()

	// Check if there is already an onging connection with the peer
	if _, ok := h.peers[p]; ok {
		return "", fmt.Errorf("there is already an ongoing connection with peer %s", p)
	}

	// The connection is valid
	h.peers[p] = conn
	if h.addHandler != nil {
		h.addHandler(p, conn)
	}

	go func() {
		// Wait until the connection is closed and then remove the peer
		<-conn.Context().Done()

		h.lk.Lock()
		delete(h.peers, p)
		if h.removeHandler != nil {
			h.removeHandler(p)
		}
		h.lk.Unlock()
	}()
	return p, nil
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

		p, err := h.handleConnection(conn)
		if err != nil {
			log.Warnf("incoming connection is invalid: %v", err)
			// Close the connection. The error code is always 0 for now
			conn.CloseWithError(0, err.Error())
			continue
		}
		log.Infof("accepted a connection from %s at %s", p, conn.RemoteAddr())
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
	lk sync.Mutex

	peers map[peer.ID]quic.Connection

	crt *tls.Certificate
	ep  *net.UDPAddr
	pid peer.ID
	sk  crypto.PrivateKey

	tr *quic.Transport
	ln *quic.Listener

	addHandler    AddPeerHandler
	removeHandler RemovePeerHandler
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
