package field

import (
	"math/big"
	"testing"
)

// Test helper functions
func setupPrimeField() *PrimeField {
	return NewPrimeField(big.NewInt(101))
}

func setupLargePrimeField() *PrimeField {
	return NewPrimeField(big.NewInt(4_294_967_311))
}

// TestPrimeFieldBasic tests basic operations
func TestPrimeFieldBasic(t *testing.T) {
	field := setupPrimeField()

	// Test zero and one elements
	zero := field.Zero()
	one := field.One()

	if !zero.IsZero() {
		t.Errorf("Zero element should be zero")
	}
	if one.IsZero() {
		t.Errorf("One element should not be zero")
	}

	tests := []struct {
		name     string
		a, b     int64
		expected int64
		op       string
	}{
		{"add_basic", 25, 30, 55, "add"},
		{"add_with_reduction", 80, 50, 29, "add"}, // (80 + 50) % 101 = 29
		{"sub_basic", 50, 30, 20, "sub"},
		{"sub_with_reduction", 20, 30, 91, "sub"}, // (20 - 30) % 101 = 91
		{"mul_basic", 7, 9, 63, "mul"},
		{"mul_with_reduction", 15, 12, 79, "mul"}, // (15 * 12) % 101 = 79
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := field.FromBytes(big.NewInt(tt.a).Bytes())
			b := field.FromBytes(big.NewInt(tt.b).Bytes())
			expected := field.FromBytes(big.NewInt(tt.expected).Bytes())

			var result Element
			switch tt.op {
			case "add":
				result = a.Add(b)
			case "sub":
				result = a.Sub(b)
			case "mul":
				result = a.Mul(b)
			default:
				t.Fatalf("unknown operation: %s", tt.op)
			}

			if !result.Equal(expected) {
				t.Errorf("%s operation failed: %d %s %d = expected %d, got %s",
					tt.op, tt.a, tt.op, tt.b, tt.expected, result)
			}
		})
	}
}

// TestPrimeFieldInversion tests multiplicative inverse
func TestPrimeFieldInversion(t *testing.T) {
	field := setupPrimeField()
	one := field.One()

	testCases := []int64{1, 2, 3, 5, 7, 11, 25, 50, 100}

	for _, val := range testCases {
		t.Run("", func(t *testing.T) {
			a := field.FromBytes(big.NewInt(val).Bytes())
			inv := a.Inv()

			// Check that a * a^(-1) = 1
			result := a.Mul(inv)
			if !result.Equal(one) {
				t.Errorf("Inversion failed for %d: a * a^(-1) = %s, expected %s", val, result, one)
			}
		})
	}
}

// TestPrimeFieldZeroInversion tests that zero inversion panics
func TestPrimeFieldZeroInversion(t *testing.T) {
	field := setupPrimeField()
	zero := field.Zero()

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Zero inversion should panic")
		}
	}()

	zero.Inv()
}

// TestPrimeFieldElementMethods tests various element methods
func TestPrimeFieldElementMethods(t *testing.T) {
	testCases := []struct {
		name     string
		prime    *big.Int
		expected int
	}{
		{"small_prime", big.NewInt(101), 6},            // 101 has 7 bits, so 6 usable
		{"medium_prime", big.NewInt(65537), 16},        // 65537 has 17 bits, so 16 usable
		{"large_prime", big.NewInt(4_294_967_311), 32}, // Large prime has 33 bits, so 32 usable
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			field := NewPrimeField(tc.prime)

			// Test BitsPerDataElement
			if field.BitsPerDataElement() != tc.expected {
				t.Errorf("BitsPerDataElement failed: expected %d, got %d",
					tc.expected, field.BitsPerDataElement())
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

			// Test string representation for small fields
			if tc.name == "small_prime" {
				a := field.FromBytes(big.NewInt(42).Bytes())
				if a.String() != "42" {
					t.Errorf("String representation failed: expected '42', got '%s'", a.String())
				}
			}
		})
	}
}

