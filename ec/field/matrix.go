package field

import (
	"fmt"
)

// Matrix operations over finite fields

// IsLinearlyIndependent checks if the list of field element vectors is linearly independent.
func IsLinearlyIndependent(vectors [][]Element, field Field) bool {
	n := len(vectors) // number of vectors
	if n == 0 {
		return true // empty set is vacuously independent
	}
	m := len(vectors[0]) // dimension of each vector

	// Early exit: if more vectors than dimensions, they must be dependent
	if n > m {
		return false
	}

	// Make a deep copy of the matrix
	A := make([][]Element, n)
	for i := range vectors {
		A[i] = make([]Element, m)
		for j := range vectors[i] {
			A[i][j] = vectors[i][j].Clone()
		}
	}

	// Use forward elimination only to compute rank - more efficient than full RREF
	rank := 0
	for col := 0; col < m && rank < n; col++ {
		// Find pivot
		pivot := -1
		for i := rank; i < n; i++ {
			if !A[i][col].IsZero() {
				pivot = i
				break
			}
		}
		if pivot == -1 {
			continue // no pivot in this column
		}

		// Swap to current rank position
		if pivot != rank {
			A[rank], A[pivot] = A[pivot], A[rank]
		}

		// Eliminate below only (forward elimination)
		for i := rank + 1; i < n; i++ {
			if A[i][col].IsZero() {
				continue
			}
			// Use A[i][col] / A[rank][col] as elimination factor
			factor := A[i][col].Mul(A[rank][col].Inv())
			for j := col; j < m; j++ {
				tmp := factor.Mul(A[rank][j])
				A[i][j] = A[i][j].Sub(tmp)
			}
		}
		rank++
	}

	// Vectors are independent if rank equals number of vectors
	return rank == n
}

// IsLinearlyIndependentIncremental checks if adding a new vector to an existing REF matrix maintains linear independence.
// Returns the updated REF matrix and whether the vector was linearly independent.
// This is optimized for the case where existingVectors are in Row Echelon Form (REF).
// REF assumption allows for O(nm) complexity instead of O(n²m) while preserving existing vectors unchanged.
func IsLinearlyIndependentIncremental(existingVectors [][]Element, newVector []Element, field Field) ([][]Element, bool) {
	if len(existingVectors) == 0 {
		// Check if new vector is non-zero
		for _, elem := range newVector {
			if !elem.IsZero() {
				// Return single-row REF with the new vector
				return [][]Element{append([]Element(nil), newVector...)}, true
			}
		}
		return nil, false // zero vector is not independent
	}

	n := len(existingVectors)
	m := len(newVector)

	// Early exit: if we already have m linearly independent vectors in m-dimensional space,
	// adding another vector will make them dependent
	if n >= m {
		return nil, false
	}

	// Since existingVectors are in REF, we can eliminate the new vector efficiently
	// REF means each row has a leading (pivot) element, and all elements below each pivot are zero
	newVectorCopy := make([]Element, m)
	for j := 0; j < m; j++ {
		newVectorCopy[j] = newVector[j].Clone()
	}

	// Process each existing vector (in REF order)
	for i := 0; i < n; i++ {
		// Find the pivot column for this REF row
		pivotCol := -1
		for j := 0; j < m; j++ {
			if !existingVectors[i][j].IsZero() {
				pivotCol = j
				break
			}
		}
		if pivotCol == -1 {
			continue // zero row (shouldn't happen in valid REF)
		}

		// Eliminate this pivot position in newVector
		if !newVectorCopy[pivotCol].IsZero() {
			// factor = newVector[pivotCol] / existingVectors[i][pivotCol]
			factor := newVectorCopy[pivotCol].Mul(existingVectors[i][pivotCol].Inv())
			for j := pivotCol; j < m; j++ {
				tmp := factor.Mul(existingVectors[i][j])
				newVectorCopy[j] = newVectorCopy[j].Sub(tmp)
			}
		}
	}

	// Check if newVector is non-zero after elimination
	// If it's all zeros, it was a linear combination of existing vectors
	isIndependent := false
	newVectorPivot := -1
	for j := 0; j < m; j++ {
		if !newVectorCopy[j].IsZero() {
			isIndependent = true
			newVectorPivot = j
			break
		}
	}

	if !isIndependent {
		return nil, false // vector was dependent
	}

	// Find where to insert the new vector to maintain REF order
	// REF requires each pivot to be to the right of the pivot above it
	insertPos := n // Default: append at end
	for i := 0; i < n; i++ {
		// Find pivot of existing vector i
		existingPivot := -1
		for j := 0; j < m; j++ {
			if !existingVectors[i][j].IsZero() {
				existingPivot = j
				break
			}
		}

		// If new vector's pivot comes before this existing vector's pivot,
		// insert new vector at position i
		if newVectorPivot < existingPivot {
			insertPos = i
			break
		}
	}

	// Create new REF matrix with proper ordering
	newREF := make([][]Element, n+1)

	// Copy vectors before insertion point
	for i := 0; i < insertPos; i++ {
		newREF[i] = make([]Element, m)
		copy(newREF[i], existingVectors[i])
	}

	// Insert the new vector at the correct position
	newREF[insertPos] = newVectorCopy

	// Copy vectors after insertion point
	for i := insertPos; i < n; i++ {
		newREF[i+1] = make([]Element, m)
		copy(newREF[i+1], existingVectors[i])
	}

	return newREF, true
}

