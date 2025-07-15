package field

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// BinaryField represents a binary finite field GF(2^n)
type BinaryField struct {
	n           int      // field extension degree
	irreducible *big.Int // irreducible polynomial
}

// NewBinaryField creates a new binary field GF(2^n) with given irreducible polynomial
func NewBinaryField(n int, irreducible *big.Int) *BinaryField {
	return &BinaryField{
		n:           n,
		irreducible: new(big.Int).Set(irreducible),
	}
}

// NewBinaryFieldGF2_32 creates GF(2^32) with irreducible polynomial x^32 + x^7 + x^3 + x^2 + 1
func NewBinaryFieldGF2_32() *BinaryField {
	// x^32 + x^7 + x^3 + x^2 + 1 = 0x10000008D
	irreducible := big.NewInt(0x10000008D)
	return NewBinaryField(32, irreducible)
}

// BinaryFieldElement represents an element in a binary field
type BinaryFieldElement struct {
	value *big.Int     // polynomial representation
	field *BinaryField // reference to parent field
}

// Field interface implementation for BinaryField

// Zero returns the additive identity element (0)
func (f *BinaryField) Zero() Element {
	return &BinaryFieldElement{
		value: big.NewInt(0),
		field: f,
	}
}

// One returns the multiplicative identity element (1)
func (f *BinaryField) One() Element {
	return &BinaryFieldElement{
		value: big.NewInt(1),
		field: f,
	}
}

// Random returns a uniformly random field element
func (f *BinaryField) Random() (Element, error) {
	max := new(big.Int).Lsh(big.NewInt(1), uint(f.n))
	val, err := rand.Int(rand.Reader, max)
	if err != nil {
		return nil, err
	}
	return &BinaryFieldElement{
		value: val,
		field: f,
	}, nil
}

// RandomMax returns a random element with value < 2^maxBits
func (f *BinaryField) RandomMax(maxBits int) (Element, error) {
	max := new(big.Int).Lsh(big.NewInt(1), uint(maxBits))
	val, err := rand.Int(rand.Reader, max)
	if err != nil {
		return nil, err
	}

	// Ensure the value fits in the field
	fieldMax := new(big.Int).Lsh(big.NewInt(1), uint(f.n))
	val.Mod(val, fieldMax)

	return &BinaryFieldElement{
		value: val,
		field: f,
	}, nil
}

// FromBytes creates a field element from byte array
func (f *BinaryField) FromBytes(data []byte) Element {
	val := new(big.Int).SetBytes(data)
	// Ensure the value fits in the field
	fieldMax := new(big.Int).Lsh(big.NewInt(1), uint(f.n))
	val.Mod(val, fieldMax)
	return &BinaryFieldElement{
		value: val,
		field: f,
	}
}

// FromBits creates a field element from bits with specified bit length
func (f *BinaryField) FromBits(data []byte, bitLen int) Element {
	val := big.NewInt(0)
	for i := 0; i < bitLen; i++ {
		byteIdx := i / 8
		bitIdx := i % 8
		if byteIdx < len(data) && (data[byteIdx]&(1<<(7-bitIdx))) != 0 {
			val.SetBit(val, bitLen-1-i, 1)
		}
	}
	// Ensure the value fits in the field
	fieldMax := new(big.Int).Lsh(big.NewInt(1), uint(f.n))
	val.Mod(val, fieldMax)
	return &BinaryFieldElement{
		value: val,
		field: f,
	}
}

// BitsPerDataElement returns usable bits per element for data encoding
func (f *BinaryField) BitsPerDataElement() int {
	return f.n
}

// BitsPerElement returns the number of bits per field element
func (f *BinaryField) BitsPerElement() int {
	return f.n
}

// BinaryFieldElement methods implementing Element interface

// Add returns e + b in the field (XOR operation)
func (e *BinaryFieldElement) Add(b Element) Element {
	other, ok := b.(*BinaryFieldElement)
	if !ok {
		panic("incompatible field elements")
	}

	result := new(big.Int).Xor(e.value, other.value)

	return &BinaryFieldElement{
		value: result,
		field: e.field,
	}
}

// Sub returns e - b in the field (same as Add in GF(2^n))
func (e *BinaryFieldElement) Sub(b Element) Element {
	return e.Add(b)
}

