package group

import (
	"crypto/sha256"
	"fmt"
	"math/big"

	"github.com/ethp2p/eth-ec-broadcast/ec/field"
	"github.com/gtank/ristretto255"
)

// Ristretto255GroupElement wraps ristretto255.Element to implement GroupElement
type Ristretto255GroupElement struct {
	element *ristretto255.Element
}

func (e *Ristretto255GroupElement) Add(b GroupElement) GroupElement {
	other := b.(*Ristretto255GroupElement)
	result := ristretto255.NewElement()
	result.Add(e.element, other.element)
	return &Ristretto255GroupElement{element: result}
}

func (e *Ristretto255GroupElement) ScalarMult(scalar Scalar) GroupElement {
	s := scalar.(*Ristretto255Scalar)
	result := ristretto255.NewElement()
	result.ScalarMult(s.scalar, e.element)
	return &Ristretto255GroupElement{element: result}
}

func (e *Ristretto255GroupElement) Equal(b GroupElement) int {
	other := b.(*Ristretto255GroupElement)
	return e.element.Equal(other.element)
}

func (g *Ristretto255Group) Zero() GroupElement {
	result := ristretto255.NewElement().Zero()
	return &Ristretto255GroupElement{element: result}
}

func (e *Ristretto255GroupElement) Encode() []byte {
	return e.element.Encode(nil)
}

func (e *Ristretto255GroupElement) Decode(data []byte) error {
	return e.element.Decode(data)
}

// Ristretto255Scalar wraps ristretto255.Scalar to implement Scalar
type Ristretto255Scalar struct {
	scalar *ristretto255.Scalar
}

func (s *Ristretto255Scalar) Decode(data []byte) error {
	return s.scalar.Decode(data)
}

func (s *Ristretto255Scalar) Encode() []byte {
	return s.scalar.Encode(nil)
}

// Ristretto255Group implements PrimeOrderGroup interface for Ristretto255
type Ristretto255Group struct {
}

func (g *Ristretto255Group) NewElement() GroupElement {
	return &Ristretto255GroupElement{element: ristretto255.NewElement()}
}

func (g *Ristretto255Group) NewScalar() Scalar {
	return &Ristretto255Scalar{scalar: ristretto255.NewScalar()}
}

func (g *Ristretto255Group) GeneratePoint(seed []byte) GroupElement {
	// Use hash-to-group approach for generating deterministic points
	hash := sha256.Sum256(seed)

	// Expand to 64 bytes using a second hash
	hash2 := sha256.Sum256(append(hash[:], seed...))
	uniformBytes := append(hash[:], hash2[:]...)

	element := ristretto255.NewElement()
	element.FromUniformBytes(uniformBytes)

	return &Ristretto255GroupElement{element: element}
}

func (g *Ristretto255Group) ElementSize() int {
	return 32 // Ristretto255 elements are 32 bytes
}

func (g *Ristretto255Group) ScalarFromFieldElement(elem field.Element) Scalar {
	// Get the byte representation of the field element (big-endian)
	data := elem.Bytes()

	// Create a new scalar and decode the bytes
	scalar := ristretto255.NewScalar()

	// Ristretto255 expects little-endian bytes, but field elements give us big-endian
	var scalarBytes [32]byte

	// Convert big-endian to little-endian by reversing and padding
	for i := 0; i < len(data) && i < 32; i++ {
		scalarBytes[i] = data[len(data)-1-i]
	}

	// Decode the bytes as a scalar
	if err := scalar.Decode(scalarBytes[:]); err != nil {
		panic(fmt.Sprintf("failed to decode field element as scalar: %v", err))
	}

	return &Ristretto255Scalar{scalar: scalar}
}

func (g *Ristretto255Group) Order() *big.Int {
	// l = 2^252 + 27742317777372353535851937790883648493
	groupOrder := new(big.Int)
	groupOrder.Exp(big.NewInt(2), big.NewInt(252), nil)
	addend := new(big.Int)
	addend.SetString("27742317777372353535851937790883648493", 10)
	groupOrder.Add(groupOrder, addend)
	return groupOrder
}

// NewRistretto255Group creates a new Ristretto255 group
func NewRistretto255Group() *Ristretto255Group {
	return &Ristretto255Group{}
}
