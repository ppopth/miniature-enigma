package verify

import (
	"context"
	"fmt"
	"maps"
	"math/rand"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ethp2p/eth-ec-broadcast/ec"
	"github.com/ethp2p/eth-ec-broadcast/ec/encode/rlnc"
	"github.com/ethp2p/eth-ec-broadcast/ec/field"
	"github.com/ethp2p/eth-ec-broadcast/ec/group"
	"github.com/ethp2p/eth-ec-broadcast/host"
	"github.com/ethp2p/eth-ec-broadcast/pubsub"
	"github.com/herumi/bls-eth-go-binary/bls"
)

// Helper function to create a test RLNC common config
func createTestRlncConfig(networkChunkSize, elementsPerChunk int) *rlnc.RlncCommonConfig {
	f := group.NewScalarField(Ristretto255Group)
	return &rlnc.RlncCommonConfig{
		NetworkChunkSize: networkChunkSize,
		ElementsPerChunk: elementsPerChunk,
		Field:            f,
	}
}

// TestGenExtras tests the GenExtras method that generates Pedersen commitments
func TestGenExtras(t *testing.T) {
	rlncConfig := createTestRlncConfig(1024, 8)
	p, err := NewPedersenVerifier(rlncConfig, nil)
	if err != nil {
		t.Fatalf("Failed to create PedersenVerifier: %v", err)
	}

	// Test 1: Single chunk
	chunk1 := make([]byte, 1024)
	chunk1[0] = 42 // Set some test data

	extras, err := p.GenerateExtras([][]byte{chunk1})
	if err != nil {
		t.Fatalf("Failed to generate extras for single chunk: %v", err)
	}

	if len(extras) != 1 {
		t.Errorf("Expected 1 extra, got %d", len(extras))
	}

	// Extra should not be empty
	if len(extras[0]) == 0 {
		t.Error("Extra data should not be empty")
	}
}

// TestGenExtrasMultipleChunks tests GenExtras with multiple chunks
func TestGenExtrasMultipleChunks(t *testing.T) {
	rlncConfig := createTestRlncConfig(1024, 8)
	p, err := NewPedersenVerifier(rlncConfig, nil)
	if err != nil {
		t.Fatalf("Failed to create PedersenVerifier: %v", err)
	}

	// Test with 3 chunks
	chunk1 := make([]byte, 1024)
	chunk2 := make([]byte, 1024)
	chunk3 := make([]byte, 1024)

	// Set different test data
	chunk1[0] = 1
	chunk2[0] = 2
	chunk3[0] = 3

	extras, err := p.GenerateExtras([][]byte{chunk1, chunk2, chunk3})
	if err != nil {
		t.Fatalf("Failed to generate extras for multiple chunks: %v", err)
	}

	if len(extras) != 3 {
		t.Errorf("Expected 3 extras, got %d", len(extras))
	}

	// All extras should be identical (contain all commitments)
	for i := 1; i < len(extras); i++ {
		if string(extras[i]) != string(extras[0]) {
			t.Errorf("Extra %d differs from extra 0", i)
		}
	}

	// Each extra should not be empty
	for i, extra := range extras {
		if len(extra) == 0 {
			t.Errorf("Extra %d should not be empty", i)
		}
	}
}

// TestGenExtrasEmptyInput tests GenExtras with empty input
func TestGenExtrasEmptyInput(t *testing.T) {
	rlncConfig := createTestRlncConfig(1024, 8)
	p, err := NewPedersenVerifier(rlncConfig, nil)
	if err != nil {
		t.Fatalf("Failed to create PedersenVerifier: %v", err)
	}

	extras, err := p.GenerateExtras([][]byte{})
	if err != nil {
		t.Fatalf("Failed to generate extras for empty input: %v", err)
	}

	if len(extras) != 0 {
		t.Errorf("Expected 0 extras for empty input, got %d", len(extras))
	}
}

