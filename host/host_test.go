package host

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"maps"
	"net/netip"
	"testing"
	"time"

	ic "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

func TestCertificate(t *testing.T) {
	pk, sk, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	crt, err := createTLSCertFromKey(sk)
	if err != nil {
		t.Fatal(err)
	}
	crtPid, err := parsePeerIDFromCertificate(crt.Leaf)
	if err != nil {
		t.Fatal(err)
	}

	pubkey, err := ic.UnmarshalEd25519PublicKey(pk)
	if err != nil {
		t.Fatal(err)
	}
	p, err := peer.IDFromPublicKey(pubkey)
	if err != nil {
		t.Fatal(err)
	}

	if crtPid != p {
		t.Fatal("peer id in the created cerficate is not correct")
	}
}

func TestUniqueConnection(t *testing.T) {
	_, sk, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	// Server host
	s, err := NewHost(
		WithAddrPort(netip.MustParseAddrPort("127.0.0.1:0")),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Two client hosts
	h1, err := NewHost(
		WithAddrPort(netip.MustParseAddrPort("127.0.0.1:0")),
		WithIdentity(sk),
	)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := NewHost(
		WithAddrPort(netip.MustParseAddrPort("127.0.0.1:0")),
		WithIdentity(sk),
	)
	if err != nil {
		t.Fatal(err)
	}

	// First client connects first and then the second one
	addr := s.LocalAddr()
	if err := h1.Connect(context.Background(), addr); err != nil {
		t.Fatal(err)
	}
	// Wait for a bit before let the second connect
	time.Sleep(50 * time.Millisecond)
	// Second client connects
	if err := h2.Connect(context.Background(), addr); err != nil {
		t.Fatal(err)
	}

	// The first client should have connected successfully while the second
	// shouldn't because it has a duplicated peer id
	time.Sleep(50 * time.Millisecond)
	if len(h1.connections) != 1 {
		t.Fatal("the first client didn't connect successfully")
	}
	if len(h2.connections) != 0 {
		t.Fatal("the second client did connect successfully")
	}

	s.Close()
	h1.Close()
	h2.Close()
}

func TestHandlers(t *testing.T) {
	// Server host
	s, err := NewHost(WithAddrPort(netip.MustParseAddrPort("127.0.0.1:0")))
	if err != nil {
		t.Fatal(err)
	}

	// Two client hosts
	h1, err := NewHost(WithAddrPort(netip.MustParseAddrPort("127.0.0.1:0")))
	if err != nil {
		t.Fatal(err)
	}
	h2, err := NewHost(WithAddrPort(netip.MustParseAddrPort("127.0.0.1:0")))
	if err != nil {
		t.Fatal(err)
	}

	addr := s.LocalAddr()
	if err := h1.Connect(context.Background(), addr); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)

	empty := make(map[peer.ID]struct{})
	h1map := make(map[peer.ID]struct{})
	h2map := make(map[peer.ID]struct{})
	h12map := make(map[peer.ID]struct{})
	h1map[h1.peerID] = struct{}{}
	h2map[h2.peerID] = struct{}{}
	h12map[h1.peerID] = struct{}{}
	h12map[h2.peerID] = struct{}{}

	added := make(map[peer.ID]struct{})
	removed := make(map[peer.ID]struct{})

	addHandler := func(p peer.ID, conn Connection) {
		added[p] = struct{}{}
	}
	removeHandler := func(p peer.ID) {
		removed[p] = struct{}{}
	}
	s.SetPeerHandlers(addHandler, removeHandler)

	// The existing peers before the handlers are set must be called with the add handler as well
	if !maps.Equal(added, h1map) {
		t.Fatalf("the set of added peers is incorrect got %v expected %v", added, h1map)
	}
	if !maps.Equal(removed, empty) {
		t.Fatalf("the set of removed peers is incorrect got %v expected empty", removed)
	}
	added = make(map[peer.ID]struct{})
	removed = make(map[peer.ID]struct{})

	time.Sleep(50 * time.Millisecond)
	if err := h2.Connect(context.Background(), addr); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)

	if !maps.Equal(added, h2map) {
		t.Fatalf("the set of added peers is incorrect got %v expected %v", added, h2map)
	}
	if !maps.Equal(removed, empty) {
		t.Fatalf("the set of removed peers is incorrect got %v expected empty", removed)
	}
	added = make(map[peer.ID]struct{})
	removed = make(map[peer.ID]struct{})

	h1.connections[s.peerID].CloseWithError(0, "")
	h2.connections[s.peerID].CloseWithError(0, "")
	time.Sleep(50 * time.Millisecond)

	if !maps.Equal(added, empty) {
		t.Fatalf("the set of added peers is incorrect got %v expected empty", added)
	}
	if !maps.Equal(removed, h12map) {
		t.Fatalf("the set of removed peers is incorrect got %v expected %v", removed, h12map)
	}

	s.Close()
	h1.Close()
	h2.Close()
}

func TestClose(t *testing.T) {
	h1, err := NewHost(WithAddrPort(netip.MustParseAddrPort("127.0.0.1:0")))
	if err != nil {
		t.Fatal(err)
	}
	h2, err := NewHost(WithAddrPort(netip.MustParseAddrPort("127.0.0.1:0")))
	if err != nil {
		t.Fatal(err)
	}
	if err := h1.Connect(context.Background(), h2.LocalAddr()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	h1.Close()
	h2.Close()
}
