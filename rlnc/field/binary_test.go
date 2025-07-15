package field

import (
	"math/big"
	"testing"
)

// Test helper function
func setupBinaryField() *BinaryField {
	return NewBinaryFieldGF2_32()
}

// TestBinaryFieldBasic tests basic operations
func TestBinaryFieldBasic(t *testing.T) {
	field := setupBinaryField()

	// Test zero and one elements
	zero := field.Zero()
	one := field.One()

	if !zero.IsZero() {
		t.Errorf("Zero element should be zero")
	}
	if one.IsZero() {
		t.Errorf("One element should not be zero")
	}

	// Test addition (XOR)
	a := field.FromBytes([]byte{0x12, 0x34, 0x56, 0x78})
	b := field.FromBytes([]byte{0x9A, 0xBC, 0xDE, 0xF0})
	result := a.Add(b)

	// Should be XOR of the values: 0x12345678 ^ 0x9ABCDEF0 = 0x88888888
	expected := field.FromBytes([]byte{0x88, 0x88, 0x88, 0x88})
	if !result.Equal(expected) {
		t.Errorf("Addition failed: expected %s, got %s", expected, result)
	}

	// Test subtraction (same as addition in GF(2^n))
	result = a.Sub(b)
	if !result.Equal(expected) {
		t.Errorf("Subtraction failed: expected %s, got %s", expected, result)
	}

	// Test multiplication by one
	result = a.Mul(one)
	if !result.Equal(a) {
		t.Errorf("Multiplication by one failed: expected %s, got %s", a, result)
	}

	// Test multiplication by zero
	result = a.Mul(zero)
	if !result.Equal(zero) {
		t.Errorf("Multiplication by zero failed: expected %s, got %s", zero, result)
	}

	// Test addition with zero
	result = a.Add(zero)
	if !result.Equal(a) {
		t.Errorf("Addition with zero failed: expected %s, got %s", a, result)
	}

	// Test self-addition (should be zero)
	result = a.Add(a)
	if !result.Equal(zero) {
		t.Errorf("Self addition should be zero: got %s", result)
	}
}

// TestBinaryFieldInversion tests multiplicative inverse
func TestBinaryFieldInversion(t *testing.T) {
	field := setupBinaryField()
	one := field.One()

	testCases := [][]byte{
		{0x00, 0x00, 0x00, 0x01},
		{0x00, 0x00, 0x00, 0x02},
		{0x00, 0x00, 0x00, 0x03},
		{0x12, 0x34, 0x56, 0x78},
		{0xFF, 0xFF, 0xFF, 0xFF},
	}

	for _, val := range testCases {
		t.Run("", func(t *testing.T) {
			a := field.FromBytes(val)
			inv := a.Inv()

			// Check that a * a^(-1) = 1
			result := a.Mul(inv)
			if !result.Equal(one) {
				t.Errorf("Inversion failed for %x: a * a^(-1) = %s, expected %s", val, result, one)
			}
		})
	}
}

// TestBinaryFieldZeroInversion tests that zero inversion panics
func TestBinaryFieldZeroInversion(t *testing.T) {
	field := setupBinaryField()
	zero := field.Zero()

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Zero inversion should panic")
		}
	}()

	zero.Inv()
}

// TestBinaryFieldElementMethods tests various element methods
func TestBinaryFieldElementMethods(t *testing.T) {
	field := setupBinaryField()

	// Test BitsPerDataElement
	if field.BitsPerDataElement() != 32 {
		t.Errorf("Binary field should have 32 bits per element, got %d", field.BitsPerDataElement())
	}

	// Test basic element properties
	zero := field.Zero()
	one := field.One()

	if !zero.IsZero() {
		t.Errorf("Zero element should be zero")
	}
	if one.IsZero() {
		t.Errorf("One element should not be zero")
	}

	// Test clone
	oneClone := one.Clone()
	if !one.Equal(oneClone) {
		t.Errorf("Clone should be equal to original")
	}

	// Test string representation
	a := field.FromBytes([]byte{0x12, 0x34, 0x56, 0x78})
	str := a.String()
	if str != "0x12345678" {
		t.Errorf("String representation failed: expected '0x12345678', got '%s'", str)
	}
}

// TestBinaryFieldFromBitsAndBits tests FromBits() and Bits() methods
func TestBinaryFieldFromBitsAndBits(t *testing.T) {
	field := setupBinaryField()

	testCases := []struct {
		data   []byte
		bitLen int
		desc   string
	}{
		{[]byte{0x12, 0x34, 0x56, 0x78}, 32, "full_32_bits"},
		{[]byte{0xFF, 0x00, 0x00, 0x00}, 8, "first_8_bits"},
		{[]byte{0x80, 0x00, 0x00, 0x00}, 1, "single_bit"},
		{[]byte{0x0F, 0xFF, 0x00, 0x00}, 16, "first_16_bits"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			elem := field.FromBits(tc.data, tc.bitLen)
			recovered := elem.Bits(tc.bitLen)

			// Compare the relevant bits
			for bit := 0; bit < tc.bitLen; bit++ {
				srcByteIdx := bit / 8
				srcBitIdx := bit % 8

				// Extract original bit
				origBit := false
				if srcByteIdx < len(tc.data) {
					origBit = (tc.data[srcByteIdx] & (1 << (7 - srcBitIdx))) != 0
				}

				// Extract recovered bit
				recBit := false
				if srcByteIdx < len(recovered) {
					recBit = (recovered[srcByteIdx] & (1 << (7 - srcBitIdx))) != 0
				}

				if origBit != recBit {
					t.Errorf("Bit %d mismatch for data %x bitLen %d: orig=%v, recovered=%v",
						bit, tc.data, tc.bitLen, origBit, recBit)
				}
			}
		})
	}
}