// TestGenExtrasConsistency tests that GenExtras produces consistent results
func TestGenExtrasConsistency(t *testing.T) {
	rlncConfig := createTestRlncConfig(1024, 8)
	p, err := NewPedersenVerifier(rlncConfig, nil)
	if err != nil {
		t.Fatalf("Failed to create PedersenVerifier: %v", err)
	}

	chunk := make([]byte, 1024)
	chunk[0] = 123

	// Generate extras multiple times
	extras1, err := p.GenerateExtras([][]byte{chunk})
	if err != nil {
		t.Fatalf("Failed to generate extras (first time): %v", err)
	}

	extras2, err := p.GenerateExtras([][]byte{chunk})
	if err != nil {
		t.Fatalf("Failed to generate extras (second time): %v", err)
	}

	// Results should be deterministic
	if string(extras1[0]) != string(extras2[0]) {
		t.Error("GenExtras should produce consistent results for the same input")
	}
}

// TestGenExtrasDifferentChunks tests that different chunks produce different commitments
func TestGenExtrasDifferentChunks(t *testing.T) {
	rlncConfig := createTestRlncConfig(1024, 8)
	p, err := NewPedersenVerifier(rlncConfig, nil)
	if err != nil {
		t.Fatalf("Failed to create PedersenVerifier: %v", err)
	}

	chunk1 := make([]byte, 1024)
	chunk2 := make([]byte, 1024)
	chunk1[0] = 1
	chunk2[0] = 2 // Different data

	extras1, err := p.GenerateExtras([][]byte{chunk1})
	if err != nil {
		t.Fatalf("Failed to generate extras for chunk1: %v", err)
	}

	extras2, err := p.GenerateExtras([][]byte{chunk2})
	if err != nil {
		t.Fatalf("Failed to generate extras for chunk2: %v", err)
	}

	// Different chunks should produce different extra data
	if string(extras1[0]) == string(extras2[0]) {
		t.Error("Different chunks should produce different extra data")
	}
}

// TestVerify tests the Verify method with various scenarios
func TestVerify(t *testing.T) {
	rlncConfig := createTestRlncConfig(1024, 8)
	p, err := NewPedersenVerifier(rlncConfig, nil)
	if err != nil {
		t.Fatalf("Failed to create PedersenVerifier: %v", err)
	}

	// Test 1: Valid single chunk
	chunk1 := make([]byte, 1024)
	chunk1[0] = 42

	extras, err := p.GenerateExtras([][]byte{chunk1})
	if err != nil {
		t.Fatalf("Failed to generate extras: %v", err)
	}

	// Create a chunk with identity coefficients (this should verify)
	validChunk := &rlnc.Chunk{
		ChunkData: chunk1,
		Coeffs:    []field.Element{group.NewScalarField(p.group).One()}, // Identity coefficient
		Extra:     extras[0],
	}

	if !p.Verify(validChunk) {
		t.Error("Valid chunk should pass verification")
	}

	// Test 2: Invalid chunk with wrong data
	invalidChunk := &rlnc.Chunk{
		ChunkData: make([]byte, 1024), // Different data (all zeros)
		Coeffs:    []field.Element{group.NewScalarField(p.group).One()},
		Extra:     extras[0], // But same extra data
	}

	if p.Verify(invalidChunk) {
		t.Error("Invalid chunk with wrong data should fail verification")
	}

	// Test 3: Chunk with nil data
	if p.Verify(nil) {
		t.Error("Nil chunk should fail verification")
	}

	// Test 4: Chunk with empty extra data
	chunkNoExtra := &rlnc.Chunk{
		ChunkData: chunk1,
		Coeffs:    []field.Element{group.NewScalarField(p.group).One()},
		Extra:     []byte{},
	}

	if p.Verify(chunkNoExtra) {
		t.Error("Chunk with empty extra data should fail verification")
	}
}

