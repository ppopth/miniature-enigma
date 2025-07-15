package rlnc

import (
	"context"
	"maps"
	"math/big"
	"math/rand"
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ppopth/p2p-broadcast/host"
	"github.com/ppopth/p2p-broadcast/pubsub"
	"github.com/ppopth/p2p-broadcast/rlnc/field"
)

// TestRlncSparse tests RLNC message distribution in a sparsely connected network
func TestRlncSparse(t *testing.T) {
	hosts := getHosts(t, 5)
	psubs := getPubsubs(t, hosts)

	var topics []*pubsub.Topic
	var subs []*pubsub.Subscription
	for _, ps := range psubs {
		f := field.NewPrimeField(big.NewInt(4_294_967_311))
		c, err := NewRlnc(
			WithRlncParams(RlncParams{
				MessageChunkSize:   8,
				NetworkChunkSize:   9,
				ElementsPerChunk:   2,
				MaxCoefficientBits: 16,
				PublishMultiplier:  4,
				ForwardMultiplier:  8,
			}),
			WithField(f),
		)
		if err != nil {
			t.Fatal(err)
		}
		tp, err := ps.Join("foobar", c)
		if err != nil {
			t.Fatal(err)
		}
		sub, err := tp.Subscribe()
		if err != nil {
			t.Fatal(err)
		}
		topics = append(topics, tp)
		subs = append(subs, sub)
	}

	sparseConnect(t, hosts)

	msgs := make(map[string]struct{})
	msgs["fooofooofooofooo"] = struct{}{}
	msgs["barrbarrbarrbarr"] = struct{}{}

	var wg sync.WaitGroup
	var lk sync.Mutex
	received := make([]map[string]struct{}, len(subs))
	for i := range received {
		received[i] = make(map[string]struct{})
	}

	for i, sub := range subs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _ = range msgs {
				buf, err := sub.Next(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				lk.Lock()
				received[i][string(buf)] = struct{}{}
				lk.Unlock()
			}
		}()
	}

	time.Sleep(200 * time.Millisecond)

	for m := range msgs {
		err := topics[0].Publish([]byte(m))
		if err != nil {
			t.Fatal(err)
		}
	}
	wg.Wait()

	for i := range received {
		if !maps.Equal(received[i], msgs) {
			t.Fatalf("node %d received %v but expected %v", i, received[i], msgs)
		}
	}
}

// TestRlncDense tests RLNC message distribution in a densely connected network
func TestRlncDense(t *testing.T) {
	hosts := getHosts(t, 20)
	psubs := getPubsubs(t, hosts)

	var topics []*pubsub.Topic
	var subs []*pubsub.Subscription
	for _, ps := range psubs {
		f := field.NewPrimeField(big.NewInt(4_294_967_311))
		c, err := NewRlnc(
			WithRlncParams(RlncParams{
				MessageChunkSize:   8,
				NetworkChunkSize:   9,
				ElementsPerChunk:   2,
				MaxCoefficientBits: 16,
				PublishMultiplier:  4,
				ForwardMultiplier:  8,
			}),
			WithField(f),
		)
		if err != nil {
			t.Fatal(err)
		}
		tp, err := ps.Join("foobar", c)
		if err != nil {
			t.Fatal(err)
		}
		sub, err := tp.Subscribe()
		if err != nil {
			t.Fatal(err)
		}
		topics = append(topics, tp)
		subs = append(subs, sub)
	}

	denseConnect(t, hosts)

	msgs := make(map[string]struct{})
	msgs["fooofooofooofooo"] = struct{}{}
	msgs["barrbarrbarrbarr"] = struct{}{}

	var wg sync.WaitGroup
	var lk sync.Mutex
	received := make([]map[string]struct{}, len(subs))
	for i := range received {
		received[i] = make(map[string]struct{})
	}

	for i, sub := range subs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _ = range msgs {
				buf, err := sub.Next(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				lk.Lock()
				received[i][string(buf)] = struct{}{}
				lk.Unlock()
			}
		}()
	}

	time.Sleep(200 * time.Millisecond)

	for m := range msgs {
		err := topics[0].Publish([]byte(m))
		if err != nil {
			t.Fatal(err)
		}
	}
	wg.Wait()

	for i := range received {
		if !maps.Equal(received[i], msgs) {
			t.Fatalf("node %d received %v but expected %v", i, received[i], msgs)
		}
	}
}

