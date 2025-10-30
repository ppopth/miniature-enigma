package field

import "math/big"

// Element represents an element in a finite field
type Element interface {
	// Add returns a + b in the field
	Add(b Element) Element

	// Sub returns a - b in the field
	Sub(b Element) Element

	// Mul returns a * b in the field
	Mul(b Element) Element

	// Inv returns the multiplicative inverse of a in the field
	Inv() Element

	// IsZero returns true if the element is the zero element
	IsZero() bool

	// Equal returns true if two elements are equal
	Equal(b Element) bool

	// Clone returns a copy of the element
	Clone() Element

	// Bytes returns the byte representation of the element
	Bytes() []byte

	// Bits returns the bit representation with specified bit length
	Bits(bitLen int) []byte

	// String returns the string representation of the element
	String() string
}

// Field represents a finite field
type Field interface {
	// Zero returns the zero element of the field
	Zero() Element

	// One returns the one element of the field
	One() Element

	// Random returns a random element in the field
	Random() (Element, error)

	// RandomMax returns a random element with maximum number of bits
	RandomMax(maxBits int) (Element, error)

	// FromBytes creates a field element from bytes
	FromBytes(data []byte) Element

	// FromBits creates a field element from bits with specified bit length
	FromBits(data []byte, bitLen int) Element

	// BitsPerDataElement returns the number of bits per data element
	BitsPerDataElement() int

	// BitsPerElement returns the number of bits per field element
	BitsPerElement() int

	// Order returns the order (size) of the field
	Order() *big.Int
}