// TestVerifyLinearCombination tests Verify with linear combinations
func TestVerifyLinearCombination(t *testing.T) {
	rlncConfig := createTestRlncConfig(1024, 8)
	p, err := NewPedersenVerifier(rlncConfig, nil)
	if err != nil {
		t.Fatalf("Failed to create PedersenVerifier: %v", err)
	}

	// Create two different chunks
	chunk1 := make([]byte, 1024)
	chunk2 := make([]byte, 1024)
	chunk1[0] = 42
	chunk2[0] = 100

	extrasMultiple, err := p.GenerateExtras([][]byte{chunk1, chunk2})
	if err != nil {
		t.Fatalf("Failed to generate extras for multiple chunks: %v", err)
	}

	// Create a linear combination: 2*chunk1 + 3*chunk2
	scalarField := group.NewScalarField(p.group)
	coeff1 := scalarField.FromBytes([]byte{2})
	coeff2 := scalarField.FromBytes([]byte{3})

	// Compute the linear combination data
	elements1 := p.splitIntoElements(chunk1)
	elements2 := p.splitIntoElements(chunk2)

	combinedElements := make([]field.Element, len(elements1))
	for i := range combinedElements {
		// combined[i] = 2*elements1[i] + 3*elements2[i]
		term1 := coeff1.Mul(elements1[i])
		term2 := coeff2.Mul(elements2[i])
		combinedElements[i] = term1.Add(term2)
	}

	combinedData := field.FieldElementsToBytes(combinedElements, (8*1024)/8)

	linearCombChunk := &rlnc.Chunk{
		ChunkData: combinedData,
		Coeffs:    []field.Element{coeff1, coeff2},
		Extra:     extrasMultiple[0], // Same extra data for all chunks
	}

	if !p.Verify(linearCombChunk) {
		t.Error("Valid linear combination chunk should pass verification")
	}

	// Test with wrong coefficients for linear combination
	wrongCoeffChunk := &rlnc.Chunk{
		ChunkData: combinedData,
		Coeffs:    []field.Element{scalarField.One(), scalarField.One()}, // Wrong coefficients
		Extra:     extrasMultiple[0],
	}

	if p.Verify(wrongCoeffChunk) {
		t.Error("Chunk with wrong coefficients should fail verification")
	}
}

// TestVerifyCorruptedExtraData tests Verify with various corrupted extra data
func TestVerifyCorruptedExtraData(t *testing.T) {
	rlncConfig := createTestRlncConfig(1024, 8)
	p, err := NewPedersenVerifier(rlncConfig, nil)
	if err != nil {
		t.Fatalf("Failed to create PedersenVerifier: %v", err)
	}

	chunk := make([]byte, 1024)
	chunk[0] = 42

	extras, err := p.GenerateExtras([][]byte{chunk})
	if err != nil {
		t.Fatalf("Failed to generate extras: %v", err)
	}

	// Test 1: Truncated extra data
	truncatedChunk := &rlnc.Chunk{
		ChunkData: chunk,
		Coeffs:    []field.Element{group.NewScalarField(p.group).One()},
		Extra:     extras[0][:10], // Truncated
	}

	if p.Verify(truncatedChunk) {
		t.Error("Chunk with truncated extra data should fail verification")
	}

	// Test 2: Corrupted commitment in extra data
	corruptedExtra := make([]byte, len(extras[0]))
	copy(corruptedExtra, extras[0])
	corruptedExtra[8] = ^corruptedExtra[8] // Flip some bits in the commitment

	corruptedChunk := &rlnc.Chunk{
		ChunkData: chunk,
		Coeffs:    []field.Element{group.NewScalarField(p.group).One()},
		Extra:     corruptedExtra,
	}

	if p.Verify(corruptedChunk) {
		t.Error("Chunk with corrupted commitment should fail verification")
	}

	// Test 3: Wrong number of coefficients
	wrongCoeffCountChunk := &rlnc.Chunk{
		ChunkData: chunk,
		Coeffs:    []field.Element{group.NewScalarField(p.group).One(), group.NewScalarField(p.group).One()}, // Too many coefficients
		Extra:     extras[0],
	}

	if p.Verify(wrongCoeffCountChunk) {
		t.Error("Chunk with wrong number of coefficients should fail verification")
	}
}