// TestRlncPublish tests RLNC message publishing and chunk reconstruction
func TestRlncPublish(t *testing.T) {
	hosts := getHosts(t, 5)
	psubs := getPubsubs(t, hosts)

	var topics []*pubsub.Topic
	var subs []*pubsub.Subscription
	var routers []*RlncRouter

	for _, ps := range psubs {
		f := field.NewPrimeField(big.NewInt(4_294_967_311))
		c, err := NewRlnc(
			WithRlncParams(RlncParams{
				MessageChunkSize:   8,
				NetworkChunkSize:   9,
				ElementsPerChunk:   2,
				MaxCoefficientBits: 16,
				PublishMultiplier:  4,
			}),
			WithField(f),
		)
		if err != nil {
			t.Fatal(err)
		}
		tp, err := ps.Join("foobar", c)
		if err != nil {
			t.Fatal(err)
		}
		sub, err := tp.Subscribe()
		if err != nil {
			t.Fatal(err)
		}
		topics = append(topics, tp)
		subs = append(subs, sub)
		routers = append(routers, c)
	}

	for _, h := range hosts[1:] {
		if err := hosts[0].Connect(context.Background(), h.LocalAddr()); err != nil {
			t.Fatal(err)
		}
	}

	time.Sleep(200 * time.Millisecond)
	// Publish a message
	msg := []byte("cold-bird-jump-fog-grid-sand-pen")
	mid := hashSha256(msg)
	if err := topics[0].Publish(msg); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	for i, router := range routers[1:] {
		if len(router.chunks) != 1 {
			t.Fatalf("a router %d should have received one message id; received %d", i, len(router.chunks))
		}
		if _, ok := router.chunks[mid]; !ok {
			t.Fatalf("a router %d didn't receive the expected messag id", i)
		}
		if len(router.chunks[mid]) != 4 {
			t.Fatalf("a router %d should have received %d chunks; received %d", i, 4, len(router.chunks[mid]))
		}
		buf, err := router.reconstructMessage(router.chunks[mid])
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(buf, msg) {
			t.Fatalf("a router %d should have received the recovered message of %s; received %s instead", i, string(msg), string(buf))
		}
	}
}

// TestRlncPublishWithGF232 tests RLNC message publishing and chunk reconstruction, but with GF(2^32)
func TestRlncPublishWithGF232(t *testing.T) {
	hosts := getHosts(t, 5)
	psubs := getPubsubs(t, hosts)

	var topics []*pubsub.Topic
	var subs []*pubsub.Subscription
	var routers []*RlncRouter

	for _, ps := range psubs {
		f := field.NewBinaryFieldGF2_32()
		c, err := NewRlnc(
			WithRlncParams(RlncParams{
				MessageChunkSize:   8,
				NetworkChunkSize:   9,
				ElementsPerChunk:   2,
				MaxCoefficientBits: 16,
				PublishMultiplier:  4,
			}),
			WithField(f),
		)
		if err != nil {
			t.Fatal(err)
		}
		tp, err := ps.Join("foobar", c)
		if err != nil {
			t.Fatal(err)
		}
		sub, err := tp.Subscribe()
		if err != nil {
			t.Fatal(err)
		}
		topics = append(topics, tp)
		subs = append(subs, sub)
		routers = append(routers, c)
	}

	for _, h := range hosts[1:] {
		if err := hosts[0].Connect(context.Background(), h.LocalAddr()); err != nil {
			t.Fatal(err)
		}
	}

	time.Sleep(200 * time.Millisecond)
	// Publish a message
	msg := []byte("cold-bird-jump-fog-grid-sand-pen")
	mid := hashSha256(msg)
	if err := topics[0].Publish(msg); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	for i, router := range routers[1:] {
		if len(router.chunks) != 1 {
			t.Fatalf("a router %d should have received one message id; received %d", i, len(router.chunks))
		}
		if _, ok := router.chunks[mid]; !ok {
			t.Fatalf("a router %d didn't receive the expected messag id", i)
		}
		if len(router.chunks[mid]) != 4 {
			t.Fatalf("a router %d should have received %d chunks; received %d", i, 4, len(router.chunks[mid]))
		}
		buf, err := router.reconstructMessage(router.chunks[mid])
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(buf, msg) {
			t.Fatalf("a router %d should have received the recovered message of %s; received %s instead", i, string(msg), string(buf))
		}
	}
}

