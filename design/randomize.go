// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Co-authored by Claude Sonnet 4.6
// Distributed under the GNU General Public License v3.

package design

import (
	"math/rand"
)

// Note: Go 1.20+ automatically seeds the global RNG, so there is no need
// for an explicit rand.Seed() call here. The previous init() that called
// rand.Seed(time.Now().UnixNano()) has been removed.

// RandIntSequence returns a shuffled sequence of integers in [first, last] (inclusive).
func RandIntSequence(first, last int) []int {
	if first > last {
		return []int{}
	}
	res := make([]int, last-first+1)
	for i := range res {
		res[i] = first + i
	}
	rand.Shuffle(len(res), func(i, j int) {
		res[i], res[j] = res[j], res[i]
	})
	return res
}

// RandInt returns a random integer in the range [a, b].
func RandInt(a, b int) int {
	if a > b {
		return a
	}
	return rand.Intn(b-a+1) + a
}

// RandElement returns a random element from a slice.
func RandElement[T any](list []T) T {
	if len(list) == 0 {
		var zero T
		return zero
	}
	return list[rand.Intn(len(list))]
}

// CoinFlip returns true with probability headBias (0–1), false otherwise.
func CoinFlip(headBias float64) bool {
	return rand.Float64() <= headBias
}

// RandNorm returns a normally distributed random number in [a, b], truncated to that range.
func RandNorm(a, b float64) float64 {
	mu := a + (b-a)/2.0
	sigma := (b - a) / 6.0
	for {
		r := rand.NormFloat64()*sigma + mu
		if r >= a && r <= b {
			return r
		}
	}
}

// ShuffleList shuffles any slice in place.
func ShuffleList[T any](list []T) {
	rand.Shuffle(len(list), func(i, j int) {
		list[i], list[j] = list[j], list[i]
	})
}

// MakeMultipliedShuffledList concatenates xtimes copies of the list, each copy shuffled independently.
func MakeMultipliedShuffledList[T any](list []T, xtimes int) []T {
	newlist := make([]T, 0, len(list)*xtimes)
	for i := 0; i < xtimes; i++ {
		tmp := make([]T, len(list))
		copy(tmp, list)
		ShuffleList(tmp)
		newlist = append(newlist, tmp...)
	}
	return newlist
}