// TestGenExtrasWithBLS tests GenExtras with BLS signature generation
func TestGenExtrasWithBLS(t *testing.T) {
	// Generate a BLS secret key
	var sk bls.SecretKey
	sk.SetByCSPRNG()

	rlncConfig := createTestRlncConfig(1024, 8)
	config := &PedersenConfig{
		BLSSecretKey: &sk,
		PublisherID:  0,
	}
	p, err := NewPedersenVerifier(rlncConfig, config)
	if err != nil {
		t.Fatalf("Failed to create PedersenVerifier with BLS key: %v", err)
	}

	// Test with multiple chunks
	chunk1 := make([]byte, 1024)
	chunk2 := make([]byte, 1024)
	chunk1[0] = 42
	chunk2[0] = 100

	extras, err := p.GenerateExtras([][]byte{chunk1, chunk2})
	if err != nil {
		t.Fatalf("Failed to generate extras with BLS: %v", err)
	}

	if len(extras) != 2 {
		t.Errorf("Expected 2 extras, got %d", len(extras))
	}

	// Parse extra data to verify BLS signature is present
	extraData, err := p.parseExtraData(extras[0])
	if err != nil {
		t.Fatalf("Failed to parse extra data: %v", err)
	}

	// BLS signature should be 96 bytes
	if len(extraData.BLSSignature) != 96 {
		t.Errorf("Expected 96-byte BLS signature, got %d bytes", len(extraData.BLSSignature))
	}

	// Verify that the signature is not all zeros (indicating it was actually generated)
	allZeros := true
	for _, b := range extraData.BLSSignature {
		if b != 0 {
			allZeros = false
			break
		}
	}
	if allZeros {
		t.Error("BLS signature should not be all zeros when secret key is provided")
	}
}

// TestVerifyWithBLSSignature tests signature verification using the callback
func TestVerifyWithBLSSignature(t *testing.T) {
	// Generate a BLS key pair
	var sk bls.SecretKey
	sk.SetByCSPRNG()
	pk := sk.GetPublicKey()

	// Create callback that returns our public key
	publicKeyCallback := func(publisherID int) *bls.PublicKey {
		return pk
	}

	rlncConfig := createTestRlncConfig(1024, 8)
	config := &PedersenConfig{
		BLSSecretKey:      &sk,
		PublicKeyCallback: publicKeyCallback,
		PublisherID:       0,
	}
	p, err := NewPedersenVerifier(rlncConfig, config)
	if err != nil {
		t.Fatalf("Failed to create PedersenVerifier: %v", err)
	}

	// Generate chunks and extras with real BLS signature
	chunk1 := make([]byte, 1024)
	chunk1[0] = 42

	extras, err := p.GenerateExtras([][]byte{chunk1})
	if err != nil {
		t.Fatalf("Failed to generate extras: %v", err)
	}

	// Create a valid chunk
	validChunk := &rlnc.Chunk{
		ChunkData: chunk1,
		Coeffs:    []field.Element{group.NewScalarField(p.group).One()},
		Extra:     extras[0],
	}

	// This should pass verification (both commitment and BLS signature)
	if !p.Verify(validChunk) {
		t.Error("Valid chunk with correct BLS signature should pass verification")
	}

	// Test with corrupted signature
	corruptedExtra := make([]byte, len(extras[0]))
	copy(corruptedExtra, extras[0])
	// Corrupt the signature (last 96 bytes)
	corruptedExtra[len(corruptedExtra)-1] = ^corruptedExtra[len(corruptedExtra)-1]

	corruptedChunk := &rlnc.Chunk{
		ChunkData: chunk1,
		Coeffs:    []field.Element{group.NewScalarField(p.group).One()},
		Extra:     corruptedExtra,
	}

	// This should fail verification due to invalid BLS signature
	if p.Verify(corruptedChunk) {
		t.Error("Chunk with corrupted BLS signature should fail verification")
	}
}

// TestVerifyWithoutCallback tests that verification works when no callback is provided
func TestVerifyWithoutCallback(t *testing.T) {
	// Create algorithm without callback (skips BLS verification)
	rlncConfig := createTestRlncConfig(1024, 8)
	p, err := NewPedersenVerifier(rlncConfig, nil)
	if err != nil {
		t.Fatalf("Failed to create PedersenVerifier: %v", err)
	}

	chunk1 := make([]byte, 1024)
	chunk1[0] = 42

	extras, err := p.GenerateExtras([][]byte{chunk1})
	if err != nil {
		t.Fatalf("Failed to generate extras: %v", err)
	}

	validChunk := &rlnc.Chunk{
		ChunkData: chunk1,
		Coeffs:    []field.Element{group.NewScalarField(p.group).One()},
		Extra:     extras[0],
	}

	// Should still pass verification (only commitment check, BLS verification skipped)
	if !p.Verify(validChunk) {
		t.Error("Valid chunk should pass verification even without BLS callback")
	}
}

