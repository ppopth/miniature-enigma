package field

import (
	"math/big"
	"testing"
)

// TestSplitBitsToFieldElementsAndBack tests round-trip conversion of bytes to field elements
func TestSplitBitsToFieldElementsAndBack(t *testing.T) {
	field := NewPrimeField(big.NewInt(4_294_967_311))

	// Test valid cases with different bit lengths
	testCases := []struct {
		data []byte
		k    int
	}{
		{[]byte{0x12, 0x34, 0x56, 0x78}, 16}, // 32 bits, k=16 -> 2 elements
		{[]byte{0xAB, 0xCD}, 8},              // 16 bits, k=8 -> 2 elements
		{[]byte{0xFF}, 4},                    // 8 bits, k=4 -> 2 elements
		{[]byte{0x12, 0x34, 0x56}, 12},       // 24 bits, k=12 -> 2 elements
	}

	for _, tc := range testCases {
		// Split bytes into field elements
		elements := SplitBitsToFieldElements(tc.data, tc.k, field)

		// Check element count
		expectedElements := (len(tc.data) * 8) / tc.k
		if len(elements) != expectedElements {
			t.Errorf("Expected %d elements, got %d for data %x, k=%d",
				expectedElements, len(elements), tc.data, tc.k)
		}

		// Convert back to bytes
		recovered := FieldElementsToBytes(elements, tc.k)

		// Check length consistency
		if len(recovered) != len(tc.data) {
			t.Errorf("Recovered data length mismatch: expected %d, got %d",
				len(tc.data), len(recovered))
		}

		// Check byte-by-byte equality
		for i := range tc.data {
			if recovered[i] != tc.data[i] {
				t.Errorf("Data mismatch at byte %d: expected %02x, got %02x",
					i, tc.data[i], recovered[i])
				break
			}
		}
	}
}

// TestSplitBitsDiscardingRemaining tests that remaining bits are discarded
func TestSplitBitsDiscardingRemaining(t *testing.T) {
	field := NewPrimeField(big.NewInt(101))

	// Test case: totalBits not divisible by k
	data := []byte{0x12, 0x34, 0x56} // 24 bits
	k := 7                           // 24 % 7 != 0, so 3 bits will be discarded

	elements := SplitBitsToFieldElements(data, k, field)

	// Should have 3 elements (24 / 7 = 3 remainder 3)
	if len(elements) != 3 {
		t.Errorf("Expected 3 elements, got %d", len(elements))
	}
}

// TestFieldElementsToBytesRoundingUp tests that output is rounded up when there's a remainder
func TestFieldElementsToBytesRoundingUp(t *testing.T) {
	field := NewPrimeField(big.NewInt(101))

	// Create elements
	elements := []Element{
		field.FromBytes(big.NewInt(1).Bytes()),
		field.FromBytes(big.NewInt(2).Bytes()),
	}

	// Test case: totalBits not divisible by 8
	k := 7 // 2 elements * 7 bits = 14 bits, 14 % 8 != 0

	result := FieldElementsToBytes(elements, k)

	// Should have 2 bytes ((14 + 7) / 8 = 2)
	if len(result) != 2 {
		t.Errorf("Expected 2 bytes, got %d", len(result))
	}
}

// TestSplitBitsEdgeCases tests boundary conditions and edge cases
func TestSplitBitsEdgeCases(t *testing.T) {
	field := NewPrimeField(big.NewInt(4_294_967_311))

	// Test empty data
	elements := SplitBitsToFieldElements([]byte{}, 8, field)
	if len(elements) != 0 {
		t.Errorf("Empty data should produce no elements")
	}

	// Convert empty elements back
	recovered := FieldElementsToBytes([]Element{}, 8)
	if len(recovered) != 0 {
		t.Errorf("Empty elements should produce no bytes")
	}

	// Test single byte
	data := []byte{0x80} // 10000000
	elements = SplitBitsToFieldElements(data, 8, field)
	if len(elements) != 1 {
		t.Errorf("Single byte should produce one element")
	}

	// Test single byte round-trip
	recovered = FieldElementsToBytes(elements, 8)
	if len(recovered) != 1 || recovered[0] != 0x80 {
		t.Errorf("Expected [0x80], got %x", recovered)
	}
}

// TestBitDiscardingBehavior tests the discarding behavior comprehensively
func TestBitDiscardingBehavior(t *testing.T) {
	field := NewPrimeField(big.NewInt(4_294_967_311))

	testCases := []struct {
		name     string
		data     []byte
		k        int
		elements int
		outBytes int
	}{
		{
			name:     "24 bits, k=10, discards 4 bits",
			data:     []byte{0xFF, 0xFF, 0xFF}, // 24 bits
			k:        10,
			elements: 2, // 24/10 = 2 remainder 4
			outBytes: 3, // (2*10+7)/8 = 3
		},
		{
			name:     "16 bits, k=5, discards 1 bit",
			data:     []byte{0xAB, 0xCD}, // 16 bits
			k:        5,
			elements: 3, // 16/5 = 3 remainder 1
			outBytes: 2, // (3*5+7)/8 = 2
		},
		{
			name:     "8 bits, k=3, discards 2 bits",
			data:     []byte{0xFF}, // 8 bits
			k:        3,
			elements: 2, // 8/3 = 2 remainder 2
			outBytes: 1, // (2*3+7)/8 = 1
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Split into elements
			elements := SplitBitsToFieldElements(tc.data, tc.k, field)

			if len(elements) != tc.elements {
				t.Errorf("Expected %d elements, got %d", tc.elements, len(elements))
			}

			// Convert back
			result := FieldElementsToBytes(elements, tc.k)

			if len(result) != tc.outBytes {
				t.Errorf("Expected %d output bytes, got %d", tc.outBytes, len(result))
			}
		})
	}
}
