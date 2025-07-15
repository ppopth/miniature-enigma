package rlnc

import (
	"github.com/ppopth/p2p-broadcast/rlnc/field"
)

type ChunkVerifyAlgorithm interface {
	// Verify if the chunk is valid.
	Verify(*Chunk) bool
	// Given a list of chunks, produce the extra field. Return an error, if applicable.
	CombineExtras([]Chunk, []field.Element) ([]byte, error)
	// Given chunk datas, generate extra fields for all chunks.
	GenerateExtras([][]byte) ([][]byte, error)
}
