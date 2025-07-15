package field

import (
	"math/big"
	"math/rand"
	"testing"
	"time"
)

var p = big.NewInt(101) // small prime field for test consistency

// Test helper functions

// identityMatrix creates an n×n identity matrix over the given field
func identityMatrix(n int, field Field) [][]Element {
	I := make([][]Element, n)
	for i := 0; i < n; i++ {
		I[i] = make([]Element, n)
		for j := 0; j < n; j++ {
			if i == j {
				I[i][j] = field.One()
			} else {
				I[i][j] = field.Zero()
			}
		}
	}
	return I
}

// matrixMultiply computes A × B matrix multiplication over the field
func matrixMultiply(A, B [][]Element, field Field) [][]Element {
	n, m := len(A), len(B[0])
	k := len(B)
	C := make([][]Element, n)
	for i := range C {
		C[i] = make([]Element, m)
		for j := 0; j < m; j++ {
			sum := field.Zero()
			for l := 0; l < k; l++ {
				tmp := A[i][l].Mul(B[l][j])
				sum = sum.Add(tmp)
			}
			C[i][j] = sum
		}
	}
	return C
}

// matricesEqual checks if two matrices are element-wise equal
func matricesEqual(A, B [][]Element) bool {
	for i := range A {
		for j := range A[i] {
			if !A[i][j].Equal(B[i][j]) {
				return false
			}
		}
	}
	return true
}

// generateRandomMatrix creates an n×m matrix with random field elements
func generateRandomMatrix(n, m int, field Field) [][]Element {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	M := make([][]Element, n)
	for i := range M {
		M[i] = make([]Element, m)
		for j := range M[i] {
			val := new(big.Int).Rand(rng, p)
			M[i][j] = field.FromBytes(val.Bytes())
		}
	}
	return M
}

// TestInvertMatrixIdentity tests that the inverse of identity matrix is itself
func TestInvertMatrixIdentity(t *testing.T) {
	field := NewPrimeField(p)
	n := 4
	I := identityMatrix(n, field)

	Ainv, err := InvertMatrix(I, field)
	if err != nil {
		t.Fatalf("Failed to invert identity matrix: %v", err)
	}
	if !matricesEqual(Ainv, I) {
		t.Errorf("Inverse of identity matrix should be identity")
	}
}

// TestInvertMatrixSmall tests matrix inversion for small 1x1 and 2x2 matrices
func TestInvertMatrixSmall(t *testing.T) {
	field := NewPrimeField(p)

	// 1x1 invertible
	A := [][]Element{{field.FromBytes(big.NewInt(3).Bytes())}}
	Ainv, err := InvertMatrix(A, field)
	if err != nil {
		t.Errorf("1x1 matrix invert failed: %v", err)
	}

	// Check A * Ainv == I
	result := A[0][0].Mul(Ainv[0][0])
	if !result.Equal(field.One()) {
		t.Errorf("1x1 invert failed: A * Ainv != 1")
	}

	// 2x2 invertible
	A = [][]Element{
		{field.FromBytes(big.NewInt(4).Bytes()), field.FromBytes(big.NewInt(7).Bytes())},
		{field.FromBytes(big.NewInt(2).Bytes()), field.FromBytes(big.NewInt(6).Bytes())},
	}
	Ainv, err = InvertMatrix(A, field)
	if err != nil {
		t.Fatalf("2x2 matrix invert failed: %v", err)
	}
	// Check A * Ainv == I
	I := matrixMultiply(A, Ainv, field)
	if !matricesEqual(I, identityMatrix(2, field)) {
		t.Errorf("2x2 invert failed: A * Ainv != I")
	}
}

// TestInvertMatrixSingular tests that singular matrices return an error when inverted
func TestInvertMatrixSingular(t *testing.T) {
	field := NewPrimeField(p)

	// Singular matrix (rows are linearly dependent)
	A := [][]Element{
		{field.FromBytes(big.NewInt(1).Bytes()), field.FromBytes(big.NewInt(2).Bytes())},
		{field.FromBytes(big.NewInt(2).Bytes()), field.FromBytes(big.NewInt(4).Bytes())},
	}
	_, err := InvertMatrix(A, field)
	if err == nil {
		t.Errorf("Expected error for singular matrix")
	}
}

