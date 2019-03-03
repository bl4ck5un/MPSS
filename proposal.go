package Schultz

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"../../utils/conv"
	polycommit "../../utils/polycommit/pbc"
	"../../utils/polyring"
	"../../utils/vector"
	"github.com/ncw/gmp"
	log "github.com/sirupsen/logrus"
)

type OldNodeID int32
type NewNodeID int32

type PointsOnBlindingPoly struct {
	points map[NewNodeID]*gmp.Int
}

func (pz PointsOnBlindingPoly) Equal(other PointsOnBlindingPoly) bool {
	if len(pz.points) != len(other.points) {
		return false
	}

	for id, p := range pz.points {
		if pOther, ok := other.points[id]; ok {
			if 0 != p.Cmp(pOther) {
				return false
			}
		} else {
			return false
		}
	}

	return true
}

func (pz PointsOnBlindingPoly) GobEncode() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	if err := enc.Encode(pz.points); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (pz *PointsOnBlindingPoly) GobDecode(buf []byte) error {
	r := bytes.NewBuffer(buf)
	dec := gob.NewDecoder(r)

	if err := dec.Decode(&pz.points); err != nil {
		return err
	}

	return nil
}

func (pz PointsOnBlindingPoly) Bytes() []byte {
	var buf bytes.Buffer

	// To store the keys in slice in sorted order
	var keys []NewNodeID
	for k := range pz.points {
		keys = append(keys, k)
	}

	sort.SliceStable(keys, func(i, j int) bool { return keys[i] < keys[j] })

	for _, k := range keys {
		buf.Write(pz.points[k].Bytes())
	}

	return buf.Bytes()
}

func (pz PointsOnBlindingPoly) String() string {
	s := ""
	for id, point := range pz.points {
		s += fmt.Sprintf("%d => %s, ", id, point.String())
	}

	return s
}

type Proposal struct {
	commQ polycommit.PolyCommit
	// one for each new node
	commRs map[NewNodeID]polycommit.PolyCommit
	// one for each old node
	pointToPeers map[OldNodeID]PointsOnBlindingPoly
}

func (p Proposal) GobEncode() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	if err := enc.Encode(p.commQ); err != nil {
		return nil, err
	}

	if err := enc.Encode(p.commRs); err != nil {
		return nil, err
	}

	if err := enc.Encode(p.pointToPeers); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (p *Proposal) GobDecode(buf []byte) error {
	r := bytes.NewBuffer(buf)
	dec := gob.NewDecoder(r)

	if err := dec.Decode(&p.commQ); err != nil {
		return err
	}

	if err := dec.Decode(&p.commRs); err != nil {
		return err
	}

	return dec.Decode(&p.pointToPeers)
}

func (p Proposal) Equal(other Proposal) bool {
	if !p.commQ.Equals(other.commQ) {
		return false
	}

	if len(p.commRs) != len(other.commRs) {
		return false
	}

	for i, v := range p.commRs {
		if !v.Equals(other.commRs[i]) {
			return false
		}
	}

	if len(p.pointToPeers) != len(p.pointToPeers) {
		return false
	}

	for id, pp := range p.pointToPeers {
		if pOther, ok := other.pointToPeers[id]; ok {
			if !pp.Equal(pOther) {
				return false
			}
		} else {
			return false
		}
	}

	return true
}

func (p Proposal) Hash() [32]byte {
	hash := sha256.New()

	hash.Write(p.commQ.Bytes())

	{
		// To store the keys in slice in sorted order
		var keys []NewNodeID
		for k := range p.commRs {
			keys = append(keys, k)
		}

		sort.SliceStable(keys, func(i, j int) bool { return keys[i] < keys[j] })

		for _, k := range keys {
			hash.Write(p.commRs[k].Bytes())
		}
	}

	{
		// To store the keys in slice in sorted order
		var keys []OldNodeID
		for k := range p.pointToPeers {
			keys = append(keys, k)
		}

		sort.SliceStable(keys, func(i, j int) bool { return keys[i] < keys[j] })

		for _, k := range keys {
			hash.Write(p.pointToPeers[k].Bytes())
		}
	}

	var result [32]byte
	copy(result[:], hash.Sum(nil))

	return result
}

func (p Proposal) ToBytes() []byte {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(p); err != nil {
		panic(err.Error())
	}

	return buf.Bytes()
}

func (p Proposal) String() string {
	s := fmt.Sprintf("Comm(Q): %s\n", p.commQ.String())
	for _, comm := range p.commRs {
		s += fmt.Sprintf("-- Comm(Rs): %s\n", comm.String())
	}
	for i, peer := range p.pointToPeers {
		s += fmt.Sprintf("points to peer %d: %s\n", i, peer.String())
	}

	return strings.TrimSuffix(s, "\n")
}

func (p Proposal) Print() {
	fmt.Println(p.String())
}

func GenerateProposal(pp PublicParameter) Proposal {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	Q, err := polyring.NewRand(pp.degree, r, pp.prime)
	if err != nil {
		panic(err.Error())
	}

	// indices of old group nodes
	oldGroupIndices := vector.FromInt64(pp.oldGroup...).GetPtr()

	// make it zero know
	Q.GetPtrToConstant().SetUint64(0)
	// commit to it!
	commQ := polycommit.NewPolyCommit(Q)

	// blinding polynomials
	blindingPolys := make(map[NewNodeID]polyring.Polynomial, len(pp.newGroup))
	commBlindingPolyList := make(map[NewNodeID]polycommit.PolyCommit, len(pp.newGroup))

	for _, newNodeId := range pp.newGroup {
		blindingPolyForI, err := polyring.NewRand(pp.degree-1, r, pp.prime)
		if err != nil {
			panic(err.Error())
		}

		// Rk = Rk * (x-newNodeId)
		blindingPolyForI.MulSelf(polyring.FromVec(int64(-newNodeId), 1))
		blindingPolyForI.Mod(pp.prime)

		// store the blinding poly
		blindingPolys[NewNodeID(newNodeId)] = blindingPolyForI

		// commitment to the blinding polynomials
		commBlindingPolyList[NewNodeID(newNodeId)] = polycommit.NewPolyCommit(blindingPolyForI)
	}

	// for each old group member, evaluate points on blinding polynomials
	pointsOnQ := vector.New(len(pp.oldGroup))
	Q.EvalModArray(oldGroupIndices, pp.prime, pointsOnQ.GetPtr())

	proposal := Proposal{
		commQ,
		commBlindingPolyList,
		make(map[OldNodeID]PointsOnBlindingPoly, len(pp.oldGroup)),
	}

	// j is the id for an old group member
	for _, j := range pp.oldGroup {
		nodeJ := gmp.NewInt(int64(j))
		Qj := gmp.NewInt(0)
		Q.EvalMod(nodeJ, pp.prime, Qj)

		if !commQ.VerifyEval(conv.GmpInt2BigInt(nodeJ), conv.GmpInt2BigInt(Qj)) {
			log.Fatalf("Q(j) not on Q, which is a bug")
		}

		BlidingPointsForJ := make(map[NewNodeID]*gmp.Int, len(pp.newGroup))

		for nodeK, Rk := range blindingPolys {
			Rkj := gmp.NewInt(0)
			Rk.EvalMod(nodeJ, pp.prime, Rkj)

			BlidingPointsForJ[NewNodeID(nodeK)] = gmp.NewInt(0)
			BlidingPointsForJ[NewNodeID(nodeK)].Add(Qj, Rkj)
		}

		proposal.pointToPeers[OldNodeID(j)] = PointsOnBlindingPoly{
			points: BlidingPointsForJ,
		}
	}

	return proposal
}
