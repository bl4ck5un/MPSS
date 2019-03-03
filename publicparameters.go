package Schultz

import (
	"github.com/ncw/gmp"
)

type PublicParameter struct {
	degree int
	// prime for Fp
	prime *gmp.Int

	oldGroup []int64
	newGroup []int64
}

func (c PublicParameter) GetThreshold() int {
	return c.degree
}

func (c PublicParameter) GetDegree() int {
	return c.degree
}

func (c PublicParameter) GetPrime() *gmp.Int {
	return c.prime
}

func BuildConfig(polydegree int, prime *gmp.Int, oldGroup, newGroup []int64) PublicParameter {
	return PublicParameter{
		degree:   polydegree,
		prime:    prime,
		oldGroup: oldGroup,
		newGroup: newGroup,
	}
}
