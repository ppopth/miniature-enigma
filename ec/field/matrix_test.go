package field

import (
	"fmt"
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

// matricesEqual checks if two matrices are element-wise equal
func matricesEqual(A, B [][]Element) bool {
	if len(A) != len(B) {
		return false
	}
	for i := range A {
		if len(A[i]) != len(B[i]) {
			return false
		}
		for j := range A[i] {
			if !A[i][j].Equal(B[i][j]) {
				return false
			}
		}
	}
	return true
}

func TestMatrixMultiply(t *testing.T) {
	field := NewPrimeField(big.NewInt(17)) // Small prime for easy verification

	// Test case 1: Identity matrix multiplication
	A := [][]Element{
		{field.FromBytes([]byte{1}), field.FromBytes([]byte{0})},
		{field.FromBytes([]byte{0}), field.FromBytes([]byte{1})},
	}
	B := [][]Element{
		{field.FromBytes([]byte{3}), field.FromBytes([]byte{4})},
		{field.FromBytes([]byte{5}), field.FromBytes([]byte{6})},
	}
	result := MatrixMultiply(A, B, field)
	if !matricesEqual(result, B) {
		t.Errorf("Identity matrix multiplication failed")
	}

	// Test case 2: Known multiplication
	// A = [[2, 3], [1, 4]], B = [[5, 6], [7, 8]]
	// Expected: [[2*5+3*7, 2*6+3*8], [1*5+4*7, 1*6+4*8]] = [[31, 36], [33, 38]]
	// In GF(17): [[31%17, 36%17], [33%17, 38%17]] = [[14, 2], [16, 4]]
	A2 := [][]Element{
		{field.FromBytes([]byte{2}), field.FromBytes([]byte{3})},
		{field.FromBytes([]byte{1}), field.FromBytes([]byte{4})},
	}
	B2 := [][]Element{
		{field.FromBytes([]byte{5}), field.FromBytes([]byte{6})},
		{field.FromBytes([]byte{7}), field.FromBytes([]byte{8})},
	}
	expected := [][]Element{
		{field.FromBytes([]byte{14}), field.FromBytes([]byte{2})},
		{field.FromBytes([]byte{16}), field.FromBytes([]byte{4})},
	}
	result2 := MatrixMultiply(A2, B2, field)
	if !matricesEqual(result2, expected) {
		t.Errorf("Known matrix multiplication failed. Expected %v, got %v", expected, result2)
	}

	// Test case 3: Non-square matrices
	// A is 2x3, B is 3x2, result should be 2x2
	A3 := [][]Element{
		{field.FromBytes([]byte{1}), field.FromBytes([]byte{2}), field.FromBytes([]byte{3})},
		{field.FromBytes([]byte{4}), field.FromBytes([]byte{5}), field.FromBytes([]byte{6})},
	}
	B3 := [][]Element{
		{field.FromBytes([]byte{7}), field.FromBytes([]byte{8})},
		{field.FromBytes([]byte{9}), field.FromBytes([]byte{10})},
		{field.FromBytes([]byte{11}), field.FromBytes([]byte{12})},
	}
	result3 := MatrixMultiply(A3, B3, field)
	if len(result3) != 2 || len(result3[0]) != 2 {
		t.Errorf("Expected 2x2 result matrix, got %dx%d", len(result3), len(result3[0]))
	}

	// Test case 4: Empty matrices
	emptyResult := MatrixMultiply([][]Element{}, [][]Element{}, field)
	if emptyResult != nil {
		t.Errorf("Expected nil for empty matrices, got %v", emptyResult)
	}
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
	I := MatrixMultiply(A, Ainv, field)
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

// TestInvertMatrixZeroDiagonal tests matrices with zero diagonal elements
func TestInvertMatrixZeroDiagonal(t *testing.T) {
	field := NewPrimeField(p)

	// Test case 1: Simple 2x2 matrix with zero at [0,0]
	A1 := [][]Element{
		{field.Zero(), field.One()},
		{field.FromBytes(big.NewInt(3).Bytes()), field.FromBytes(big.NewInt(2).Bytes())},
	}

	Ainv1, err := InvertMatrix(A1, field)
	if err != nil {
		t.Fatalf("Failed to invert matrix with zero diagonal at [0,0]: %v", err)
	}

	// Verify the inverse is correct
	result := MatrixMultiply(A1, Ainv1, field)
	if !matricesEqual(result, identityMatrix(2, field)) {
		t.Errorf("A * A^(-1) != I for matrix with zero diagonal")
	}

	// Test case 2: 3x3 permutation matrix
	A2 := [][]Element{
		{field.Zero(), field.One(), field.Zero()},
		{field.Zero(), field.Zero(), field.One()},
		{field.One(), field.Zero(), field.Zero()},
	}

	Ainv2, err := InvertMatrix(A2, field)
	if err != nil {
		t.Fatalf("Failed to invert permutation matrix: %v", err)
	}

	// Verify the inverse is correct
	result = MatrixMultiply(A2, Ainv2, field)
	if !matricesEqual(result, identityMatrix(3, field)) {
		t.Errorf("A * A^(-1) != I for permutation matrix")
	}

	// Test case 3: Matrix with all zeros on diagonal
	A3 := [][]Element{
		{field.Zero(), field.FromBytes(big.NewInt(2).Bytes()), field.FromBytes(big.NewInt(3).Bytes())},
		{field.One(), field.Zero(), field.FromBytes(big.NewInt(4).Bytes())},
		{field.FromBytes(big.NewInt(5).Bytes()), field.FromBytes(big.NewInt(6).Bytes()), field.Zero()},
	}

	Ainv3, err := InvertMatrix(A3, field)
	if err != nil {
		t.Fatalf("Failed to invert 3x3 matrix with zero diagonal: %v", err)
	}

	// Verify the inverse is correct
	result = MatrixMultiply(A3, Ainv3, field)
	if !matricesEqual(result, identityMatrix(3, field)) {
		t.Errorf("A * A^(-1) != I for 3x3 matrix with zero diagonal")
	}
}

// TestInvertMatrixRequiresPivoting tests cases where row swapping is needed
func TestInvertMatrixRequiresPivoting(t *testing.T) {
	field := NewPrimeField(p)

	// Test case: Matrix that requires pivoting
	A := [][]Element{
		{field.One(), field.FromBytes(big.NewInt(2).Bytes()), field.FromBytes(big.NewInt(3).Bytes())},
		{field.Zero(), field.Zero(), field.FromBytes(big.NewInt(4).Bytes())},
		{field.Zero(), field.FromBytes(big.NewInt(5).Bytes()), field.FromBytes(big.NewInt(6).Bytes())},
	}

	Ainv, err := InvertMatrix(A, field)
	if err != nil {
		t.Fatalf("Failed to invert matrix that requires pivoting: %v", err)
	}

	// Verify the inverse is correct
	result := MatrixMultiply(A, Ainv, field)
	if !matricesEqual(result, identityMatrix(3, field)) {
		t.Errorf("A * A^(-1) != I for matrix requiring pivoting")
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
	R := MatrixMultiply(A, V, field)

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
	R := MatrixMultiply(A, V, field)
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
	R := MatrixMultiply(A, V, field)
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
	R := MatrixMultiply(A, V, field)

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

// TestIsLinearlyIndependentIncremental_EmptyExisting tests incremental with empty existing set
func TestIsLinearlyIndependentIncremental_EmptyExisting(t *testing.T) {
	field := NewPrimeField(p)
	var existingVectors [][]Element

	// Non-zero vector should be independent
	newVector := []Element{
		field.FromBytes(big.NewInt(3).Bytes()),
		field.FromBytes(big.NewInt(1).Bytes()),
	}
	newREF, isIndependent := IsLinearlyIndependentIncremental(existingVectors, newVector, field)
	if !isIndependent {
		t.Errorf("Non-zero vector should be independent when no existing vectors")
	}
	if isIndependent && newREF == nil {
		t.Errorf("newREF should not be nil for independent vector")
	}
	if isIndependent && newREF != nil && !IsRowEchelonForm(newREF) {
		t.Errorf("Returned matrix should be in REF form")
	}

	// Zero vector should not be independent
	zeroVector := []Element{field.Zero(), field.Zero()}
	newREF, isIndependent = IsLinearlyIndependentIncremental(existingVectors, zeroVector, field)
	if isIndependent {
		t.Errorf("Zero vector should not be independent")
	}
	if newREF != nil {
		t.Errorf("Should return nil matrix for dependent vector")
	}
}

// TestIsLinearlyIndependentIncremental_Independent tests adding independent vector
func TestIsLinearlyIndependentIncremental_Independent(t *testing.T) {
	field := NewPrimeField(p)

	existingVectors := [][]Element{
		{field.One(), field.Zero(), field.Zero()},
		{field.Zero(), field.One(), field.Zero()},
	}

	// Adding a linearly independent vector
	newVector := []Element{field.Zero(), field.Zero(), field.One()}

	newREF, isIndependent := IsLinearlyIndependentIncremental(existingVectors, newVector, field)
	if !isIndependent {
		t.Errorf("Should be independent: orthogonal to existing vectors")
	}
	if isIndependent && newREF == nil {
		t.Errorf("newREF should not be nil for independent vector")
	}
	if isIndependent && newREF != nil && !IsRowEchelonForm(newREF) {
		t.Errorf("Returned matrix should be in REF form")
	}
}

// TestIsLinearlyIndependentIncremental_Dependent tests adding dependent vector
func TestIsLinearlyIndependentIncremental_Dependent(t *testing.T) {
	field := NewPrimeField(p)

	existingVectors := [][]Element{
		{field.One(), field.Zero(), field.Zero()},
		{field.Zero(), field.One(), field.Zero()},
	}

	// Adding a vector that's a linear combination of existing ones
	newVector := []Element{
		field.FromBytes(big.NewInt(3).Bytes()),
		field.FromBytes(big.NewInt(2).Bytes()),
		field.Zero(),
	} // newVector = 3 * v1 + 2 * v2

	newREF, isIndependent := IsLinearlyIndependentIncremental(existingVectors, newVector, field)
	if isIndependent {
		t.Errorf("Should be dependent: linear combination of existing vectors")
	}
	if newREF != nil {
		t.Errorf("Should return nil matrix for dependent vector")
	}
}

// TestIsLinearlyIndependentIncremental_FullRank tests when we already have full rank
func TestIsLinearlyIndependentIncremental_FullRank(t *testing.T) {
	field := NewPrimeField(p)

	// Already have 2 independent vectors in 2D space
	existingVectors := [][]Element{
		{field.One(), field.Zero()},
		{field.Zero(), field.One()},
	}

	// Any new vector in 2D space must be dependent
	newVector := []Element{
		field.FromBytes(big.NewInt(5).Bytes()),
		field.FromBytes(big.NewInt(7).Bytes()),
	}

	newREF, isIndependent := IsLinearlyIndependentIncremental(existingVectors, newVector, field)
	if isIndependent {
		t.Errorf("Should be dependent: already have full rank")
	}
	if newREF != nil {
		t.Errorf("Should return nil matrix for dependent vector")
	}
}

// TestIsLinearlyIndependentIncremental_ConsistentWithOriginal tests that incremental gives same result as original
func TestIsLinearlyIndependentIncremental_ConsistentWithOriginal(t *testing.T) {
	field := NewPrimeField(p)

	// Test several scenarios
	testCases := []struct {
		name     string
		existing [][]Element
		newVec   []Element
	}{
		{
			name: "independent_case",
			existing: [][]Element{
				{field.One(), field.Zero(), field.Zero()},
				{field.Zero(), field.One(), field.Zero()},
			},
			newVec: []Element{field.Zero(), field.Zero(), field.One()},
		},
		{
			name: "dependent_case",
			existing: [][]Element{
				{field.One(), field.Zero()},
				{field.Zero(), field.One()},
			},
			newVec: []Element{field.One(), field.One()},
		},
		{
			name: "single_existing",
			existing: [][]Element{
				{field.One(), field.Zero(), field.Zero()},
			},
			newVec: []Element{field.Zero(), field.One(), field.Zero()},
		},
		{
			name: "zero_at_0_0",
			existing: [][]Element{
				{field.Zero(), field.One(), field.Zero()},
				{field.Zero(), field.Zero(), field.One()},
			},
			newVec: []Element{field.FromBytes(big.NewInt(3).Bytes()), field.FromBytes(big.NewInt(2).Bytes()), field.Zero()},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test cases are already in REF form - test directly
			newREF, incrementalResult := IsLinearlyIndependentIncremental(tc.existing, tc.newVec, field)

			// Test with original version using all vectors
			allVectors := make([][]Element, len(tc.existing)+1)
			copy(allVectors, tc.existing)
			allVectors[len(tc.existing)] = tc.newVec
			originalResult := IsLinearlyIndependent(allVectors, field)

			if incrementalResult != originalResult {
				t.Errorf("REF Incremental (%v) != Original (%v)", incrementalResult, originalResult)
			}

			// Verify that returned matrix is in REF (when independent)
			if incrementalResult && newREF != nil {
				if !IsRowEchelonForm(newREF) {
					t.Errorf("Returned matrix is not in REF form")
				}
			}
		})
	}
}

// TestIsRowEchelonForm tests the REF verification function
func TestIsRowEchelonForm(t *testing.T) {
	field := NewPrimeField(p)

	// Test case 1: Empty matrix (should be REF)
	if !IsRowEchelonForm([][]Element{}) {
		t.Errorf("Empty matrix should be in REF")
	}

	// Test case 2: Single row (non-zero) - should be REF
	singleRow := [][]Element{
		{field.FromBytes(big.NewInt(3).Bytes()), field.FromBytes(big.NewInt(1).Bytes()), field.Zero()},
	}
	if !IsRowEchelonForm(singleRow) {
		t.Errorf("Single non-zero row should be in REF")
	}

	// Test case 3: Single zero row - should be REF
	zeroRow := [][]Element{
		{field.Zero(), field.Zero(), field.Zero()},
	}
	if !IsRowEchelonForm(zeroRow) {
		t.Errorf("Single zero row should be in REF")
	}

	// Test case 4: Identity matrix - should be REF
	identity := [][]Element{
		{field.One(), field.Zero(), field.Zero()},
		{field.Zero(), field.One(), field.Zero()},
		{field.Zero(), field.Zero(), field.One()},
	}
	if !IsRowEchelonForm(identity) {
		t.Errorf("Identity matrix should be in REF")
	}

	// Test case 5: Proper REF matrix
	properREF := [][]Element{
		{field.FromBytes(big.NewInt(2).Bytes()), field.FromBytes(big.NewInt(3).Bytes()), field.FromBytes(big.NewInt(1).Bytes()), field.FromBytes(big.NewInt(4).Bytes())},
		{field.Zero(), field.Zero(), field.FromBytes(big.NewInt(5).Bytes()), field.FromBytes(big.NewInt(2).Bytes())},
		{field.Zero(), field.Zero(), field.Zero(), field.FromBytes(big.NewInt(7).Bytes())},
	}
	if !IsRowEchelonForm(properREF) {
		t.Errorf("Proper REF matrix should be identified as REF")
	}

	// Test case 6: REF with zero rows at bottom
	refWithZeros := [][]Element{
		{field.One(), field.FromBytes(big.NewInt(2).Bytes()), field.FromBytes(big.NewInt(3).Bytes())},
		{field.Zero(), field.Zero(), field.FromBytes(big.NewInt(4).Bytes())},
		{field.Zero(), field.Zero(), field.Zero()},
		{field.Zero(), field.Zero(), field.Zero()},
	}
	if !IsRowEchelonForm(refWithZeros) {
		t.Errorf("REF matrix with zero rows at bottom should be identified as REF")
	}

	// Test case 7: NOT REF - pivot not to the right of previous
	notREF1 := [][]Element{
		{field.Zero(), field.FromBytes(big.NewInt(2).Bytes()), field.FromBytes(big.NewInt(3).Bytes())}, // pivot at col 1
		{field.FromBytes(big.NewInt(1).Bytes()), field.Zero(), field.FromBytes(big.NewInt(4).Bytes())}, // pivot at col 0 - VIOLATION!
	}
	if IsRowEchelonForm(notREF1) {
		t.Errorf("Matrix with pivot not to the right should NOT be REF")
	}

	// Test case 8: NOT REF - equal pivot positions
	notREF2 := [][]Element{
		{field.Zero(), field.FromBytes(big.NewInt(2).Bytes()), field.FromBytes(big.NewInt(3).Bytes())}, // pivot at col 1
		{field.Zero(), field.FromBytes(big.NewInt(5).Bytes()), field.FromBytes(big.NewInt(4).Bytes())}, // pivot at col 1 - VIOLATION!
	}
	if IsRowEchelonForm(notREF2) {
		t.Errorf("Matrix with equal pivot positions should NOT be REF")
	}

	// Test case 9: NOT REF - non-zero below pivot
	notREF3 := [][]Element{
		{field.FromBytes(big.NewInt(1).Bytes()), field.FromBytes(big.NewInt(2).Bytes())}, // pivot at col 0
		{field.FromBytes(big.NewInt(3).Bytes()), field.FromBytes(big.NewInt(4).Bytes())}, // non-zero at col 0 below pivot - VIOLATION!
	}
	if IsRowEchelonForm(notREF3) {
		t.Errorf("Matrix with non-zero below pivot should NOT be REF")
	}

	// Test case 10: NOT REF - zero row above non-zero row
	notREF4 := [][]Element{
		{field.Zero(), field.Zero(), field.Zero()},                                                     // zero row
		{field.Zero(), field.FromBytes(big.NewInt(1).Bytes()), field.FromBytes(big.NewInt(2).Bytes())}, // non-zero row - VIOLATION!
	}
	if IsRowEchelonForm(notREF4) {
		t.Errorf("Matrix with zero row above non-zero row should NOT be REF")
	}

	// Test case 11: Edge case - all zero matrix
	allZeros := [][]Element{
		{field.Zero(), field.Zero(), field.Zero()},
		{field.Zero(), field.Zero(), field.Zero()},
	}
	if !IsRowEchelonForm(allZeros) {
		t.Errorf("All-zero matrix should be in REF")
	}
}

// Benchmarks

func BenchmarkIsLinearlyIndependent(b *testing.B) {
	field := NewPrimeField(big.NewInt(4_294_967_311))

	sizes := []int{4, 8, 16, 32}
	for _, size := range sizes {
		// Create random vectors
		vectors := make([][]Element, size)
		for i := 0; i < size; i++ {
			vectors[i] = make([]Element, size)
			for j := 0; j < size; j++ {
				val := rand.Int63n(1000000)
				vectors[i][j] = field.FromBytes(big.NewInt(val).Bytes())
			}
		}

		b.Run(fmt.Sprintf("vectors=%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = IsLinearlyIndependent(vectors, field)
			}
		})
	}
}

func BenchmarkIsLinearlyIndependentIncremental(b *testing.B) {
	// Use RLNC-typical large prime field
	p := new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil)
	p.Add(p, big.NewInt(297))
	field := NewPrimeField(p)

	sizes := []int{4, 8, 16, 32}
	for _, size := range sizes {
		// Test case 1: Independent case - existing vectors don't span full space
		existingVectorsIndep := make([][]Element, size-1)
		for i := 0; i < size-1; i++ {
			existingVectorsIndep[i] = make([]Element, size)
			for j := 0; j < size; j++ {
				if i == j {
					existingVectorsIndep[i][j] = field.One()
				} else {
					existingVectorsIndep[i][j] = field.Zero()
				}
			}
		}
		// New vector fills the missing dimension (independent)
		newVectorIndep := make([]Element, size)
		for j := 0; j < size; j++ {
			if j == size-1 {
				newVectorIndep[j] = field.One() // Last dimension
			} else {
				elem, _ := field.RandomMax(8)
				newVectorIndep[j] = elem
			}
		}

		b.Run(fmt.Sprintf("incremental_independent_size=%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = IsLinearlyIndependentIncremental(existingVectorsIndep, newVectorIndep, field)
			}
		})

		allVectorsIndep := make([][]Element, size)
		copy(allVectorsIndep, existingVectorsIndep)
		allVectorsIndep[size-1] = newVectorIndep

		b.Run(fmt.Sprintf("original_independent_size=%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = IsLinearlyIndependent(allVectorsIndep, field)
			}
		})

		// Test case 2: Dependent case - existing vectors span full space
		existingVectorsDep := make([][]Element, size)
		for i := 0; i < size; i++ {
			existingVectorsDep[i] = make([]Element, size)
			for j := 0; j < size; j++ {
				if i == j {
					existingVectorsDep[i][j] = field.One()
				} else {
					existingVectorsDep[i][j] = field.Zero()
				}
			}
		}
		// New vector with random elements (will be dependent since we have full rank)
		newVectorDep := make([]Element, size)
		for j := 0; j < size; j++ {
			elem, _ := field.RandomMax(8)
			newVectorDep[j] = elem
		}

		b.Run(fmt.Sprintf("incremental_dependent_size=%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = IsLinearlyIndependentIncremental(existingVectorsDep, newVectorDep, field)
			}
		})

		allVectorsDep := make([][]Element, size+1)
		copy(allVectorsDep, existingVectorsDep)
		allVectorsDep[size] = newVectorDep

		b.Run(fmt.Sprintf("original_dependent_size=%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = IsLinearlyIndependent(allVectorsDep, field)
			}
		})
	}
}

