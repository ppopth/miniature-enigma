package rlnc

import (
	"github.com/ethp2p/eth-ec-broadcast/ec/field"
)

// ChunkVerifier provides verification functionality for RLNC chunks
type ChunkVerifier interface {
	// Verify checks if the chunk is valid
	Verify(chunk *Chunk) bool

	// CombineExtras combines verification data from multiple chunks using linear coefficients.
	// Used when creating linear combinations: if C = a1*C1 + a2*C2, then
	// Extra = CombineExtras([C1, C2], [a1, a2])
	CombineExtras(chunks []Chunk, coefficients []field.Element) ([]byte, error)

	// GenerateExtras generates verification data for original message chunks
	GenerateExtras(chunkDatas [][]byte) ([][]byte, error)
}
