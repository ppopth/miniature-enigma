package verify

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/ethp2p/eth-ec-broadcast/ec/encode/rlnc"
	"github.com/ethp2p/eth-ec-broadcast/ec/field"
	"github.com/ethp2p/eth-ec-broadcast/ec/group"
	"github.com/herumi/bls-eth-go-binary/bls"
)

var (
	// Ristretto255Group is the Ristretto255 group instance
	Ristretto255Group group.PrimeOrderGroup
)

func init() {
	// Initialize the Ristretto255 group (which includes its scalar field)
	Ristretto255Group = group.NewRistretto255Group()

	// Initialize BLS
	if err := bls.Init(bls.BLS12_381); err != nil {
		panic(fmt.Sprintf("failed to initialize BLS: %v", err))
	}
	if err := bls.SetETHmode(bls.EthModeDraft07); err != nil {
		panic(fmt.Sprintf("failed to set ETH mode: %v", err))
	}
}

// PublicKeyCallback is a function type that takes a publisher ID and returns a BLS public key for verification
type PublicKeyCallback func(publisherID int) *bls.PublicKey

// PedersenConfig contains configuration for Pedersen verification
type PedersenConfig struct {
	Group             group.PrimeOrderGroup // The prime-order group to use for commitments
	BLSSecretKey      *bls.SecretKey        // BLS secret key for signing commitments
	PublicKeyCallback PublicKeyCallback     // Callback function to get public key for verification
	PublisherID       int                   // ID of the publisher who signs the data
}

// PedersenVerifier implements Pedersen commitments for chunk verification
// Based on the Ethereum research: "Faster Block/Blob Propagation in Ethereum"
// https://ethresear.ch/t/faster-block-blob-propagation-in-ethereum/21370
// Uses a prime-order group interface for secure cryptographic operations
type PedersenVerifier struct {
	chunkSize             int
	networkBitsPerElement int
	// The prime-order group used for commitments
	group group.PrimeOrderGroup
	// The scalar field for the group
	scalarField field.Field
	// Generator points for the commitment scheme
	generators []group.GroupElement
	// BLS secret key for signing commitments
	blsSecretKey *bls.SecretKey
	// Callback function to get public key for verification
	publicKeyCallback PublicKeyCallback
	// Publisher ID to include in extra data
	publisherID int
}

// ExtraData represents the structure of the extra field
// Contains publisher ID (4 bytes) + N commitments (variable bytes each) + BLS signature (96 bytes)
type ExtraData struct {
	PublisherID  int      // ID of the publisher who signed this data
	Commitments  [][]byte // N commitments, each encoded group element
	BLSSignature []byte   // BLS signature, 96 bytes
}

// NewPedersenVerifier creates a new Pedersen commitment verifier using the specified configurations
func NewPedersenVerifier(rlncConfig *rlnc.RlncCommonConfig, pedersenConfig *PedersenConfig) (*PedersenVerifier, error) {
	if rlncConfig == nil {
		return nil, fmt.Errorf("rlncConfig cannot be nil")
	}
	if rlncConfig.Field == nil {
		return nil, fmt.Errorf("rlncConfig.Field cannot be nil")
	}

	pv := &PedersenVerifier{
		chunkSize: rlncConfig.NetworkChunkSize,
	}

	// Set configuration if provided, otherwise use defaults
	if pedersenConfig != nil {
		pv.group = pedersenConfig.Group
		pv.blsSecretKey = pedersenConfig.BLSSecretKey
		pv.publicKeyCallback = pedersenConfig.PublicKeyCallback
		pv.publisherID = pedersenConfig.PublisherID
	}

	// Use default group if not provided
	if pv.group == nil {
		pv.group = Ristretto255Group
	}

	// Create the scalar field for the group
	pv.scalarField = group.NewScalarField(pv.group)

	// Verify that the RLNC field and Pedersen group have the same order
	// The RLNC field order must match the scalar field order of the group
	rlncOrder := rlncConfig.Field.Order()
	scalarOrder := pv.scalarField.Order()
	if rlncOrder.Cmp(scalarOrder) != 0 {
		return nil, fmt.Errorf("rlncConfig.Field order (%s) does not match pedersenConfig.Group scalar field order (%s)",
			rlncOrder.String(), scalarOrder.String())
	}

	pv.networkBitsPerElement = 8 * rlncConfig.NetworkChunkSize / rlncConfig.ElementsPerChunk
	if pv.networkBitsPerElement < pv.scalarField.BitsPerElement() {
		return nil, fmt.Errorf("(8*networkChunkSize)/elementsPerChunk (%d) is too low for the field", pv.networkBitsPerElement)
	}

	// Generate deterministic generator points for the commitment scheme
	pv.generators = make([]group.GroupElement, rlncConfig.ElementsPerChunk)

	for i := 0; i < rlncConfig.ElementsPerChunk; i++ {
		pv.generators[i] = pv.generatePoint(i)
	}

	return pv, nil
}

