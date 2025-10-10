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
	"io"
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

// TransportMode defines how data is transmitted over QUIC connections
type TransportMode int

const (
	// TransportDatagram uses QUIC datagrams for unreliable, unordered delivery
	TransportDatagram TransportMode = iota
	// TransportStream uses QUIC streams for reliable, ordered delivery
	TransportStream
)

// HostOption configures a Host during construction
type HostOption func(*Host) error

// NewHost creates a new Host for peer-to-peer QUIC connections
func NewHost(opts ...HostOption) (*Host, error) {
	ctx, cancel := context.WithCancel(context.Background())

	host := &Host{
		ctx:    ctx,
		cancel: cancel,

		endpoint:      net.UDPAddrFromAddrPort(netip.AddrPortFrom(netip.IPv4Unspecified(), DefaultPort)),
		connections:   make(map[peer.ID]Connection),
		transportMode: TransportDatagram, // Default to datagram mode for backward compatibility
	}

	// Apply configuration options
	for _, opt := range opts {
		err := opt(host)
		if err != nil {
			return nil, err
		}
	}

	// Generate identity if not provided
	if host.privateKey == nil {
		_, privateKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
		err = WithIdentity(privateKey)(host)
		if err != nil {
			return nil, err
		}
	}

	// Create UDP socket
	udpConn, err := net.ListenUDP("udp", host.endpoint)
	if err != nil {
		return nil, err
	}

	// Wrap UDP connection to hide SyscallConn for Shadow compatibility
	var conn net.PacketConn = udpConn
	if host.shadowMode {
		conn = &shadowUDPConn{PacketConn: udpConn}
	}

	host.transport = &quic.Transport{
		Conn: conn,
	}

	// Create self-signed certificate from identity
	if host.certificate, err = createTLSCertFromKey(host.privateKey); err != nil {
		return nil, err
	}
	// Configure TLS with mutual authentication
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*host.certificate},
		ClientAuth:   tls.RequireAnyClientCert,
	}
	quicConfig := &quic.Config{
		EnableDatagrams: true,
		MaxIdleTimeout:  30 * time.Minute, // Keep connections alive for long Shadow simulations
	}
	host.listener, err = host.transport.Listen(tlsConfig, quicConfig)
	if err != nil {
		return nil, err
	}

	host.waitGroup.Add(1)
	go host.acceptLoop()

	return host, nil
}

// Connect establishes an outgoing connection to a peer
func (h *Host) Connect(ctx context.Context, addr net.Addr) error {
	// Create context that cancels when either host or request context is done
	dialCtx, cancel := context.WithCancel(context.Background())
	h.waitGroup.Add(1)
	go func() {
		defer h.waitGroup.Done()
		defer cancel()

		select {
		case <-h.ctx.Done():
		case <-ctx.Done():
		}
	}()

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{*h.certificate}, // Put a certificate to do client authentication
		InsecureSkipVerify: true,                              // TODO: Verify the certifcate properly when it's implemented
	}
	quicConfig := &quic.Config{
		EnableDatagrams: true,
		MaxIdleTimeout:  30 * time.Minute, // Keep connections alive for long Shadow simulations
	}
	conn, err := h.transport.Dial(dialCtx, addr, tlsConfig, quicConfig)
	if err != nil {
		return err
	}

	peerID, err := h.handleConnection(conn)
	if err != nil {
		return err
	}
	log.Infof("connected to %s at %s", peerID, addr)
	return nil
}

func (h *Host) LocalAddr() net.Addr {
	return h.transport.Conn.LocalAddr()
}

func (h *Host) ID() peer.ID {
	return h.peerID
}

func (h *Host) Close() error {
	if err := h.transport.Close(); err != nil {
		return err
	}
	h.cancel()
	h.waitGroup.Wait()
	return nil
}

type Sender interface {
	Send([]byte) error

	LocalAddr() net.Addr
	RemoteAddr() net.Addr
}
type Receiver interface {
	Receive(context.Context) ([]byte, error)

	LocalAddr() net.Addr
	RemoteAddr() net.Addr
}
type Connection interface {
	Sender
	Receiver

	// Close closes the underlying connection
	Close() error
}