// TestRecoverVectorsIdentity tests vector recovery when encoding matrix is identity
func TestRecoverVectorsIdentity(t *testing.T) {
	field := NewPrimeField(p)

	// A is identity, R should equal V
	n := 3
	V := [][]Element{
		{field.FromBytes(big.NewInt(1).Bytes()), field.FromBytes(big.NewInt(2).Bytes())},
		{field.FromBytes(big.NewInt(3).Bytes()), field.FromBytes(big.NewInt(4).Bytes())},
		{field.FromBytes(big.NewInt(5).Bytes()), field.FromBytes(big.NewInt(6).Bytes())},
	}
	A := identityMatrix(n, field)
	R := matrixMultiply(A, V, field)

	Vrec, err := RecoverVectors(A, R, field)
	if err != nil {
		t.Fatalf("RecoverVectors failed: %v", err)
	}
	if !matricesEqual(V, Vrec) {
		t.Errorf("Expected recovered = original, got %v", Vrec)
	}
}

// TestRecoverVectorsKnown tests vector recovery with known encoding matrix and vectors
func TestRecoverVectorsKnown(t *testing.T) {
	field := NewPrimeField(p)

	// A: 2x2, V: 2x2
	A := [][]Element{
		{field.FromBytes(big.NewInt(2).Bytes()), field.FromBytes(big.NewInt(3).Bytes())},
		{field.FromBytes(big.NewInt(1).Bytes()), field.FromBytes(big.NewInt(4).Bytes())},
	}
	V := [][]Element{
		{field.FromBytes(big.NewInt(5).Bytes()), field.FromBytes(big.NewInt(6).Bytes())},
		{field.FromBytes(big.NewInt(7).Bytes()), field.FromBytes(big.NewInt(8).Bytes())},
	}
	R := matrixMultiply(A, V, field)
	Vrec, err := RecoverVectors(A, R, field)
	if err != nil {
		t.Fatalf("RecoverVectors failed: %v", err)
	}
	if !matricesEqual(V, Vrec) {
		t.Errorf("Expected recovered = original, got %v", Vrec)
	}
}

// TestRecoverVectorsSingular tests that vector recovery fails with singular matrices
func TestRecoverVectorsSingular(t *testing.T) {
	field := NewPrimeField(p)

	// A is singular (non-invertible), should fail
	A := [][]Element{
		{field.FromBytes(big.NewInt(1).Bytes()), field.FromBytes(big.NewInt(2).Bytes())},
		{field.FromBytes(big.NewInt(2).Bytes()), field.FromBytes(big.NewInt(4).Bytes())},
	}
	V := [][]Element{
		{field.FromBytes(big.NewInt(3).Bytes())},
		{field.FromBytes(big.NewInt(5).Bytes())},
	}
	R := matrixMultiply(A, V, field)
	_, err := RecoverVectors(A, R, field)
	if err == nil {
		t.Errorf("Expected error for singular matrix in RecoverVectors")
	}
}

// TestRecoverVectorsRandom tests vector recovery with randomly generated invertible matrices
func TestRecoverVectorsRandom(t *testing.T) {
	field := NewPrimeField(p)
	n, m := 5, 3
	V := generateRandomMatrix(n, m, field)

	var A [][]Element
	var err error
	for {
		A = generateRandomMatrix(n, n, field)
		_, err = InvertMatrix(A, field)
		if err == nil {
			break
		}
	}
	R := matrixMultiply(A, V, field)

	Vrec, err := RecoverVectors(A, R, field)
	if err != nil {
		t.Fatalf("RecoverVectors failed: %v", err)
	}
	if !matricesEqual(V, Vrec) {
		t.Errorf("Recovered matrix did not match original")
	}
}

// TestIsLinearlyIndependent_EmptySet tests that empty vector sets are considered independent
func TestIsLinearlyIndependent_EmptySet(t *testing.T) {
	field := NewPrimeField(p)
	var vectors [][]Element
	if !IsLinearlyIndependent(vectors, field) {
		t.Errorf("Empty set should be independent")
	}
}

