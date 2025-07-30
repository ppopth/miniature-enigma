package group

import (
	"math/big"
	"testing"
)

// TestFieldElementToScalarValuePreservation verifies that the numerical value
// is preserved when converting from field element to scalar.
// For example, 3 as a field element should become 3 as a scalar.
func TestFieldElementToScalarValuePreservation(t *testing.T) {
	// Create Ristretto255 group and its scalar field
	group := NewRistretto255Group()
	field := NewScalarField(group)

	// Test that field element n converts to scalar that represents n
	testCases := []struct {
		name  string
		value int64
	}{
		{"zero", 0},
		{"one", 1},
		{"two", 2},
		{"three", 3},
		{"five", 5},
		{"ten", 10},
		{"hundred", 100},
	}

	// Use the group's base element (generator)
	G := group.GeneratePoint([]byte("base_generator"))

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create field element from the value
			fieldElem := field.FromBytes(big.NewInt(tc.value).Bytes())

			// Convert to scalar
			scalar := group.ScalarFromFieldElement(fieldElem)

			// Compute scalar * G
			scalarG := G.ScalarMult(scalar)

			// Compute n * G by repeated addition to verify the scalar represents n
			expectedG := group.Zero()
			for i := int64(0); i < tc.value; i++ {
				expectedG = expectedG.Add(G)
			}

			// They should be equal, proving that the scalar represents the value n
			if scalarG.Equal(expectedG) != 1 {
				t.Errorf("scalar(%d) * G != %d * G: field element %d did not convert to scalar %d",
					tc.value, tc.value, tc.value, tc.value)
			}
		})
	}
}

// TestFieldElementToScalarArithmetic verifies that arithmetic operations
// work correctly with the group operations
func TestFieldElementToScalarArithmetic(t *testing.T) {
	// Create Ristretto255 group and its scalar field
	group := NewRistretto255Group()
	field := NewScalarField(group)

	// Test that scalar operations work correctly with group operations
	G := group.GeneratePoint([]byte("base_generator"))

	// Test 3 * G
	three := field.FromBytes(big.NewInt(3).Bytes())
	threeScalar := group.ScalarFromFieldElement(three)
	threeG := G.ScalarMult(threeScalar)

	// Test that 3 * G = G + G + G
	expectedThreeG := group.Zero()
	for i := 0; i < 3; i++ {
		expectedThreeG = expectedThreeG.Add(G)
	}

	if threeG.Equal(expectedThreeG) != 1 {
		t.Errorf("3 * G != G + G + G: scalar multiplication does not work correctly")
	}

	// Test zero scalar
	zero := field.Zero()
	zeroScalar := group.ScalarFromFieldElement(zero)
	zeroG := G.ScalarMult(zeroScalar)

	if zeroG.Equal(group.Zero()) != 1 {
		t.Errorf("0 * G != 0: zero scalar multiplication does not work correctly")
	}
}

// TestRistretto255GroupBasicOperations tests basic group operations
func TestRistretto255GroupBasicOperations(t *testing.T) {
	group := NewRistretto255Group()

	// Test zero element
	zero := group.Zero()
	if zero == nil {
		t.Fatal("Zero element should not be nil")
	}

	// Test generator creation
	G := group.GeneratePoint([]byte("test_generator"))
	if G == nil {
		t.Fatal("Generator should not be nil")
	}

	// Test that G != 0
	if G.Equal(zero) == 1 {
		t.Error("Generator should not equal zero")
	}

	// Test addition: G + 0 = G
	sum := G.Add(zero)
	if sum.Equal(G) != 1 {
		t.Error("G + 0 should equal G")
	}

	// Test element encoding/decoding
	encoded := G.Encode()
	if len(encoded) != group.ElementSize() {
		t.Errorf("Encoded element size should be %d, got %d", group.ElementSize(), len(encoded))
	}

	decoded := group.NewElement()
	err := decoded.Decode(encoded)
	if err != nil {
		t.Fatalf("Failed to decode element: %v", err)
	}

	if decoded.Equal(G) != 1 {
		t.Error("Decoded element should equal original")
	}
}

// TestRistretto255DeterministicGenerators tests that generators are deterministic
func TestRistretto255DeterministicGenerators(t *testing.T) {
	group := NewRistretto255Group()

	// Generate the same point multiple times
	seed := []byte("deterministic_seed")
	G1 := group.GeneratePoint(seed)
	G2 := group.GeneratePoint(seed)

	if G1.Equal(G2) != 1 {
		t.Error("Same seed should produce same generator point")
	}

	// Different seeds should produce different points
	G3 := group.GeneratePoint([]byte("different_seed"))
	if G1.Equal(G3) == 1 {
		t.Error("Different seeds should produce different generator points")
	}
}

// TestRistretto255ScalarField tests that we can create a scalar field from the group
func TestRistretto255ScalarField(t *testing.T) {
	group := NewRistretto255Group()
	field := NewScalarField(group)

	if field == nil {
		t.Fatal("NewScalarField should not return nil")
	}

	// Test basic field operations
	zero := field.Zero()
	one := field.One()

	if zero == nil || one == nil {
		t.Fatal("Field elements should not be nil")
	}

	if zero.Equal(one) {
		t.Error("Zero should not equal one")
	}

	// Test field arithmetic
	sum := zero.Add(one)
	if !sum.Equal(one) {
		t.Error("0 + 1 should equal 1")
	}
}

func BenchmarkRistretto255Group(b *testing.B) {
	group := NewRistretto255Group()
	elem1 := group.NewElement()
	elem2 := group.NewElement()

	// Generate some test data
	data1 := make([]byte, group.ElementSize())
	data2 := make([]byte, group.ElementSize())
	for i := range data1 {
		data1[i] = byte(i)
		data2[i] = byte(i + 1)
	}

	elem1.Decode(data1)
	elem2.Decode(data2)

	scalar1 := group.NewScalar()
	scalar2 := group.NewScalar()
	scalarData1 := make([]byte, 32) // Ristretto255 scalars are 32 bytes
	scalarData2 := make([]byte, 32)
	for i := range scalarData1 {
		scalarData1[i] = byte(i % 256)
		scalarData2[i] = byte((i + 1) % 256)
	}
	scalar1.Decode(scalarData1)
	scalar2.Decode(scalarData2)

	b.Run("Add", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = elem1.Add(elem2)
		}
	})

	b.Run("ScalarMult", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = elem1.ScalarMult(scalar1)
		}
	})

	b.Run("Encode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = elem1.Encode()
		}
	})

	b.Run("Decode", func(b *testing.B) {
		data := elem1.Encode()
		newElem := group.NewElement()
		for i := 0; i < b.N; i++ {
			_ = newElem.Decode(data)
		}
	})
}
