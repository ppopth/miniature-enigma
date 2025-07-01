package floodsub

import (
	"context"
	"crypto/sha256"
	"fmt"
	"maps"
	"math/rand"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ppopth/p2p-broadcast/host"
	"github.com/ppopth/p2p-broadcast/pubsub"
)

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

func sparseConnect(t *testing.T, hosts []*host.Host) {
	connect(t, hosts, 1)
}

func denseConnect(t *testing.T, hosts []*host.Host) {
	connect(t, hosts, 10)
}

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

func hashSha256(buf []byte) string {
	h := sha256.New()
	h.Write(buf)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func TestFloodsubSparse(t *testing.T) {
	hosts := getHosts(t, 20)
	psubs := getPubsubs(t, hosts)

	var topics []*pubsub.Topic
	var subs []*pubsub.Subscription
	for _, ps := range psubs {
		tp, err := ps.Join("foobar", NewFloodsub(hashSha256))
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
	msgs["foo"] = struct{}{}
	msgs["bar"] = struct{}{}

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
		topics[0].Publish([]byte(m))
	}
	wg.Wait()

	for i := range received {
		if !maps.Equal(received[i], msgs) {
			t.Fatalf("node %d received %v but expected %v", i, maps.Keys(received[i]), maps.Keys(msgs))
		}
	}
}

func TestFloodsubDense(t *testing.T) {
	hosts := getHosts(t, 20)
	psubs := getPubsubs(t, hosts)

	var topics []*pubsub.Topic
	var subs []*pubsub.Subscription
	for _, ps := range psubs {
		tp, err := ps.Join("foobar", NewFloodsub(hashSha256))
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
	msgs["foo"] = struct{}{}
	msgs["bar"] = struct{}{}

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
		topics[0].Publish([]byte(m))
	}
	wg.Wait()

	for i := range received {
		if !maps.Equal(received[i], msgs) {
			t.Fatalf("node %d received %v but expected %v", i, maps.Keys(received[i]), maps.Keys(msgs))
		}
	}
}

func TestFloodsubOneNode(t *testing.T) {
	h, err := host.NewHost(host.WithAddrPort(netip.MustParseAddrPort("127.0.0.1:0")))
	if err != nil {
		t.Fatal(err)
	}
	ps, err := pubsub.NewPubSub(h)
	if err != nil {
		t.Fatal(err)
	}
	tp, err := ps.Join("foobar", NewFloodsub(hashSha256))
	sub, err := tp.Subscribe()
	if err != nil {
		t.Fatal(err)
	}

	msgs := make(map[string]struct{})
	msgs["foo"] = struct{}{}
	msgs["bar"] = struct{}{}

	var wg sync.WaitGroup
	var lk sync.Mutex
	received := make(map[string]struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for _ = range msgs {
			buf, err := sub.Next(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			lk.Lock()
			received[string(buf)] = struct{}{}
			lk.Unlock()
		}
	}()

	time.Sleep(200 * time.Millisecond)

	for m := range msgs {
		tp.Publish([]byte(m))
	}
	wg.Wait()

	if !maps.Equal(received, msgs) {
		t.Fatalf("received %v but expected %v", maps.Keys(received), maps.Keys(msgs))
	}
}

func TestFloodsubConnectBeforeSubscribe(t *testing.T) {
	hosts := getHosts(t, 20)
	psubs := getPubsubs(t, hosts)

	sparseConnect(t, hosts)

	var topics []*pubsub.Topic
	var subs []*pubsub.Subscription
	for _, ps := range psubs {
		tp, err := ps.Join("foobar", NewFloodsub(hashSha256))
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

	msgs := make(map[string]struct{})
	msgs["foo"] = struct{}{}
	msgs["bar"] = struct{}{}

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
		topics[0].Publish([]byte(m))
	}
	wg.Wait()

	for i := range received {
		if !maps.Equal(received[i], msgs) {
			t.Fatalf("node %d received %v but expected %v", i, maps.Keys(received[i]), maps.Keys(msgs))
		}
	}
}
