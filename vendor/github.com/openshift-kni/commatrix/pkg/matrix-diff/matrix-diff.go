package matrixdiff

import (
	"fmt"

	"github.com/openshift-kni/commatrix/pkg/types"
)

type status int

// Both = cd's present in both mat1 and mat2.
// UniqueA = cd's present in mat1 but not in mat2.
// UniqueB = cd's present in mat2 but not in mat1.
const (
	both status = iota
	uniqueA
	uniqueB
)

type MatrixDiff struct {
	types.ComMatrix
	cdToStatus map[string]status
}

// Generates the diff between mat1 to mat2.
func Generate(mat1 *types.ComMatrix, mat2 *types.ComMatrix) MatrixDiff {
	matrix := types.ComMatrix{}
	epsStatus := map[string]status{}

	for _, cd := range mat1.Matrix {
		matrix.Matrix = append(matrix.Matrix, cd)
		epsStatus[cd.String()] = both

		if !mat2.Contains(cd) {
			epsStatus[cd.String()] = uniqueA
		}
	}

	for _, cd := range mat2.Matrix {
		matrix.Matrix = append(matrix.Matrix, cd)
		epsStatus[cd.String()] = both

		if !mat1.Contains(cd) {
			epsStatus[cd.String()] = uniqueB
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
		case uniqueA:
			// add "+" before cd's present in mat1 but not in mat2.
			diff += fmt.Sprintf("+ %s\n", cd)
		case uniqueB:
			// add "-" before cd's present in mat2 but not in mat1.
			diff += fmt.Sprintf("- %s\n", cd)
		}
	}

	return diff, nil
}