// Mul returns e * b in the field using polynomial multiplication with reduction
func (e *BinaryFieldElement) Mul(b Element) Element {
	other, ok := b.(*BinaryFieldElement)
	if !ok {
		panic("incompatible field elements")
	}

	// Polynomial multiplication in GF(2)
	result := big.NewInt(0)
	a := new(big.Int).Set(e.value)
	b_val := new(big.Int).Set(other.value)

	for b_val.Sign() > 0 {
		if b_val.Bit(0) == 1 {
			result.Xor(result, a)
		}
		a.Lsh(a, 1)
		b_val.Rsh(b_val, 1)
	}

	// Reduce by irreducible polynomial
	result = e.reduce(result)

	return &BinaryFieldElement{
		value: result,
		field: e.field,
	}
}

// reduce performs polynomial reduction modulo the irreducible polynomial
func (e *BinaryFieldElement) reduce(val *big.Int) *big.Int {
	result := new(big.Int).Set(val)

	for result.BitLen() > e.field.n {
		// Find the highest bit position
		pos := result.BitLen() - 1

		// If bit is set, XOR with irreducible polynomial shifted appropriately
		if result.Bit(pos) == 1 {
			shift := pos - e.field.irreducible.BitLen() + 1
			if shift >= 0 {
				temp := new(big.Int).Lsh(e.field.irreducible, uint(shift))
				result.Xor(result, temp)
			}
		}

		// Clear the bit if it's still set (shouldn't happen with correct reduction)
		if result.Bit(pos) == 1 {
			result.SetBit(result, pos, 0)
		}
	}

	return result
}

// Inv returns the multiplicative inverse of e using extended Euclidean algorithm
func (e *BinaryFieldElement) Inv() Element {
	if e.IsZero() {
		panic("zero element is not invertible")
	}

	// Extended Euclidean algorithm for polynomials over GF(2)
	old_r := new(big.Int).Set(e.field.irreducible)
	r := new(big.Int).Set(e.value)
	old_s := big.NewInt(0)
	s := big.NewInt(1)

	for r.Sign() > 0 {
		// Polynomial division in GF(2)
		q, remainder := e.polyDivMod(old_r, r)

		old_r.Set(r)
		r.Set(remainder)

		old_s, s = s, new(big.Int).Xor(old_s, e.polyMul(q, s))
	}

	// Ensure result is positive and in field
	if old_s.Sign() < 0 {
		old_s.Add(old_s, e.field.irreducible)
	}

	return &BinaryFieldElement{
		value: old_s,
		field: e.field,
	}
}

// polyDivMod performs polynomial division in GF(2)
func (e *BinaryFieldElement) polyDivMod(a, b *big.Int) (*big.Int, *big.Int) {
	if b.Sign() == 0 {
		panic("division by zero polynomial")
	}

	quotient := big.NewInt(0)
	remainder := new(big.Int).Set(a)

	bDegree := b.BitLen() - 1

	for remainder.BitLen() > bDegree {
		rDegree := remainder.BitLen() - 1
		shift := rDegree - bDegree

		quotient.SetBit(quotient, shift, 1)
		temp := new(big.Int).Lsh(b, uint(shift))
		remainder.Xor(remainder, temp)
	}

	return quotient, remainder
}

// polyMul performs polynomial multiplication in GF(2)
func (e *BinaryFieldElement) polyMul(a, b *big.Int) *big.Int {
	result := big.NewInt(0)
	temp_a := new(big.Int).Set(a)
	temp_b := new(big.Int).Set(b)

	for temp_b.Sign() > 0 {
		if temp_b.Bit(0) == 1 {
			result.Xor(result, temp_a)
		}
		temp_a.Lsh(temp_a, 1)
		temp_b.Rsh(temp_b, 1)
	}

	return result
}

// IsZero returns true if e equals zero
func (e *BinaryFieldElement) IsZero() bool {
	return e.value.Sign() == 0
}

// Equal returns true if e equals b
func (e *BinaryFieldElement) Equal(b Element) bool {
	other, ok := b.(*BinaryFieldElement)
	if !ok {
		return false
	}

	return e.value.Cmp(other.value) == 0
}

// Clone returns a copy of e
func (e *BinaryFieldElement) Clone() Element {
	return &BinaryFieldElement{
		value: new(big.Int).Set(e.value),
		field: e.field,
	}
}

// Bytes returns the byte representation of e
func (e *BinaryFieldElement) Bytes() []byte {
	return e.value.Bytes()
}

// Bits returns the bit representation with specified bit length
func (e *BinaryFieldElement) Bits(bitLen int) []byte {
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
func (e *BinaryFieldElement) String() string {
	return fmt.Sprintf("0x%x", e.value)
}

// BigInt returns the underlying big.Int value
func (e *BinaryFieldElement) BigInt() *big.Int {
	return new(big.Int).Set(e.value)
}
