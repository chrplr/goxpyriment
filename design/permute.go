// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package design

import (
	"fmt"
)

// Permutation types
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

func balancedLatinSquareSequence(nElements, row int) []int {
	result := make([]int, 0, nElements)
	j := 0
	h := 0

	for i := 0; i < nElements; i++ {
		var val int
		if i < 2 || i%2 != 0 {
			val = j
			j++
		} else {
			val = nElements - h - 1
			h++
		}
		result = append(result, (val+row)%nElements)
	}

	if nElements%2 != 0 && row%2 != 0 {
		// Reverse in place
		for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
			result[i], result[j] = result[j], result[i]
		}
	}
	return result
}

// LatinSquare returns a latin square permutation of elements.
// If elements is a list, it returns a square array of those elements.
// For simplicity in Go, we'll provide a version for ints and a generic one for slices.
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
		columns, _ := LatinSquareInts(n, PCycledLatinSquare)
		
		// Make index list to sort columns [0,1,n-1,3,n-2,4,...]
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

// LatinSquare returns a latin square permutation of elements.
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