// TestBinaryFieldRandom tests random element generation
func TestBinaryFieldRandom(t *testing.T) {
	field := setupBinaryField()

	for i := 0; i < 5; i++ {
		elem, err := field.Random()
		if err != nil {
			t.Fatalf("Random element generation failed: %v", err)
		}

		// Check that element operations work
		result := elem.Add(elem)
		if !result.IsZero() {
			t.Errorf("a + a should be zero in binary field, got %s", result)
		}

		// Test multiplication by one
		one := field.One()
		result = elem.Mul(one)
		if !result.Equal(elem) {
			t.Errorf("a * 1 should equal a in binary field")
		}
	}
}

// TestBinaryFieldRandomMax tests random element generation with max bits
func TestBinaryFieldRandomMax(t *testing.T) {
	field := setupBinaryField()
	maxBits := 16

	for i := 0; i < 10; i++ {
		elem, err := field.RandomMax(maxBits)
		if err != nil {
			t.Fatalf("RandomMax failed: %v", err)
		}

		// Check that element fits in maxBits (within field bounds)
		bytes := elem.Bytes()
		if len(bytes) > 4 {
			t.Errorf("RandomMax element too large: %s", elem)
		}
	}
}

// TestBinaryFieldBytes tests byte representation round-trip
func TestBinaryFieldBytes(t *testing.T) {
	field := setupBinaryField()

	testCases := [][]byte{
		{0x00, 0x00, 0x00, 0x00},
		{0x00, 0x00, 0x00, 0x01},
		{0x12, 0x34, 0x56, 0x78},
		{0xFF, 0xFF, 0xFF, 0xFF},
	}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			elem := field.FromBytes(tc)
			bytes := elem.Bytes()

			if len(bytes) == 0 && tc[0] == 0 && tc[1] == 0 && tc[2] == 0 && tc[3] == 0 {
				// Zero element returns empty bytes, which is acceptable
				return
			}

			// Check that leading zeros are handled correctly
			expected := tc
			for len(expected) > 0 && expected[0] == 0 {
				expected = expected[1:]
			}

			if len(expected) == 0 {
				// All zeros case
				if len(bytes) != 0 {
					t.Errorf("All-zero bytes should return empty, got %v", bytes)
				}
			} else {
				// Compare non-zero bytes
				if len(bytes) != len(expected) {
					t.Errorf("Bytes length mismatch for %x: expected %d, got %d", tc, len(expected), len(bytes))
				} else {
					for i := range bytes {
						if bytes[i] != expected[i] {
							t.Errorf("Bytes mismatch for %x: expected %v, got %v", tc, expected, bytes)
							break
						}
					}
				}
			}
		})
	}
}

// TestBinaryFieldIncompatibleElements tests that operations with incompatible elements panic
func TestBinaryFieldIncompatibleElements(t *testing.T) {
	binaryField := setupBinaryField()
	primeField := NewPrimeField(big.NewInt(101))

	binaryElem := binaryField.FromBytes([]byte{0x12, 0x34, 0x56, 0x78})
	primeElem := primeField.FromBytes([]byte{0x53})

	// Test Add with incompatible elements
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Add with incompatible elements should panic")
		}
	}()

	binaryElem.Add(primeElem)
}

// TestBinaryFieldCustomIrreducible tests binary field with custom irreducible polynomial
func TestBinaryFieldCustomIrreducible(t *testing.T) {
	// Create GF(2^4) with irreducible polynomial x^4 + x + 1 = 0x13
	field := NewBinaryField(4, big.NewInt(0x13))

	// Test that field operations work
	a := field.FromBytes([]byte{0x05}) // 0101
	b := field.FromBytes([]byte{0x03}) // 0011

	result := a.Add(b)
	expected := field.FromBytes([]byte{0x06}) // 0110 (0101 ^ 0011)

	if !result.Equal(expected) {
		t.Errorf("Custom field addition failed: expected %s, got %s", expected, result)
	}

	// Test multiplication - result should be in field bounds
	result = a.Mul(b)
	bytes := result.Bytes()
	if len(bytes) > 1 || (len(bytes) == 1 && bytes[0] >= 16) {
		t.Errorf("Custom field multiplication result out of bounds: %s", result)
	}
}
