package group

import (
	"math/big"

	"github.com/ethp2p/eth-ec-broadcast/ec/field"
)

// GroupElement represents an element in a cryptographic group
type GroupElement interface {
	// Add returns a + b in the group
	Add(b GroupElement) GroupElement

	// ScalarMult returns scalar * this element
	ScalarMult(scalar Scalar) GroupElement

	// Equal returns true if two elements are equal
	Equal(b GroupElement) int

	// Encode returns the byte representation of the element
	Encode() []byte

	// Decode decodes bytes into the element, returns error if invalid
	Decode(data []byte) error
}

// Scalar represents a scalar for group operations
type Scalar interface {
	// Decode decodes bytes into the scalar, returns error if invalid
	Decode(data []byte) error

	// Encode returns the byte representation of the scalar
	Encode() []byte
}

// PrimeOrderGroup represents a prime-order cryptographic group for Pedersen commitments
type PrimeOrderGroup interface {
	// NewElement creates a new group element
	NewElement() GroupElement

	// NewScalar creates a new scalar
	NewScalar() Scalar

	// Zero returns the zero element of the group
	Zero() GroupElement

	// GeneratePoint generates a deterministic point from seed data
	GeneratePoint(seed []byte) GroupElement

	// ElementSize returns the size in bytes of encoded group elements
	ElementSize() int

	// ScalarFromFieldElement converts a field element to a scalar
	ScalarFromFieldElement(elem field.Element) Scalar

	// Order returns the order of the group as a big integer
	Order() *big.Int
}

// NewScalarField creates the scalar field for a prime-order group
// For any prime-order group, the scalar field is the prime field with modulus equal to the group order
func NewScalarField(g PrimeOrderGroup) field.Field {
	return field.NewPrimeField(g.Order())
}