// ByteCounter is an interface for tracking sent/received bytes
type ByteCounter interface {
	AddBytesSent(n uint64)
	AddBytesReceived(n uint64)
}

// quicDatagramConnection wraps a QUIC connection using datagrams
type quicDatagramConnection struct {
	conn    quic.Connection
	counter ByteCounter
}

func (qc *quicDatagramConnection) Send(buf []byte) error {
	err := qc.conn.SendDatagram(buf)
	if err == nil && qc.counter != nil {
		qc.counter.AddBytesSent(uint64(len(buf)))
	}
	return err
}
func (qc *quicDatagramConnection) Receive(ctx context.Context) ([]byte, error) {
	buf, err := qc.conn.ReceiveDatagram(ctx)
	if err == nil && qc.counter != nil {
		qc.counter.AddBytesReceived(uint64(len(buf)))
	}
	return buf, err
}
func (qc *quicDatagramConnection) LocalAddr() net.Addr {
	return qc.conn.LocalAddr()
}
func (qc *quicDatagramConnection) RemoteAddr() net.Addr {
	return qc.conn.RemoteAddr()
}

func (qc *quicDatagramConnection) Close() error {
	return qc.conn.CloseWithError(0, "")
}

// quicStreamConnection wraps a QUIC connection using bidirectional streams
type quicStreamConnection struct {
	conn       quic.Connection
	sendStream quic.Stream
	recvStream quic.Stream
	sendMutex  sync.Mutex
	recvReady  chan struct{} // Closed when recvStream is available
	counter    ByteCounter
}

func newQuicStreamConnection(conn quic.Connection, counter ByteCounter) *quicStreamConnection {
	sc := &quicStreamConnection{
		conn:      conn,
		recvReady: make(chan struct{}),
		counter:   counter,
	}
	// Start accepting streams in the background
	go sc.acceptStreams()
	return sc
}

func (qc *quicStreamConnection) acceptStreams() {
	// Accept the first incoming stream for receiving
	stream, err := qc.conn.AcceptStream(context.Background())
	if err != nil {
		return
	}
	qc.recvStream = stream
	close(qc.recvReady) // Signal that stream is ready
}

func (qc *quicStreamConnection) Send(buf []byte) error {
	qc.sendMutex.Lock()
	defer qc.sendMutex.Unlock()

	// Open stream on first write if not already open
	if qc.sendStream == nil {
		stream, err := qc.conn.OpenStreamSync(context.Background())
		if err != nil {
			return err
		}
		qc.sendStream = stream
	}

	// Write length prefix (4 bytes, big-endian)
	length := uint32(len(buf))
	lengthBuf := []byte{
		byte(length >> 24),
		byte(length >> 16),
		byte(length >> 8),
		byte(length),
	}

	if _, err := qc.sendStream.Write(lengthBuf); err != nil {
		return err
	}

	// Write message data
	_, err := qc.sendStream.Write(buf)
	if err == nil && qc.counter != nil {
		// Count length prefix + data
		qc.counter.AddBytesSent(uint64(4 + len(buf)))
	}
	return err
}

func (qc *quicStreamConnection) Receive(ctx context.Context) ([]byte, error) {
	// Wait for receive stream to be ready
	select {
	case <-qc.recvReady:
		// Stream is ready
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Read length prefix (4 bytes) - use io.ReadFull to ensure we get all 4 bytes
	lengthBuf := make([]byte, 4)
	if _, err := io.ReadFull(qc.recvStream, lengthBuf); err != nil {
		return nil, err
	}

	length := uint32(lengthBuf[0])<<24 | uint32(lengthBuf[1])<<16 |
		uint32(lengthBuf[2])<<8 | uint32(lengthBuf[3])

	// Read message data - use io.ReadFull to ensure we get all bytes
	buf := make([]byte, length)
	if _, err := io.ReadFull(qc.recvStream, buf); err != nil {
		return nil, err
	}

	if qc.counter != nil {
		// Count length prefix + data
		qc.counter.AddBytesReceived(uint64(4 + len(buf)))
	}

	return buf, nil
}

func (qc *quicStreamConnection) LocalAddr() net.Addr {
	return qc.conn.LocalAddr()
}

func (qc *quicStreamConnection) RemoteAddr() net.Addr {
	return qc.conn.RemoteAddr()
}

func (qc *quicStreamConnection) Close() error {
	return qc.conn.CloseWithError(0, "")
}

// AddPeerHandler is called when a new peer connects
type AddPeerHandler func(peer.ID, Connection)

// RemovePeerHandler is called when a peer disconnects
type RemovePeerHandler func(peer.ID)

// SetPeerHandlers registers callbacks for peer connection events
func (h *Host) SetPeerHandlers(addHandler AddPeerHandler, removeHandler RemovePeerHandler) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.addHandler = addHandler
	h.removeHandler = removeHandler

	// Notify about existing connections
	for peerID, conn := range h.connections {
		h.addHandler(peerID, conn)
	}
}

