package rs

import (
	"github.com/ethp2p/eth-ec-broadcast/ec/field"
)

// ChunkVerifier provides verification functionality for Reed-Solomon chunks
type ChunkVerifier interface {
	// Verify checks if the chunk is valid
	Verify(chunk *Chunk) bool

	// GenerateExtra generates verification data for a chunk given data polynomials and evaluation point
	// polynomials contains the field elements for each data chunk as polynomials in evaluation form
	// evalPoint is the evaluation point for this particular chunk
	GenerateExtra(polynomials [][]field.Element, evalPoint field.Element) ([]byte, error)
}
