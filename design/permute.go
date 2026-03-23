// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package design

import (
	"fmt"
)

// Permutation types for Latin square generation.
const (
	PBalancedLatinSquare = "balanced"
	PCycledLatinSquare   = "cycled"
	PRandom              = "random"
)

// IsPermutationType returns true if the string is a known permutation type.
func IsPermutationType(typeStr string) bool {
	return typeStr == PRandom || typeStr == PCycledLatinSquare || typeStr == PBalancedLatinSquare
}

func cycleList[T any](arr []T) []T {
	if len(arr) <= 1 {
		return arr
	}
	rtn := make([]T, len(arr))
	copy(rtn, arr[1:])
	rtn[len(arr)-1] = arr[0]
	return rtn
}

// balancedLatinSquareSequence generates one row of a balanced Latin square.
//
// A balanced Latin square ensures that every condition (element) follows every
// other condition equally often, controlling for first-order carry-over effects.
// This is important in within-subjects experimental designs where the preceding
// trial can influence the current one (e.g. priming, fatigue).
//
// The algorithm (from Bradley, 1958) builds each row by interleaving ascending
// indices from the front (j) with descending indices from the back (h), then
// rotating by the row number. For odd-length squares, odd-numbered rows are
// reversed to complete the balancing.
//
// For n=4, row 0 produces: [0, 1, 3, 2] (front, front, back, front pattern).
func balancedLatinSquareSequence(nElements, row int) []int {
	result := make([]int, 0, nElements)
	j := 0 // ascending index (counts up from 0)
	h := 0 // descending index (counts down from nElements-1)

	for i := 0; i < nElements; i++ {
		var val int
		if i < 2 || i%2 != 0 {
			// Odd positions and the first two: take from the ascending front.
			val = j
			j++
		} else {
			// Even positions (after the first two): take from the descending back.
			val = nElements - h - 1
			h++
		}
		// Rotate by the row number to produce a different permutation per row.
		result = append(result, (val+row)%nElements)
	}

	// For odd n, reverse odd-numbered rows to double the square size and
	// achieve full carry-over balance (since an n×n square alone cannot be
	// balanced when n is odd).
	if nElements%2 != 0 && row%2 != 0 {
		for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
			result[i], result[j] = result[j], result[i]
		}
	}
	return result
}

// LatinSquareInts returns an n×n Latin square of integers 0..n-1 using the given
// permutation type (PBalancedLatinSquare, PCycledLatinSquare, or PRandom).
func LatinSquareInts(n int, permutationType string) ([][]int, error) {
	if !IsPermutationType(permutationType) {
		return nil, fmt.Errorf("unknown permutation type: %s", permutationType)
	}

	var square [][]int

	switch permutationType {
	case PCycledLatinSquare:
		row := make([]int, n)
		for i := 0; i < n; i++ {
			row[i] = i
		}
		square = append(square, row)
		for r := 0; r < n-1; r++ {
			square = append(square, cycleList(square[r]))
		}

	case PBalancedLatinSquare:
		rows := n
		if n%2 != 0 {
			rows = n * 2
		}
		for x := 0; x < rows; x++ {
			square = append(square, balancedLatinSquareSequence(n, x))
		}

	default: // PRandom
		// Start from a cycled Latin square, then reorder its columns in a
		// zigzag pattern (0, 1, n-1, 3, n-2, 4, …) to break sequential
		// regularity, and finally randomize the element labels so that the
		// result looks random while preserving the Latin-square property
		// (every element appears exactly once per row and column).
		columns, _ := LatinSquareInts(n, PCycledLatinSquare)

		// Build a zigzag column index: alternate taking from front and back
		// of the remaining indices [2, 3, …, n-1].
		cIdx := []int{0, 1}
		tmp := make([]int, n-2)
		for i := 0; i < n-2; i++ {
			tmp[i] = i + 2
		}

		takeLast := true
		for len(tmp) > 0 {
			if takeLast {
				cIdx = append(cIdx, tmp[len(tmp)-1])
				tmp = tmp[:len(tmp)-1]
			} else {
				cIdx = append(cIdx, tmp[0])
				tmp = tmp[1:]
			}
			takeLast = !takeLast
		}

		// Write sorted columns to square
		square = make([][]int, n)
		for r := 0; r < n; r++ {
			square[r] = make([]int, n)
			for c := 0; c < n; c++ {
				square[r][c] = columns[cIdx[c]][r]
			}
		}

		// Randomise counter elements
		indices := RandIntSequence(0, n-1)
		finalSquare := make([][]int, len(square))
		for r := range square {
			finalSquare[r] = make([]int, len(square[r]))
			for c := range square[r] {
				finalSquare[r][c] = indices[square[r][c]]
			}
		}
		square = finalSquare
	}

	return square, nil
}

// LatinSquare returns a Latin square (matrix of rows) over the given elements,
// using the same permutation types as LatinSquareInts.
func LatinSquare[T any](elements []T, permutationType string) ([][]T, error) {
	n := len(elements)
	idxSquare, err := LatinSquareInts(n, permutationType)
	if err != nil {
		return nil, err
	}

	res := make([][]T, len(idxSquare))
	for r := range idxSquare {
		res[r] = make([]T, len(idxSquare[r]))
		for c := range idxSquare[r] {
			res[r][c] = elements[idxSquare[r][c]]
		}
	}
	return res, nil
}