// newConnection creates a Connection wrapper based on the configured transport mode
func (h *Host) newConnection(conn quic.Connection) Connection {
	switch h.transportMode {
	case TransportDatagram:
		return &quicDatagramConnection{conn: conn, counter: h}
	case TransportStream:
		return newQuicStreamConnection(conn, h)
	default:
		// This should never happen due to validation in WithTransportMode
		panic(fmt.Sprintf("unsupported transport mode: %d", h.transportMode))
	}
}

// handleConnection processes a new connection (incoming or outgoing)
func (h *Host) handleConnection(conn quic.Connection) (peer.ID, error) {
	// Extract peer ID from TLS certificate
	peerCert := conn.ConnectionState().TLS.PeerCertificates[0]
	peerID, err := parsePeerIDFromCertificate(peerCert)
	if err != nil {
		return "", fmt.Errorf("failed parsing for a peer ID from the TLS certificate: %v", err)
	}

	// Lock to read/write the peer set
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// Prevent duplicate connections
	if _, exists := h.connections[peerID]; exists {
		return "", fmt.Errorf("already connected to peer %s", peerID)
	}

	// Create and register new connection
	wrappedConn := h.newConnection(conn)
	h.connections[peerID] = wrappedConn
	if h.addHandler != nil {
		h.addHandler(peerID, wrappedConn)
	}

	h.waitGroup.Add(1)
	go func() {
		defer h.waitGroup.Done()
		// Clean up when connection closes
		<-conn.Context().Done()

		h.mutex.Lock()
		delete(h.connections, peerID)
		if h.removeHandler != nil {
			h.removeHandler(peerID)
		}
		h.mutex.Unlock()
	}()
	return peerID, nil
}

// acceptLoop handles incoming connections
func (h *Host) acceptLoop() {
	defer h.waitGroup.Done()

	log.Infof("listening on %s", h.endpoint)
	log.Infof("peer ID: %s", h.peerID)

	for {
		conn, err := h.listener.Accept(h.ctx)
		if err != nil {
			// Context cancelled, shutting down
			log.Warnf("listener accept error: %v", err)
			return
		}

		peerID, err := h.handleConnection(conn)
		if err != nil {
			log.Warnf("failed to handle connection: %v", err)
			conn.CloseWithError(0, err.Error())
			continue
		}
		log.Infof("accepted connection from %s at %s", peerID, conn.RemoteAddr())
	}
}

func WithAddrPort(ep netip.AddrPort) HostOption {
	return func(h *Host) error {
		h.endpoint = net.UDPAddrFromAddrPort(ep)
		return nil
	}
}

// WithTransportMode sets the QUIC transport mode (datagram or stream)
func WithTransportMode(mode TransportMode) HostOption {
	return func(h *Host) error {
		if mode != TransportDatagram && mode != TransportStream {
			return fmt.Errorf("unsupported transport mode: %d", mode)
		}
		h.transportMode = mode
		return nil
	}
}

// WithShadowMode enables Shadow simulator compatibility mode
func WithShadowMode() HostOption {
	return func(h *Host) error {
		h.shadowMode = true
		return nil
	}
}

// shadowUDPConn wraps a net.PacketConn to hide the SyscallConn interface
// This prevents quic-go from setting DF flags that are not supported in Shadow
type shadowUDPConn struct {
	net.PacketConn
}