// TestRlncWithPedersenVerification tests RLNC with Pedersen commitment verification in a real network
func TestRlncWithPedersenVerification(t *testing.T) {
	hosts := getHosts(t, 5)
	psubs := getPubsubs(t, hosts)

	var topics []*pubsub.Topic
	var subs []*pubsub.Subscription
	for _, ps := range psubs {
		// Create RLNC common config
		f := group.NewScalarField(Ristretto255Group)
		rlncConfig := &rlnc.RlncCommonConfig{
			MessageChunkSize:   63,
			NetworkChunkSize:   64,
			ElementsPerChunk:   2,
			MaxCoefficientBits: 16,
			Field:              f,
		}

		// Create Pedersen verifier using the common config
		pedersenVerifier, err := NewPedersenVerifier(rlncConfig, nil)
		if err != nil {
			t.Fatal(err)
		}

		// Create RLNC encoder with Pedersen verification using the same common config
		encoderConfig := &rlnc.RlncEncoderConfig{
			RlncCommonConfig: *rlncConfig,
			Verifier:         pedersenVerifier,
		}

		encoder, err := rlnc.NewRlncEncoder(encoderConfig)
		if err != nil {
			t.Fatal(err)
		}

		// Create EC router with the encoder
		router, err := ec.NewEcRouter(encoder, ec.WithEcParams(ec.EcParams{
			PublishMultiplier: 4,
			ForwardMultiplier: 8,
		}))
		if err != nil {
			t.Fatal(err)
		}

		tp, err := ps.Join("foobar", router)
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
	msgs["fooofooofooofooo"+"12345678901234567890123456789012345678901234567"] = struct{}{}
	msgs["barrbarrbarrbarr"+"12345678901234567890123456789012345678901234567"] = struct{}{}

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

// Benchmarks

func BenchmarkPedersenGenerateExtras(b *testing.B) {
	sizes := []struct {
		name      string
		chunkSize int
		numChunks int
	}{
		{"1KB_4chunks", 1024, 4},
		{"1KB_16chunks", 1024, 16},
		{"1KB_64chunks", 1024, 64},
		{"4KB_4chunks", 4096, 4},
		{"4KB_16chunks", 4096, 16},
	}

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			rlncConfig := createTestRlncConfig(size.chunkSize, 8)
			pv, err := NewPedersenVerifier(rlncConfig, nil)
			if err != nil {
				b.Fatal(err)
			}

			// Create test chunks
			chunks := make([][]byte, size.numChunks)
			for i := 0; i < size.numChunks; i++ {
				chunks[i] = make([]byte, size.chunkSize)
				rand.Read(chunks[i])
			}

			b.SetBytes(int64(size.chunkSize * size.numChunks))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := pv.GenerateExtras(chunks)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkPedersenVerify(b *testing.B) {
	// Create RLNC common config
	f := group.NewScalarField(Ristretto255Group)
	rlncConfig := &rlnc.RlncCommonConfig{
		MessageChunkSize:   8,  // Use 8 bytes (64 bits) to stay within field capacity
		NetworkChunkSize:   64, // Network chunk size should be larger
		ElementsPerChunk:   1,  // One element per chunk to avoid field overflow
		MaxCoefficientBits: 16,
		Field:              f,
	}

	// Create Pedersen verifier using the common config
	pv, err := NewPedersenVerifier(rlncConfig, nil)
	if err != nil {
		b.Fatal(err)
	}

	// Create RLNC encoder with Pedersen verification using the same common config
	encoderConfig := &rlnc.RlncEncoderConfig{
		RlncCommonConfig: *rlncConfig,
		Verifier:         pv,
	}
	encoder, err := rlnc.NewRlncEncoder(encoderConfig)
	if err != nil {
		b.Fatal(err)
	}

	// Create a test message
	chunkData := make([]byte, 8)
	rand.Read(chunkData)

	// Create a proper chunk
	messageID := "bench-msg"
	encoder.GenerateThenAddChunks(messageID, chunkData)
	composedChunk, _ := encoder.EmitChunk(messageID)

	// Decode it back to get a chunk with proper coefficients
	decodedChunk, err := encoder.DecodeChunk(messageID, composedChunk.Data(), composedChunk.EncodeExtra())
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rlncChunk := decodedChunk.(rlnc.Chunk)
		if !pv.Verify(&rlncChunk) {
			b.Fatal("Verification failed")
		}
	}
}

func BenchmarkPedersenCombineExtras(b *testing.B) {
	rlncConfig := createTestRlncConfig(1024, 8)
	pv, err := NewPedersenVerifier(rlncConfig, nil)
	if err != nil {
		b.Fatal(err)
	}

	f := group.NewScalarField(Ristretto255Group)

	// Create test data
	numChunks := 16
	chunks := make([]rlnc.Chunk, numChunks)
	coeffs := make([]field.Element, numChunks)

	for i := 0; i < numChunks; i++ {
		chunkData := make([]byte, 1024)
		rand.Read(chunkData)

		// Generate extras for the chunk
		extras, _ := pv.GenerateExtras([][]byte{chunkData})

		chunks[i] = rlnc.Chunk{
			MessageID: "bench-msg",
			ChunkData: chunkData,
			Coeffs:    make([]field.Element, numChunks),
			Extra:     extras[0],
		}

		// Set coefficient for this chunk
		chunks[i].Coeffs[i] = f.One()
		coeffs[i] = f.FromBytes([]byte{byte(i + 1)})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := pv.CombineExtras(chunks, coeffs)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPedersenWithDifferentElementCounts(b *testing.B) {
	elementCounts := []int{1, 2, 4, 8, 16, 32}
	chunkSize := 1024

	for _, elemCount := range elementCounts {
		b.Run(fmt.Sprintf("elements=%d", elemCount), func(b *testing.B) {
			rlncConfig := createTestRlncConfig(chunkSize, elemCount)
			pv, err := NewPedersenVerifier(rlncConfig, nil)
			if err != nil {
				b.Fatal(err)
			}

			// Create test chunks
			numChunks := 4
			chunks := make([][]byte, numChunks)
			for i := 0; i < numChunks; i++ {
				chunks[i] = make([]byte, chunkSize)
				rand.Read(chunks[i])
			}

			b.SetBytes(int64(chunkSize * numChunks))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := pv.GenerateExtras(chunks)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkPedersenEndToEnd(b *testing.B) {
	// Create RLNC common config
	f := group.NewScalarField(Ristretto255Group)
	rlncConfig := &rlnc.RlncCommonConfig{
		MessageChunkSize:   8,  // Use 8 bytes (64 bits) to stay within field capacity
		NetworkChunkSize:   64, // Network chunk size should be larger
		ElementsPerChunk:   1,  // One element per chunk to avoid field overflow
		MaxCoefficientBits: 16,
		Field:              f,
	}

	// Create Pedersen verifier using the common config
	pv, err := NewPedersenVerifier(rlncConfig, nil)
	if err != nil {
		b.Fatal(err)
	}

	// Create RLNC encoder with Pedersen verification using the same common config
	encoderConfig := &rlnc.RlncEncoderConfig{
		RlncCommonConfig: *rlncConfig,
		Verifier:         pv,
	}

	// Test with different message sizes (must be multiples of 8)
	sizes := []int{8, 16, 32}
	for _, size := range sizes {
		msg := make([]byte, size)
		rand.Read(msg)

		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Create encoder
				encoder, err := rlnc.NewRlncEncoder(encoderConfig)
				if err != nil {
					b.Fatal(err)
				}
				messageID := fmt.Sprintf("msg-%d", i)

				// Generate chunks with Pedersen commitments
				numChunks, err := encoder.GenerateThenAddChunks(messageID, msg)
				if err != nil {
					b.Fatal(err)
				}

				// Create separate encoder for verification
				verifyEncoder, _ := rlnc.NewRlncEncoder(encoderConfig)

				// Verify all chunks
				for i := 0; i < numChunks; i++ {
					emittedChunk, _ := encoder.EmitChunk(messageID)
					if !verifyEncoder.VerifyThenAddChunk(emittedChunk) {
						b.Fatal("Chunk verification failed")
					}
				}

				// Reconstruct message
				_, err = encoder.ReconstructMessage(messageID)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