// GenerateExtras generates extra fields for all chunks
// chunks is a slice of chunk data, returns a slice of extra fields for each chunk
func (pv *PedersenVerifier) GenerateExtras(chunks [][]byte) ([][]byte, error) {
	if len(chunks) == 0 {
		return [][]byte{}, nil
	}

	// Generate commitments for ALL chunks
	commitments := make([][]byte, len(chunks))
	for i, chunkData := range chunks {
		elements := pv.splitIntoElements(chunkData)
		commitment := pv.commit(elements)
		commitments[i] = pv.serializeCommitment(commitment)
	}

	// Generate BLS signature over all commitments
	blsSignature := pv.generateBLSSignature(commitments)

	// Create extra data structure with publisher ID, commitments and signature
	extraData := &ExtraData{
		PublisherID:  pv.publisherID,
		Commitments:  commitments,
		BLSSignature: blsSignature,
	}

	// Each chunk gets the same extra data (containing all commitments)
	serializedExtra := pv.serializeExtraData(extraData)
	extras := make([][]byte, len(chunks))
	for i := range chunks {
		extras[i] = serializedExtra
	}

	return extras, nil
}

// CombineExtras copies the extra data from the first chunk and returns it
func (pv *PedersenVerifier) CombineExtras(chunks []rlnc.Chunk, factors []field.Element) ([]byte, error) {
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks provided")
	}
	return chunks[0].Extra, nil
}

// Verify checks if the chunk is valid using Pedersen commitments
func (pv *PedersenVerifier) Verify(chunk *rlnc.Chunk) bool {
	if chunk == nil || len(chunk.Extra) == 0 {
		return false
	}

	// Parse the extra data to get commitments and BLS signature
	extraData, err := pv.parseExtraData(chunk.Extra)
	if err != nil {
		return false
	}

	// Check that we have the right number of coefficients
	if len(chunk.Coeffs) != len(extraData.Commitments) {
		return false
	}

	// Verify BLS signature over the commitments
	if !pv.verifyBLSSignature(extraData.Commitments, extraData.BLSSignature, extraData.PublisherID) {
		return false
	}

	// Split chunk data into field elements
	elements := pv.splitIntoElements(chunk.Data())

	// Compute the commitment for this chunk's data
	computedCommitment := pv.commit(elements)

	// Compute the linear combination of stored commitments using chunk coefficients
	expectedCommitment := pv.group.Zero()
	for i, storedCommitmentBytes := range extraData.Commitments {
		// Parse the stored commitment
		storedCommitment := pv.parseCommitment(storedCommitmentBytes)
		if storedCommitment == nil {
			return false
		}

		// Convert coefficient to scalar
		scalar := pv.group.ScalarFromFieldElement(chunk.Coeffs[i])

		// Add coefficient * commitment to the result
		term := storedCommitment.ScalarMult(scalar)
		expectedCommitment = expectedCommitment.Add(term)
	}

	// Verify that computed commitment equals expected linear combination
	return computedCommitment.Equal(expectedCommitment) == 1
}

// Helper methods

// generatePoint generates a deterministic point using the group's GeneratePoint method
func (pv *PedersenVerifier) generatePoint(index int) group.GroupElement {
	seed := []byte(fmt.Sprintf("pedersen_generator_%d", index))
	return pv.group.GeneratePoint(seed)
}

// splitIntoElements splits data into field elements using the field package
func (pv *PedersenVerifier) splitIntoElements(data []byte) []field.Element {
	// Use the field package to split bits into field elements
	return field.SplitBitsToFieldElements(data, pv.networkBitsPerElement, pv.scalarField)
}

// commit creates a Pedersen commitment for the given field elements
func (pv *PedersenVerifier) commit(values []field.Element) group.GroupElement {
	result := pv.group.Zero()

	// Add each value term: Σ(vi * Gi)
	for i, value := range values {
		if i >= len(pv.generators) {
			break
		}

		// Convert field element to scalar
		valueScalar := pv.group.ScalarFromFieldElement(value)
		term := pv.generators[i].ScalarMult(valueScalar)
		result = result.Add(term)
	}

	return result
}

