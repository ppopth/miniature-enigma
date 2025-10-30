package field

import (
	"crypto/rand"
	"math/big"
)

// PrimeField represents a prime finite field F_p
type PrimeField struct {
	p *big.Int // the prime modulus
}

// NewPrimeField creates a new prime field
func NewPrimeField(p *big.Int) *PrimeField {
	return &PrimeField{p: new(big.Int).Set(p)}
}

// PrimeFieldElement represents an element in a prime field
type PrimeFieldElement struct {
	value *big.Int    // element value in range [0, p-1]
	field *PrimeField // reference to parent field
}

// Field interface implementation for PrimeField

// Zero returns the additive identity element (0)
func (f *PrimeField) Zero() Element {
	return &PrimeFieldElement{
		value: big.NewInt(0),
		field: f,
	}
}

// One returns the multiplicative identity element (1)
func (f *PrimeField) One() Element {
	return &PrimeFieldElement{
		value: big.NewInt(1),
		field: f,
	}
}

// Random returns a uniformly random field element
func (f *PrimeField) Random() (Element, error) {
	val, err := rand.Int(rand.Reader, f.p)
	if err != nil {
		return nil, err
	}
	return &PrimeFieldElement{
		value: val,
		field: f,
	}, nil
}

// RandomMax returns a random element with value < 2^maxBits
func (f *PrimeField) RandomMax(maxBits int) (Element, error) {
	max := new(big.Int).Lsh(big.NewInt(1), uint(maxBits))
	val, err := rand.Int(rand.Reader, max)
	if err != nil {
		return nil, err
	}

	// Ensure the value is in the field
	val.Mod(val, f.p)

	return &PrimeFieldElement{
		value: val,
		field: f,
	}, nil
}

// FromBytes creates a field element from byte array
func (f *PrimeField) FromBytes(data []byte) Element {
	val := new(big.Int).SetBytes(data)
	val.Mod(val, f.p)
	return &PrimeFieldElement{
		value: val,
		field: f,
	}
}

// FromBits creates a field element from bits with specified bit length
func (f *PrimeField) FromBits(data []byte, bitLen int) Element {
	// Extract exactly bitLen bits from data and build big.Int value
	val := big.NewInt(0)
	for i := 0; i < bitLen; i++ {
		byteIdx := i / 8
		bitIdx := i % 8
		if byteIdx < len(data) && (data[byteIdx]&(1<<(7-bitIdx))) != 0 {
			// Set bit in big.Int, preserving bit order
			val.SetBit(val, bitLen-1-i, 1)
		}
	}
	val.Mod(val, f.p)
	return &PrimeFieldElement{
		value: val,
		field: f,
	}
}

// BitsPerDataElement returns usable bits per element for data encoding
func (f *PrimeField) BitsPerDataElement() int {
	return f.p.BitLen() - 1
}

// BitsPerElement returns the number of bits per field element
func (f *PrimeField) BitsPerElement() int {
	return f.p.BitLen()
}

// Order returns the order (size) of the field, which is p for a prime field
func (f *PrimeField) Order() *big.Int {
	return new(big.Int).Set(f.p)
}

// PrimeFieldElement methods implementing Element interface

// Add returns e + b in the field
func (e *PrimeFieldElement) Add(b Element) Element {
	other, ok := b.(*PrimeFieldElement)
	if !ok {
		panic("incompatible field elements")
	}

	result := new(big.Int).Add(e.value, other.value)
	result.Mod(result, e.field.p)

	return &PrimeFieldElement{
		value: result,
		field: e.field,
	}
}

// Sub returns e - b in the field
func (e *PrimeFieldElement) Sub(b Element) Element {
	other, ok := b.(*PrimeFieldElement)
	if !ok {
		panic("incompatible field elements")
	}

	result := new(big.Int).Sub(e.value, other.value)
	result.Mod(result, e.field.p)

	return &PrimeFieldElement{
		value: result,
		field: e.field,
	}
}

// Mul returns e * b in the field
func (e *PrimeFieldElement) Mul(b Element) Element {
	other, ok := b.(*PrimeFieldElement)
	if !ok {
		panic("incompatible field elements")
	}

	result := new(big.Int).Mul(e.value, other.value)
	result.Mod(result, e.field.p)

	return &PrimeFieldElement{
		value: result,
		field: e.field,
	}
}

// Inv returns the multiplicative inverse of e
func (e *PrimeFieldElement) Inv() Element {
	inv := new(big.Int).ModInverse(e.value, e.field.p)
	if inv == nil {
		panic("element is not invertible")
	}

	return &PrimeFieldElement{
		value: inv,
		field: e.field,
	}
}

// IsZero returns true if e equals zero
func (e *PrimeFieldElement) IsZero() bool {
	return e.value.Cmp(big.NewInt(0)) == 0
}

// Equal returns true if e equals b
func (e *PrimeFieldElement) Equal(b Element) bool {
	other, ok := b.(*PrimeFieldElement)
	if !ok {
		return false
	}

	return e.value.Cmp(other.value) == 0
}

// Clone returns a copy of e
func (e *PrimeFieldElement) Clone() Element {
	return &PrimeFieldElement{
		value: new(big.Int).Set(e.value),
		field: e.field,
	}
}

// Bytes returns the byte representation of e
func (e *PrimeFieldElement) Bytes() []byte {
	return e.value.Bytes()
}

// Bits returns the bit representation with specified bit length
func (e *PrimeFieldElement) Bits(bitLen int) []byte {
	// Convert to bits with exact bitLen length
	result := make([]byte, (bitLen+7)/8)
	for i := 0; i < bitLen; i++ {
		if e.value.Bit(bitLen-1-i) == 1 {
			byteIdx := i / 8
			bitIdx := i % 8
			result[byteIdx] |= 1 << (7 - bitIdx)
		}
	}
	return result
}

// String returns the string representation of e
func (e *PrimeFieldElement) String() string {
	return e.value.String()
}

// BigInt returns the underlying big.Int value
func (e *PrimeFieldElement) BigInt() *big.Int {
	return new(big.Int).Set(e.value)
}