// Ensure shadowUDPConn only implements net.PacketConn, not syscall.Conn
var _ net.PacketConn = (*shadowUDPConn)(nil)

// WithIdentity sets the host's identity from a private key
func WithIdentity(privateKey crypto.PrivateKey) HostOption {
	return func(h *Host) error {
		var pubkey ic.PubKey
		var privkey ic.PrivKey
		var err error

		// Extract peer ID from private key
		switch key := privateKey.(type) {
		case ed25519.PrivateKey:
			privkey, err = ic.UnmarshalEd25519PrivateKey(key)
			pubkey = privkey.GetPublic()
		default:
			return fmt.Errorf("unsupported key type: %T", privateKey)
		}
		if err != nil {
			return err
		}
		peerID, err := peer.IDFromPublicKey(pubkey)
		if err != nil {
			return err
		}

		h.privateKey = privateKey
		h.peerID = peerID
		return nil
	}
}

// Host manages peer-to-peer QUIC connections
type Host struct {
	ctx       context.Context
	cancel    context.CancelFunc
	waitGroup sync.WaitGroup

	mutex sync.Mutex // Protects connections map

	connections map[peer.ID]Connection // Active wrapped connections

	shadowMode    bool          // Enable Shadow simulator compatibility mode
	transportMode TransportMode // How data is transmitted (datagram or stream)

	certificate *tls.Certificate  // Self-signed TLS certificate
	endpoint    *net.UDPAddr      // Local UDP endpoint
	peerID      peer.ID           // This host's peer ID
	privateKey  crypto.PrivateKey // Identity private key

	transport *quic.Transport // QUIC transport layer
	listener  *quic.Listener  // Incoming connection listener

	// Byte counters
	bytesSent     uint64     // Total bytes sent
	bytesReceived uint64     // Total bytes received
	statsMutex    sync.Mutex // Protects byte counters

	addHandler    AddPeerHandler    // Called when peer connects
	removeHandler RemovePeerHandler // Called when peer disconnects
}

// AddBytesSent increments the sent byte counter
func (h *Host) AddBytesSent(n uint64) {
	h.statsMutex.Lock()
	defer h.statsMutex.Unlock()
	h.bytesSent += n
}

// AddBytesReceived increments the received byte counter
func (h *Host) AddBytesReceived(n uint64) {
	h.statsMutex.Lock()
	defer h.statsMutex.Unlock()
	h.bytesReceived += n
}

// GetBytesSent returns the total bytes sent
func (h *Host) GetBytesSent() uint64 {
	h.statsMutex.Lock()
	defer h.statsMutex.Unlock()
	return h.bytesSent
}

// GetBytesReceived returns the total bytes received
func (h *Host) GetBytesReceived() uint64 {
	h.statsMutex.Lock()
	defer h.statsMutex.Unlock()
	return h.bytesReceived
}

// createTLSCertFromKey creates a self-signed certificate from a private key
func createTLSCertFromKey(key crypto.PrivateKey) (*tls.Certificate, error) {
	var publicKey crypto.PublicKey

	// Extract public key from private key
	switch privateKey := key.(type) {
	case ed25519.PrivateKey:
		publicKey = privateKey.Public()
	default:
		return nil, fmt.Errorf("unsupported key type: %T", key)
	}

	// Certificate template with minimal fields
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey, key)
	if err != nil {
		return nil, err
	}

	// Marshal private key
	keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}

	// Encode to PEM format
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})

	// Create TLS certificate
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

// parsePeerIDFromCertificate extracts peer ID from a TLS certificate
func parsePeerIDFromCertificate(cert *x509.Certificate) (peer.ID, error) {
	certPublicKey := cert.PublicKey

	var pubkey ic.PubKey
	var err error

	// Convert certificate public key to peer ID
	switch key := certPublicKey.(type) {
	case ed25519.PublicKey:
		pubkey, err = ic.UnmarshalEd25519PublicKey(key)
	default:
		return "", fmt.Errorf("unsupported public key type: %T", certPublicKey)
	}
	if err != nil {
		return "", err
	}

	peerID, err := peer.IDFromPublicKey(pubkey)
	if err != nil {
		return "", err
	}
	return peerID, nil
}