func BenchmarkMatrixMultiply(b *testing.B) {
	field := NewPrimeField(big.NewInt(4_294_967_311))

	sizes := []int{4, 8, 16, 32, 64}
	for _, size := range sizes {
		// Create random matrices
		A := make([][]Element, size)
		B := make([][]Element, size)
		for i := 0; i < size; i++ {
			A[i] = make([]Element, size)
			B[i] = make([]Element, size)
			for j := 0; j < size; j++ {
				valA := rand.Int63n(1000000)
				valB := rand.Int63n(1000000)
				A[i][j] = field.FromBytes(big.NewInt(valA).Bytes())
				B[i][j] = field.FromBytes(big.NewInt(valB).Bytes())
			}
		}

		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = MatrixMultiply(A, B, field)
			}
		})
	}
}

func BenchmarkInvertMatrix(b *testing.B) {
	field := NewPrimeField(big.NewInt(4_294_967_311))

	sizes := []int{4, 8, 16, 32, 64}
	for _, size := range sizes {
		// Create a random invertible matrix
		A := make([][]Element, size)
		for i := 0; i < size; i++ {
			A[i] = make([]Element, size)
			for j := 0; j < size; j++ {
				val := rand.Int63n(1000000)
				A[i][j] = field.FromBytes(big.NewInt(val).Bytes())
			}
		}

		// Make sure the matrix is invertible by setting diagonal to non-zero
		for i := 0; i < size; i++ {
			A[i][i] = field.FromBytes(big.NewInt(int64(i + 1)).Bytes())
		}

		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				// Create a copy of A since InvertMatrix modifies it
				Acopy := make([][]Element, size)
				for j := 0; j < size; j++ {
					Acopy[j] = make([]Element, size)
					copy(Acopy[j], A[j])
				}
				_, err := InvertMatrix(Acopy, field)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkRecoverVectors(b *testing.B) {
	field := NewPrimeField(big.NewInt(4_294_967_311))

	// Test different matrix sizes and number of result vectors
	sizes := []struct {
		n, m int // n x n matrix, m result vectors
	}{
		{4, 1},
		{4, 4},
		{8, 1},
		{8, 8},
		{16, 1},
		{16, 4},
		{32, 1},
		{32, 4},
	}

	for _, size := range sizes {
		// Create square coefficient matrix A (n x n)
		A := make([][]Element, size.n)
		for i := 0; i < size.n; i++ {
			A[i] = make([]Element, size.n)
			for j := 0; j < size.n; j++ {
				val := rand.Int63n(1000000)
				A[i][j] = field.FromBytes(big.NewInt(val).Bytes())
			}
		}

		// Make sure the matrix is invertible by setting diagonal to non-zero
		for i := 0; i < size.n; i++ {
			A[i][i] = field.FromBytes(big.NewInt(int64(i + 1)).Bytes())
		}

		// Create result vectors R (n x m)
		R := make([][]Element, size.n)
		for i := 0; i < size.n; i++ {
			R[i] = make([]Element, size.m)
			for j := 0; j < size.m; j++ {
				val := rand.Int63n(1000000)
				R[i][j] = field.FromBytes(big.NewInt(val).Bytes())
			}
		}

		b.Run(fmt.Sprintf("n=%d_m=%d", size.n, size.m), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				// Create copies since RecoverVectors modifies the inputs
				Acopy := make([][]Element, size.n)
				for j := 0; j < size.n; j++ {
					Acopy[j] = make([]Element, size.n)
					copy(Acopy[j], A[j])
				}
				Rcopy := make([][]Element, size.n)
				for j := 0; j < size.n; j++ {
					Rcopy[j] = make([]Element, size.m)
					copy(Rcopy[j], R[j])
				}

				_, err := RecoverVectors(Acopy, Rcopy, field)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
