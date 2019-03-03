package Schultz

import (
	polycommit "../../utils/polycommit/pbc"
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"github.com/ncw/gmp"
	"github.com/stretchr/testify/assert"
	"testing"
)

type s struct {
	P *gmp.Int
}

func TestGobGmpInt(t *testing.T) {
	ps := new(s)

	ps.P = gmp.NewInt(66)

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	err := enc.Encode(ps)
	assert.Nil(t, err)

	ps2 := new(s)
	dec := gob.NewDecoder(&buf)
	err = dec.Decode(&ps2)
	assert.Nil(t, err)
}

func TestProposal_GobEncode(t *testing.T) {
	pp := BuildConfig(
		1,
		polycommit.Curve.Ngmp,
		[]int64{1, 2, 3, 4},
		[]int64{1, 2, 3, 4},
	)

	p := GenerateProposal(pp)

	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)

	err := enc.Encode(p)
	if err != nil {
		assert.Fail(t, err.Error())
	}

	dec := gob.NewDecoder(&buf)

	pNew := Proposal{}

	err = dec.Decode(&pNew)
	assert.Nil(t, err)

	assert.True(t, p.Equal(pNew))

	pHash := p.Hash()
	pNewHash := pNew.Hash()
	assert.True(t, pNew.Equal(p), "decoding")

	assert.Equal(t, hex.EncodeToString(pHash[:]), hex.EncodeToString(pNewHash[:]))

}

func genProposalWithDegree(degree int) int {
	pp := BuildConfig(
		degree,
		polycommit.Curve.Ngmp,
		makeOneToN(3*degree+1),
		makeOneToN(3*degree+1),
	)

	p := GenerateProposal(pp)

	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)

	err := enc.Encode(p)
	if err != nil {
		panic(err.Error())
	}

	return buf.Len()
}

func makeOneToN(n int) []int64 {
	a := make([]int64, n)
	for i := range a {
		a[i] = int64(i + 1)
	}
	return a
}

func TestGenerateProposal(t *testing.T) {
	//for _, d := range []int{2, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55, 60, 65, 70, 75, 80, 85, 90, 95, 100} {
	//	size := genProposalWithDegree(d)
	//	fmt.Printf("degree=%d, groupsize=%d, proposalsize=%d\n", d, 3*d+1, size)
	//}
}

//func BenchmarkGenerateProposal100(b *testing.B) {
//	genProposalWithDegree(100)
//}
