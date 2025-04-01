package matrixdiff

import (
	"fmt"

	"github.com/openshift-kni/commatrix/pkg/types"
)

type status int

// Both = cd's present in both primary mat and secondary mat.
// UniquePrimary = cd's present in primary mat but not in secondary mat.
// UniqueSecondary = cd's present in secondary mat but not in primary mat.
const (
	both status = iota
	uniquePrimary
	uniqueSecondary
)

// MatrixDiff represent the diff between two comMatrices.
type MatrixDiff struct {
	// Matrix Diff's ComMatrix is the combined matrix of the primary and secondary matrices.
	types.ComMatrix
	cdToStatus map[string]status
}

// Generates the diff between primary mat to secondary mat.
func Generate(primary *types.ComMatrix, secondary *types.ComMatrix) MatrixDiff {
	matrix := types.ComMatrix{}
	epsStatus := map[string]status{}

	for _, cd := range primary.Matrix {
		matrix.Matrix = append(matrix.Matrix, cd)
		epsStatus[cd.String()] = both

		if !secondary.Contains(cd) {
			epsStatus[cd.String()] = uniquePrimary
		}
	}

	for _, cd := range secondary.Matrix {
		matrix.Matrix = append(matrix.Matrix, cd)
		epsStatus[cd.String()] = both

		if !primary.Contains(cd) {
			epsStatus[cd.String()] = uniqueSecondary
		}
	}

	matrix.SortAndRemoveDuplicates()

	return MatrixDiff{matrix, epsStatus}
}

func (m *MatrixDiff) String() (string, error) {
	colNames, err := types.GetComMatrixHeadersByFormat(types.FormatCSV)
	if err != nil {
		return "", fmt.Errorf("error getting commatrix CSV tags: %v", err)
	}
	diff := colNames + "\n"

	for _, cd := range m.Matrix {
		switch m.cdToStatus[cd.String()] {
		case both:
			diff += fmt.Sprintf("%s\n", cd)
		case uniquePrimary:
			// add "+" before cd's present in primary mat but not in secondary mat.
			diff += fmt.Sprintf("+ %s\n", cd)
		case uniqueSecondary:
			// add "-" before cd's present in secondary mat but not in primary mat.
			diff += fmt.Sprintf("- %s\n", cd)
		}
	}

	return diff, nil
}

// Get the unique entries in primary mat.
func (m *MatrixDiff) GetUniquePrimary() *types.ComMatrix {
	matrix := types.ComMatrix{}

	for _, cd := range m.Matrix {
		if m.cdToStatus[cd.String()] == uniquePrimary {
			matrix.Matrix = append(matrix.Matrix, cd)
		}
	}

	return &matrix
}

// Get the unique entries in secondary mat.
func (m *MatrixDiff) GetUniqueSecondary() *types.ComMatrix {
	matrix := types.ComMatrix{}

	for _, cd := range m.Matrix {
		if m.cdToStatus[cd.String()] == uniqueSecondary {
			matrix.Matrix = append(matrix.Matrix, cd)
		}
	}

	return &matrix
}

// Get the common entries in both mat.
func (m *MatrixDiff) GetSharedEntries() *types.ComMatrix {
	matrix := types.ComMatrix{}

	for _, cd := range m.Matrix {
		if m.cdToStatus[cd.String()] == both {
			matrix.Matrix = append(matrix.Matrix, cd)
		}
	}

	return &matrix
}