// IsRowEchelonForm checks if a matrix is in Row Echelon Form (REF).
// REF requirements:
// 1. All non-zero rows are above any zero rows
// 2. Each leading entry (pivot) of a row is to the right of the leading entry of the row above it
// 3. All entries in a column below a leading entry are zeros
func IsRowEchelonForm(matrix [][]Element) bool {
	if len(matrix) == 0 {
		return true // empty matrix is trivially in REF
	}

	prevPivotCol := -1

	for i, row := range matrix {
		// Find the pivot column for this row
		pivotCol := -1
		for j, elem := range row {
			if !elem.IsZero() {
				pivotCol = j
				break
			}
		}

		// If this is a zero row
		if pivotCol == -1 {
			// All remaining rows must also be zero rows
			for k := i + 1; k < len(matrix); k++ {
				for _, elem := range matrix[k] {
					if !elem.IsZero() {
						return false // non-zero row after zero row
					}
				}
			}
			break // rest are zero rows, we're done
		}

		// Check that pivot is to the right of previous pivot
		if pivotCol <= prevPivotCol {
			return false // pivot not to the right of previous pivot
		}

		// Check that all entries below this pivot are zero
		for k := i + 1; k < len(matrix); k++ {
			if pivotCol < len(matrix[k]) && !matrix[k][pivotCol].IsZero() {
				return false // non-zero entry below pivot
			}
		}

		prevPivotCol = pivotCol
	}

	return true
}

// InvertMatrix computes the inverse of an n x n matrix over the field using Gaussian elimination.
func InvertMatrix(A [][]Element, field Field) ([][]Element, error) {
	n := len(A)
	// Initialize inverse matrix as identity matrix
	inv := make([][]Element, n)
	for i := range inv {
		inv[i] = make([]Element, n)
		for j := range inv[i] {
			if i == j {
				inv[i][j] = field.One()
			} else {
				inv[i][j] = field.Zero()
			}
		}
	}

	// Make a deep copy of A to work on
	B := make([][]Element, n)
	for i := range A {
		B[i] = make([]Element, n)
		for j := range A[i] {
			B[i][j] = A[i][j].Clone()
		}
	}

	// Perform Gaussian elimination with pivoting
	for i := 0; i < n; i++ {
		// Find pivot: look for a non-zero element in column i
		pivot := -1
		for k := i; k < n; k++ {
			if !B[k][i].IsZero() {
				pivot = k
				break
			}
		}

		// If no pivot found, matrix is singular
		if pivot == -1 {
			return nil, fmt.Errorf("matrix not invertible")
		}

		// Swap rows if needed
		if pivot != i {
			B[i], B[pivot] = B[pivot], B[i]
			inv[i], inv[pivot] = inv[pivot], inv[i]
		}

		// Normalize the pivot row
		invPivot := B[i][i].Inv()
		for j := 0; j < n; j++ {
			B[i][j] = B[i][j].Mul(invPivot)
			inv[i][j] = inv[i][j].Mul(invPivot)
		}

		// Eliminate other rows
		for k := 0; k < n; k++ {
			if k == i {
				continue
			}
			factor := B[k][i].Clone()
			for j := 0; j < n; j++ {
				tmp := factor.Mul(B[i][j])
				B[k][j] = B[k][j].Sub(tmp)

				tmp2 := factor.Mul(inv[i][j])
				inv[k][j] = inv[k][j].Sub(tmp2)
			}
		}
	}
	return inv, nil
}

// MatrixMultiply computes A × B matrix multiplication over the field
// A is m×n, B is n×p, result is m×p
func MatrixMultiply(A, B [][]Element, field Field) [][]Element {
	if len(A) == 0 || len(B) == 0 {
		return nil
	}

	m := len(A)    // rows of A
	n := len(A[0]) // cols of A = rows of B
	p := len(B[0]) // cols of B

	// Verify dimensions match
	if len(B) != n {
		panic(fmt.Sprintf("matrix dimensions mismatch: A is %d×%d, B is %d×%d", m, n, len(B), p))
	}

	// Create result matrix
	C := make([][]Element, m)
	for i := range C {
		C[i] = make([]Element, p)
		for j := 0; j < p; j++ {
			sum := field.Zero()
			for k := 0; k < n; k++ {
				product := A[i][k].Mul(B[k][j])
				sum = sum.Add(product)
			}
			C[i][j] = sum
		}
	}
	return C
}

// RecoverVectors solves V = A⁻¹ * R, where A is the coefficient matrix and R the combined vectors.
func RecoverVectors(A [][]Element, R [][]Element, field Field) ([][]Element, error) {
	n := len(A)
	m := len(R[0])
	Ainv, err := InvertMatrix(A, field)
	if err != nil {
		return nil, err
	}

	// Compute V = A⁻¹ * R using matrix multiplication
	V := make([][]Element, n)
	for i := range V {
		V[i] = make([]Element, m)
		for j := 0; j < m; j++ {
			V[i][j] = field.Zero()
			for k := 0; k < n; k++ {
				tmp := Ainv[i][k].Mul(R[k][j])
				V[i][j] = V[i][j].Add(tmp)
			}
		}
	}
	return V, nil
}