// TestPrimeFieldFromBitsAndBits tests FromBits() and Bits() methods
func TestPrimeFieldFromBitsAndBits(t *testing.T) {
	field := setupLargePrimeField() // Use large prime to avoid reduction

	testCases := []struct {
		data   []byte
		bitLen int
		desc   string
	}{
		{[]byte{0x55}, 8, "simple_8_bit"},          // 01010101 = 85
		{[]byte{0xA0}, 4, "partial_byte"},          // 1010____
		{[]byte{0x12, 0x30}, 12, "multi_byte"},     // 12 bits spanning bytes
		{[]byte{0x80}, 1, "single_bit"},            // just first bit
		{[]byte{0x7F}, 7, "seven_bits"},            // 0111111_
		{[]byte{0x12, 0x34}, 16, "full_two_bytes"}, // full 2 bytes
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

	// Test modular reduction case with small prime
	t.Run("modular_reduction", func(t *testing.T) {
		smallField := setupPrimeField()
		// Create a value larger than the prime (255 > 101)
		elem := smallField.FromBits([]byte{0xFF}, 8) // 255
		recovered := elem.Bits(8)
		expected := byte(53) // 255 % 101 = 53
		if len(recovered) != 1 || recovered[0] != expected {
			t.Errorf("Modular reduction failed: expected [%d], got %x", expected, recovered)
		}
	})
}

// TestPrimeFieldRandom tests random element generation
func TestPrimeFieldRandom(t *testing.T) {
	field := setupPrimeField()
	p := big.NewInt(101)

	for i := 0; i < 10; i++ {
		elem, err := field.Random()
		if err != nil {
			t.Fatalf("Random element generation failed: %v", err)
		}

		// Check that element is in field bounds [0, p-1]
		bytes := elem.Bytes()
		if len(bytes) == 0 {
			// Zero element is valid
			continue
		}
		val := new(big.Int).SetBytes(bytes)
		if val.Cmp(p) >= 0 {
			t.Errorf("Random element out of bounds: %s >= %s", val, p)
		}

		// Check that operations work
		result := elem.Add(elem)
		_ = result // Should not panic
	}
}

// TestPrimeFieldRandomMax tests random element generation with max bits
func TestPrimeFieldRandomMax(t *testing.T) {
	field := setupPrimeField()
	p := big.NewInt(101)
	maxBits := 4

	for i := 0; i < 10; i++ {
		elem, err := field.RandomMax(maxBits)
		if err != nil {
			t.Fatalf("RandomMax failed: %v", err)
		}

		// Check that element is reduced modulo p
		bytes := elem.Bytes()
		if len(bytes) == 0 {
			// Zero element is valid
			continue
		}
		val := new(big.Int).SetBytes(bytes)
		if val.Cmp(p) >= 0 {
			t.Errorf("RandomMax element not reduced: %s >= %s", val, p)
		}
	}
}

// TestPrimeFieldBytes tests byte representation round-trip
func TestPrimeFieldBytes(t *testing.T) {
	field := setupPrimeField()

	testCases := []int64{0, 1, 42, 100}

	for _, val := range testCases {
		t.Run("", func(t *testing.T) {
			elem := field.FromBytes(big.NewInt(val).Bytes())
			bytes := elem.Bytes()

			recovered := new(big.Int).SetBytes(bytes)
			if recovered.Int64() != val {
				t.Errorf("Bytes round-trip failed for %d: expected %d, got %d", val, val, recovered.Int64())
			}
		})
	}
}

// TestPrimeFieldLargePrime tests operations with large primes
func TestPrimeFieldLargePrime(t *testing.T) {
	field := setupLargePrimeField()
	largePrime := big.NewInt(4_294_967_311)

	// Test basic operations with large values
	a := field.FromBytes(big.NewInt(1000000).Bytes())
	b := field.FromBytes(big.NewInt(2000000).Bytes())

	// Addition
	result := a.Add(b)
	expected := field.FromBytes(big.NewInt(3000000).Bytes())
	if !result.Equal(expected) {
		t.Errorf("Large prime addition failed: expected %s, got %s", expected, result)
	}

	// Multiplication
	result = a.Mul(b)
	expectedVal := new(big.Int).Mul(big.NewInt(1000000), big.NewInt(2000000))
	expectedVal.Mod(expectedVal, largePrime)
	expected = field.FromBytes(expectedVal.Bytes())
	if !result.Equal(expected) {
		t.Errorf("Large prime multiplication failed: expected %s, got %s", expected, result)
	}

	// Test wrap-around at prime boundary
	largeVal := new(big.Int).Sub(largePrime, big.NewInt(1))
	elem := field.FromBytes(largeVal.Bytes())
	result = elem.Add(field.One())
	if !result.Equal(field.Zero()) {
		t.Errorf("Large value addition should wrap to zero, got %s", result)
	}
}