// TestRlncCombine tests linear combination and reconstruction of RLNC chunks
func TestRlncCombine(t *testing.T) {
	f := field.NewPrimeField(big.NewInt(4_294_967_311))
	c, err := NewRlnc(
		WithRlncParams(RlncParams{
			MessageChunkSize:   8,
			NetworkChunkSize:   9,
			ElementsPerChunk:   2,
			MaxCoefficientBits: 16,
			PublishMultiplier:  4,
		}),
		WithField(f),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Creating a list of chunks to combine
	msg := []byte("cold-bird-jump-fog-grid-sand-pen")
	buf := msg
	var chunks []Chunk
	for len(buf) > 0 {
		chunk := buf[:c.params.MessageChunkSize]
		// Extend the chunk into the network size
		v := field.SplitBitsToFieldElements(chunk, c.messageBitsPerElement, c.field)
		chunk = field.FieldElementsToBytes(v, c.networkBitsPerElement)

		chunks = append(chunks, Chunk{
			Data: chunk,
		})
		buf = buf[c.params.MessageChunkSize:]
	}
	coeffs := make([]field.Element, len(chunks))
	for i := range chunks {
		coeffs[i] = f.Zero()
	}
	for i := range chunks {
		coeffs[i] = f.One()
		chunks[i].Coeffs = slices.Clone(coeffs)
		coeffs[i] = f.Zero()
	}

	// Combine chunks a few rounds
	var newChunks []Chunk
	numRounds := 3
	for i := 0; i < numRounds; i++ {
		for _ = range chunks {
			combined, err := c.combineChunks(chunks)
			if err != nil {
				t.Fatal(err)
			}
			newChunks = append(newChunks, combined)
		}
		chunks = newChunks
		newChunks = nil
	}

	buf, err = c.reconstructMessage(chunks)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(buf, msg) {
		t.Fatalf("the recovered message of should be %s; received %s instead", string(msg), string(buf))
	}
}

// isConnected checks if a graph represented as adjacency matrix is connected
func isConnected(graph [][]bool) bool {
	n := len(graph)
	if n == 0 {
		return true
	}

	visited := make([]bool, n)
	queue := []int{0}

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]

		if visited[node] {
			continue
		}
		visited[node] = true

		for neighbor := 0; neighbor < n; neighbor++ {
			// Traverse both directions to simulate undirected edges
			if (graph[node][neighbor] || graph[neighbor][node]) && !visited[neighbor] {
				queue = append(queue, neighbor)
			}
		}
	}

	for _, v := range visited {
		if !v {
			return false
		}
	}
	return true
}

// randGraph generates a random graph with n nodes where each node connects to d others
func randGraph(n, d int) [][]bool {
	graph := make([][]bool, n)
	for i := 0; i < n; i++ {
		graph[i] = make([]bool, n)
	}
	for i := 0; i < n; i++ {
		p := rand.Perm(n)
		for j := 0; j < d; j++ {
			if j >= n {
				break
			}
			n := p[j]
			// Don't connect to itself
			if n == i {
				continue
			}
			graph[i][n] = true
		}
	}
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if graph[i][j] {
				graph[j][i] = true
			}
		}
	}
	return graph
}

// connect establishes connections between hosts based on a random graph with degree d
func connect(t *testing.T, hosts []*host.Host, d int) {
	graph := randGraph(len(hosts), d)
	for !isConnected(graph) {
		graph = randGraph(len(hosts), d)
	}

	for i := 0; i < len(hosts); i++ {
		for j := i; j < len(hosts); j++ {
			if graph[i][j] {
				a := hosts[i]
				b := hosts[j]
				if err := a.Connect(context.Background(), b.LocalAddr()); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

// sparseConnect creates a sparsely connected network topology
func sparseConnect(t *testing.T, hosts []*host.Host) {
	connect(t, hosts, 1)
}

// denseConnect creates a densely connected network topology
func denseConnect(t *testing.T, hosts []*host.Host) {
	connect(t, hosts, 10)
}

// getHosts creates n test hosts with random local addresses
func getHosts(t *testing.T, n int) []*host.Host {
	var hs []*host.Host

	for i := 0; i < n; i++ {
		h, err := host.NewHost(host.WithAddrPort(netip.MustParseAddrPort("127.0.0.1:0")))
		if err != nil {
			t.Fatal(err)
		}
		hs = append(hs, h)
	}
	return hs
}

// getPubsubs creates PubSub instances for each provided host
func getPubsubs(t *testing.T, hs []*host.Host) []*pubsub.PubSub {
	var psubs []*pubsub.PubSub
	for _, h := range hs {
		ps, err := pubsub.NewPubSub(h)
		if err != nil {
			t.Fatal(err)
		}
		psubs = append(psubs, ps)
	}
	return psubs
}