// TestIsLinearlyIndependent_SingleZeroVector tests that zero vector is linearly dependent
func TestIsLinearlyIndependent_SingleZeroVector(t *testing.T) {
	field := NewPrimeField(p)
	vectors := [][]Element{
		{field.Zero(), field.Zero(), field.Zero()},
	}
	if IsLinearlyIndependent(vectors, field) {
		t.Errorf("Zero vector is not independent")
	}
}

// TestIsLinearlyIndependent_SingleNonZeroVector tests that single non-zero vector is independent
func TestIsLinearlyIndependent_SingleNonZeroVector(t *testing.T) {
	field := NewPrimeField(p)
	vectors := [][]Element{
		{
			field.FromBytes(big.NewInt(3).Bytes()),
			field.FromBytes(big.NewInt(1).Bytes()),
			field.FromBytes(big.NewInt(2).Bytes()),
		},
	}
	if !IsLinearlyIndependent(vectors, field) {
		t.Errorf("Single non-zero vector should be independent")
	}
}

// TestIsLinearlyIndependent_DuplicateVectors tests that duplicate vectors are dependent
func TestIsLinearlyIndependent_DuplicateVectors(t *testing.T) {
	field := NewPrimeField(p)
	vectors := [][]Element{
		{field.FromBytes(big.NewInt(1).Bytes()), field.FromBytes(big.NewInt(2).Bytes())},
		{field.FromBytes(big.NewInt(1).Bytes()), field.FromBytes(big.NewInt(2).Bytes())},
	}
	if IsLinearlyIndependent(vectors, field) {
		t.Errorf("Duplicate vectors should be dependent")
	}
}

// TestIsLinearlyIndependent_OrthogonalVectors tests that orthogonal basis vectors are independent
func TestIsLinearlyIndependent_OrthogonalVectors(t *testing.T) {
	field := NewPrimeField(p)
	vectors := [][]Element{
		{field.One(), field.Zero(), field.Zero()},
		{field.Zero(), field.One(), field.Zero()},
		{field.Zero(), field.Zero(), field.One()},
	}
	if !IsLinearlyIndependent(vectors, field) {
		t.Errorf("Orthogonal basis should be independent")
	}
}

// TestIsLinearlyIndependent_ThreeWithDependency tests detection of linear dependency in three vectors
func TestIsLinearlyIndependent_ThreeWithDependency(t *testing.T) {
	field := NewPrimeField(p)
	v1 := []Element{field.FromBytes(big.NewInt(1).Bytes()), field.FromBytes(big.NewInt(2).Bytes())}
	v2 := []Element{field.FromBytes(big.NewInt(2).Bytes()), field.FromBytes(big.NewInt(3).Bytes())}
	v3 := []Element{v1[0].Add(v2[0]), v1[1].Add(v2[1])}

	vectors := [][]Element{v1, v2, v3}
	if IsLinearlyIndependent(vectors, field) {
		t.Errorf("Should be dependent: v3 = v1 + v2")
	}
}

// TestIsLinearlyIndependent_RandomFullRank tests that full-rank matrix vectors are independent
func TestIsLinearlyIndependent_RandomFullRank(t *testing.T) {
	field := NewPrimeField(p)
	V := [][]Element{
		{
			field.FromBytes(big.NewInt(1).Bytes()),
			field.FromBytes(big.NewInt(2).Bytes()),
			field.FromBytes(big.NewInt(3).Bytes()),
		},
		{
			field.Zero(),
			field.FromBytes(big.NewInt(1).Bytes()),
			field.FromBytes(big.NewInt(4).Bytes()),
		},
		{
			field.Zero(),
			field.Zero(),
			field.FromBytes(big.NewInt(1).Bytes()),
		},
	}
	if !IsLinearlyIndependent(V, field) {
		t.Errorf("Upper triangular full-rank matrix should be independent")
	}
}

// TestIsLinearlyIndependent_RandomWithDependency tests detection of dependency in constructed vectors
func TestIsLinearlyIndependent_RandomWithDependency(t *testing.T) {
	field := NewPrimeField(p)
	V := [][]Element{
		{field.One(), field.Zero(), field.Zero()},
		{field.Zero(), field.One(), field.Zero()},
		{field.One(), field.One(), field.Zero()}, // v3 = v1 + v2
	}
	if IsLinearlyIndependent(V, field) {
		t.Errorf("Should be dependent: third vector is sum of first two")
	}
}