// parseCommitment converts bytes back to a group element
func (pv *PedersenVerifier) parseCommitment(data []byte) group.GroupElement {
	if len(data) != pv.group.ElementSize() {
		return nil
	}

	element := pv.group.NewElement()
	if err := element.Decode(data); err != nil {
		return nil // Invalid encoding
	}

	return element
}

// parseExtraData parses the extra field into commitments and BLS signature
func (pv *PedersenVerifier) parseExtraData(data []byte) (*ExtraData, error) {
	elementSize := pv.group.ElementSize()
	minSize := 8 + elementSize + 96 // 4 bytes publisher + 4 bytes count + 1 commitment + BLS signature
	if len(data) < minSize {
		return nil, fmt.Errorf("extra data too short: %d bytes", len(data))
	}

	offset := 0

	// First 4 bytes contain the publisher ID
	publisherID := int(binary.BigEndian.Uint32(data[offset : offset+4]))
	offset += 4

	// Next 4 bytes contain the number of commitments
	numCommitments := binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4

	expectedSize := 8 + (numCommitments * uint32(elementSize)) + 96 // 4 bytes publisher + 4 bytes count + commitments + BLS signature
	if len(data) != int(expectedSize) {
		return nil, fmt.Errorf("invalid extra data size: expected %d, got %d", expectedSize, len(data))
	}

	commitments := make([][]byte, numCommitments)

	// Parse commitments
	for i := uint32(0); i < numCommitments; i++ {
		commitments[i] = make([]byte, elementSize)
		copy(commitments[i], data[offset:offset+elementSize])
		offset += elementSize
	}

	// Parse BLS signature (last 96 bytes)
	blsSignature := make([]byte, 96)
	copy(blsSignature, data[offset:offset+96])

	return &ExtraData{
		PublisherID:  publisherID,
		Commitments:  commitments,
		BLSSignature: blsSignature,
	}, nil
}

// serializeCommitment converts a group element to bytes
func (pv *PedersenVerifier) serializeCommitment(element group.GroupElement) []byte {
	return element.Encode()
}

// serializeExtraData serializes the extra data structure to bytes
func (pv *PedersenVerifier) serializeExtraData(extraData *ExtraData) []byte {
	numCommitments := uint32(len(extraData.Commitments))
	elementSize := pv.group.ElementSize()
	totalSize := 8 + (numCommitments * uint32(elementSize)) + 96 // 4 bytes publisher + 4 bytes count + commitments + signature

	result := make([]byte, totalSize)
	offset := 0

	// Write publisher ID
	binary.BigEndian.PutUint32(result[offset:offset+4], uint32(extraData.PublisherID))
	offset += 4

	// Write number of commitments
	binary.BigEndian.PutUint32(result[offset:offset+4], numCommitments)
	offset += 4

	// Write commitments
	for _, commitment := range extraData.Commitments {
		if len(commitment) != elementSize {
			panic(fmt.Sprintf("commitment must be %d bytes", elementSize))
		}
		copy(result[offset:offset+elementSize], commitment)
		offset += elementSize
	}

	// Write BLS signature
	if len(extraData.BLSSignature) != 96 {
		panic("BLS signature must be 96 bytes")
	}
	copy(result[offset:offset+96], extraData.BLSSignature)

	return result
}

// generateBlsSignature generates a BLS signature over the commitments
func (pv *PedersenVerifier) generateBLSSignature(commitments [][]byte) []byte {
	// If no BLS secret key is provided, return empty signature
	if pv.blsSecretKey == nil {
		return make([]byte, 96) // BLS signatures are 96 bytes in BLS12-381
	}

	// Create message to sign by hashing all commitments
	hasher := sha256.New()
	for _, commitment := range commitments {
		hasher.Write(commitment)
	}
	message := hasher.Sum(nil)

	// Sign the message
	sig := pv.blsSecretKey.SignByte(message)
	return sig.Serialize()
}

// verifyBlsSignature verifies a BLS signature over the commitments using the callback
func (pv *PedersenVerifier) verifyBLSSignature(commitments [][]byte, signature []byte, publisherID int) bool {
	// If no callback provided, skip verification
	if pv.publicKeyCallback == nil {
		return true
	}

	// Get public key from callback using publisher ID
	publicKey := pv.publicKeyCallback(publisherID)
	if publicKey == nil {
		return false // No public key available
	}

	// Create message by hashing all commitments
	hasher := sha256.New()
	for _, commitment := range commitments {
		hasher.Write(commitment)
	}
	message := hasher.Sum(nil)

	// Deserialize signature
	sig := &bls.Sign{}
	if err := sig.Deserialize(signature); err != nil {
		return false
	}

	// Verify signature
	return sig.VerifyByte(publicKey, message)
}
