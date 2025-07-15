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

	// Make a deep copy of the matrix
	A := make([][]Element, n)
	for i := range vectors {
		A[i] = make([]Element, m)
		for j := range vectors[i] {
			A[i][j] = vectors[i][j].Clone()
		}
	}

	// Use Gaussian elimination to compute rank
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

		// Swap to top
		A[rank], A[pivot] = A[pivot], A[rank]

		// Normalize pivot row
		inv := A[rank][col].Inv()
		for j := col; j < m; j++ {
			A[rank][j] = A[rank][j].Mul(inv)
		}

		// Eliminate below and above
		for i := 0; i < n; i++ {
			if i == rank {
				continue
			}
			f := A[i][col].Clone()
			for j := col; j < m; j++ {
				tmp := f.Mul(A[rank][j])
				A[i][j] = A[i][j].Sub(tmp)
			}
		}
		rank++
	}

	// Vectors are independent if rank equals number of vectors
	return rank == n
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
		// Find pivot
		if B[i][i].IsZero() {
			return nil, fmt.Errorf("matrix not invertible")
		}
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
