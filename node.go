package Schultz

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"google.golang.org/grpc/status"
	"math/big"
	"net"
	"strconv"
	"sync"
	"time"

	"../../utils/conv"
	"../../utils/interpolation"
	polycommit "../../utils/polycommit/pbc"
	"./services"
	"github.com/golang/protobuf/proto"
	"github.com/montanaflynn/stats"
	"github.com/ncw/gmp"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

type Hash [32]byte

func (h Hash) Equal(o Hash) bool {
	if len(h) != len(o) {
		return false
	}

	for i := range h {
		if h[i] != o[i] {
			return false
		}
	}

	return true
}

type Node struct {
	id     int64
	config PublicParameter
	share  *gmp.Int

	myIP       string
	peerIPList map[NewNodeID]string
	primaryIP  string

	nodes       map[NewNodeID]services.NodeClient
	primaryNode services.BulletinBoardServiceClient

	blindedShareChan chan *services.BlindedShare
	proposalListChan chan *services.ProposalHashList
	proposalChan     chan *services.Proposal

	waitAdvancedEpoch *sync.WaitGroup

	// logging
	log *logrus.Entry
}

type BenchmarkEntry struct {
	latency       time.Duration
	bytesOnChain  int
	bytesOffChain int
}

type Benchmark map[Epoch]BenchmarkEntry

func (node *Node) Report(b *Benchmark) {
	var latency []float64
	var onChain []float64
	var offChain []float64

	for _, be := range *b {
		latency = append(latency, be.latency.Seconds())
		onChain = append(onChain, float64(be.bytesOnChain))
		offChain = append(offChain, float64(be.bytesOffChain))
	}

	latencyMean, _ := stats.Mean(latency)
	latencyStd, _ := stats.StandardDeviation(latency)

	onChainMean, _ := stats.Mean(onChain)
	onChainStd, _ := stats.StandardDeviation(onChain)

	offChainMean, _ := stats.Mean(offChain)
	offChainStd, _ := stats.StandardDeviation(offChain)

	node.log.WithFields(logrus.Fields{
		"degree":       node.config.degree,
		"groupsize":    len(node.config.oldGroup),
		"latencyMean":  latencyMean,
		"latencyStd":   latencyStd,
		"onChainMean":  onChainMean,
		"onChainStd":   onChainStd,
		"offChainMean": offChainMean,
		"offChainStd":  offChainStd,
	}).Warn("benchmark.")
}

func (node *Node) AdvanceEpoch(ctx context.Context, hashList *services.Empty) (*services.Empty, error) {
	node.log.Debugf("starting the protocol, as instructed by the primary")

	node.waitAdvancedEpoch.Done()

	return &services.Empty{}, nil
}

func (node *Node) StartCheckingProposals(ctx context.Context, hashList *services.ProposalHashList) (*services.Empty, error) {
	node.proposalListChan <- hashList

	node.log.Debugf("channel received hashes from the primary")

	return &services.Empty{}, nil
}

func (node *Node) startProposalHashCollector(e Epoch, b *BenchmarkEntry) chan map[int64]Hash {
	out := make(chan map[int64]Hash)

	go func() {
		proposalList := make(map[int64]Hash)

		for {
			list := <-node.proposalListChan

			if Epoch(list.Epoch) < e {
				node.log.Infof("ignoring message from a previous epoch: %d (at epoch %d)", list.Epoch, e)
				continue
			} else if Epoch(list.Epoch) > e {
				panic("this can not happen in out setting.")
			}

			for i := range list.List {
				pp := list.List[i]

				// benchmark
				b.bytesOnChain += proto.Size(pp)

				if len(pp.Hash) != sha256.Size {
					panic("wrong size")
				}

				var tmp Hash
				copy(tmp[:], pp.Hash)

				proposalList[pp.Proposer] = tmp
			}

			out <- proposalList
			break
		}
	}()

	return out
}

func (node *Node) SubmitProposal(ctx context.Context, proposal *services.Proposal) (*services.Empty, error) {
	sender, ok := peer.FromContext(ctx)
	if !ok {
		node.log.Error("can't get peer info")
	} else {
		node.log.Debugf("receiving a proposal from %s", sender.Addr)
	}
	// this should not block for too long
	node.proposalChan <- proposal

	return &services.Empty{}, nil
}

func (node *Node) startProposalCollector(e Epoch, b *BenchmarkEntry) chan map[NewNodeID]*gmp.Int {
	hashListChan := node.startProposalHashCollector(e, b)

	out := make(chan map[NewNodeID]*gmp.Int)

	allProposals := make(chan map[int64]*Proposal)

	// start receiving all proposals
	go func() {
		proposalReceived := make(map[int64]*Proposal)

		for {
			proposal := <-node.proposalChan

			// ignore proposals for a previous epoch
			if Epoch(proposal.Epoch) < e {
				node.log.Infof("ignoring proposal from a previous epoch: %d (at epoch %d)", proposal.Epoch, e)
				continue
			} else if Epoch(proposal.Epoch) > e {
				panic("this can not happen in out setting.")
			}

			// benchmark
			b.bytesOffChain += proto.Size(proposal)

			node.log.Debugf("proposal from=%d of size=%d", proposal.From, proto.Size(proposal))

			// reassemble protocol
			from := proposal.From

			buf := bytes.NewBuffer(proposal.Gob)
			dec := gob.NewDecoder(buf)

			p := Proposal{}
			dec.Decode(&p)

			proposalReceived[from] = &p

			node.log.Debugf("received a proposal from %d (%d / %d received)", from, len(proposalReceived), len(node.config.oldGroup))

			// break if one proposal from each node has been received
			if len(proposalReceived) >= len(node.config.oldGroup) {
				break
			}
		}

		node.log.Debugf("#proposals %d", len(proposalReceived))

		allProposals <- proposalReceived
	}()

	go func() {
		// block until receive all of the hashes from primary
		proposalListFromPrimary := <-hashListChan
		proposalReceived := <-allProposals

		var proposalVerified []int64

		for from, hashRef := range proposalListFromPrimary {
			proposal, ok := proposalReceived[from]
			if !ok {
				node.log.Errorf("can't find a proposal from %d, which appears in the primary's list", from)

				fmt.Printf("proposal received: ")
				for k := range proposalReceived {
					fmt.Printf("%d, ", k)
				}
				fmt.Printf("\n")
				panic("die")
			}

			if hashRef.Equal(proposal.Hash()) {
				proposalVerified = append(proposalVerified, from)
			} else {
				// should not happen
				panic("hash mismatch")
			}
		}

		node.log.Infof("Hash matched. proposal to use: %v", proposalVerified)

		myId := OldNodeID(node.id)
		myIdBig := big.NewInt(node.id)

		for _, pi := range proposalVerified {
			commQ := proposalReceived[pi].commQ
			commRs := proposalReceived[pi].commRs

			for newNodeK, pointOnQPlusRk := range proposalReceived[pi].pointToPeers[myId].points {
				Rk := commRs[newNodeK]
				comm := polycommit.AdditiveHomomorphism(commQ, Rk)
				if !comm.VerifyEval(myIdBig, conv.GmpInt2BigInt(pointOnQPlusRk)) {
					node.log.Errorf("points not on Q+Rk for k=%d", newNodeK)
				}
			}
		}

		// start to combine the proposal into one share for each new group node
		combinedNewShare := make(map[NewNodeID]*gmp.Int)
		for _, newNodeId := range node.config.newGroup {
			combinedNewShare[NewNodeID(newNodeId)] = gmp.NewInt(0)
			// set it to my share
			combinedNewShare[NewNodeID(newNodeId)].Set(node.share)
		}

		for _, pi := range proposalVerified {
			for newNodeK, pointOnQPlusRk := range proposalReceived[pi].pointToPeers[myId].points {
				p, ok := combinedNewShare[newNodeK]
				if !ok {
					fmt.Printf("%v\n", combinedNewShare)
					println(newNodeK)
					panic("can't happen")
				}

				p.Add(p, pointOnQPlusRk)
			}
		}

		out <- combinedNewShare
	}()

	return out
}

func (node *Node) SubmitBlindedShare(ctx context.Context, in *services.BlindedShare) (*services.Empty, error) {
	node.blindedShareChan <- in

	return &services.Empty{}, nil
}

func (node *Node) startShareReconstructor(epoch Epoch, b *BenchmarkEntry) <-chan *gmp.Int {
	out := make(chan *gmp.Int)

	go func() {
		sharesReceived := make(map[int64]*gmp.Int)
		i := 0
		for {
			share := <-node.blindedShareChan

			// ignore proposals for a previous epoch
			if Epoch(share.Epoch) < epoch {
				node.log.Warnf("ignoring proposal from a previous epoch: %d (at epoch %d)", share.Epoch, epoch)
				continue
			} else if Epoch(share.Epoch) > epoch {
				panic("this can not happen in out setting.")
			}

			node.log.Debugf("received a share from %d", share.From)

			// benchmark
			b.bytesOffChain += proto.Size(share)

			sharesReceived[share.From] = gmp.NewInt(0)
			sharesReceived[share.From].SetBytes(share.Share)

			i += 1
			if i >= len(node.config.oldGroup) {
				break
			}
		}

		node.log.Debugf("got enough to reconstruct new shares")

		var Xs []*gmp.Int
		var Ys []*gmp.Int

		for x, y := range sharesReceived {
			Xs = append(Xs, gmp.NewInt(int64(x)))
			Ys = append(Ys, y)
		}

		// reconstruct the share
		poly, err := interpolation.LagrangeInterpolate(node.config.degree, Xs, Ys, node.config.prime)
		if err != nil {
			panic("can't recover the secret")
		}

		newShare := gmp.NewInt(0)
		poly.EvalMod(gmp.NewInt(int64(node.id)), node.config.prime, newShare)

		out <- newShare
		close(out)
	}()

	return out
}

func (node *Node) SubmitShareToPrimary(epoch Epoch) {
	ctx := context.Background()
	msg := services.Share{
		Epoch: int32(epoch),
		From:  node.id,
		Share: node.share.Bytes(),
	}
	_, err := node.primaryNode.AssembleShare(ctx, &msg)
	if err != nil {
		panic(err.Error())
	}
}

func (node *Node) StartProtocol(wsFinish *sync.WaitGroup, maxEpoch Epoch) {
	epoch := Epoch(0)

	b := make(Benchmark)

	for {
		// wait for instructions from the primary and advance the epoch
		// epoch is only advanced here
		node.waitAdvancedEpoch.Wait()

		// enter the next epoch
		epoch += 1
		node.waitAdvancedEpoch.Add(1)
		// only run up to maxEpoch
		if epoch > maxEpoch {
			break
		}

		// connect to peers at the first epoch
		if epoch == 1 {
			if err := node.ConnectPeers(); err != nil {
				node.log.Fatalf("cannot connect to peers")
			}
		}

		// prepare for the benchmark
		benchmarkEntry := BenchmarkEntry{}

		node.log.Infof("entering epoch %d", epoch)

		// start the pipeline workers
		combinedProposalChan := node.startProposalCollector(epoch, &benchmarkEntry)

		// construct a new notification channel
		newShareChan := node.startShareReconstructor(epoch, &benchmarkEntry)

		// start the benchmark timer
		startTime := time.Now()

		p := GenerateProposal(node.config)

		// populate the message with a hash
		hash := p.Hash()
		proposalMsg := services.ProposalHash{
			Epoch:    int32(epoch),
			Proposer: node.id,
			Hash:     hash[:],
		}

		node.log.Debug("submitting hash to the primary")

		ctx := context.Background()
		_, err := node.primaryNode.SubmitProposalHash(ctx, &proposalMsg)
		if err != nil {
			st, ok := status.FromError(err)
			if !ok {
				node.log.Errorf("can't get status")
			}
			node.log.Errorf("%s", st.Message())
		}

		// send proposal messages to peers
		for i := range node.nodes {
			go func(dst NewNodeID) {
				pMsg := services.Proposal{
					Epoch: int32(epoch),
					From:  node.id,
					Gob:   p.ToBytes(),
				}

				node.log.Debugf("sending proposal to %d", dst)
				_, err := node.nodes[dst].SubmitProposal(ctx, &pMsg)
				if err != nil {
					st, ok := status.FromError(err)
					if !ok {
						node.log.Fatalf("can't get status")
					}
					node.log.Fatalf("error while sending proposal: %s", st.Message())
				}
			}(i)
		}

		// send a proposal to myself
		node.log.Debugf("sending myself a proposal")

		node.proposalChan <- &services.Proposal{
			Epoch: int32(epoch),
			From:  node.id,
			Gob:   p.ToBytes()}

		node.log.Debugf("done sending myself a proposal")

		// collect the combined proposal to be sent to new members
		combinedProposal := <-combinedProposalChan

		node.log.Infof("Proposal verified and new shares generated.")

		// handle the share to myself separately
		myReShare, ok := combinedProposal[NewNodeID(node.id)]
		if ok {
			node.log.Debugf("got a share for myself")

			node.blindedShareChan <- &services.BlindedShare{
				Epoch: int32(epoch),
				From:  node.id,
				Share: myReShare.Bytes()}

			// delete the share since we now have it
			delete(combinedProposal, NewNodeID(node.id))
		}

		for newNodeId, reShare := range combinedProposal {
			nodeClient, ok := node.nodes[NewNodeID(newNodeId)]
			if !ok {
				node.log.Fatalf("can't find the node client for %d", newNodeId)
			}

			node.log.Debugf("submitting a blinded share to %d", newNodeId)

			_, err := nodeClient.SubmitBlindedShare(ctx, &services.BlindedShare{
				Epoch: int32(epoch),
				From:  node.id,
				Share: reShare.Bytes(),
			})

			node.log.Debugf("a blinded share submitted to %d", newNodeId)

			if err != nil {
				panic(err.Error())
			}
		}

		newShare := <-newShareChan
		node.log.Infof("new share is %s", newShare.String())

		// benchmark
		endTime := time.Now()
		benchmarkEntry.latency = endTime.Sub(startTime)

		// store the benchmark results
		b[epoch] = benchmarkEntry

		// sending stuff to the primary
		node.log.Debugf("new share sending to the primary")
		_, err = node.primaryNode.AssembleShare(ctx, &services.Share{
			Epoch: int32(epoch),
			From:  node.id,
			Share: newShare.Bytes(),
		})

		node.log.Debugf("new share sent to the primary")

		if err != nil {
			panic("can't send stuff to the primary")
		}
	}

	node.Report(&b)

	ctx := context.Background()
	_, err := node.primaryNode.Kill(ctx, &services.Empty{})
	if err != nil {
		panic(err.Error())
	}

	wsFinish.Done()
	node.log.Infof("done")
}

func (node *Node) ConnectPeers() error {
	for nodeId, peerIP := range node.peerIPList {
		conn, err := grpc.Dial(peerIP, grpc.WithInsecure())
		if err != nil {
			return err
		}
		node.nodes[nodeId] = services.NewNodeClient(conn)
		node.log.Debugf("connected to a peer %d at %s", nodeId, peerIP)
	}

	return nil
}

func (node *Node) ConnectPrimary() error {
	node.log.Debugf("dialing the primary at %s", node.primaryIP)
	conn, err := grpc.Dial(node.primaryIP, grpc.WithInsecure())
	if err != nil {
		return err
	}

	node.primaryNode = services.NewBulletinBoardServiceClient(conn)
	node.log.Debugf("connected to the primary at %s", node.primaryIP)

	return nil
}

func (node *Node) Serve() {
	_, port, err := net.SplitHostPort(node.myIP)
	if err != nil {
		node.log.Fatalf("can't serve: %s", err.Error())
	}
	portUint, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		panic(err.Error())
	}

	hostPort := fmt.Sprintf("0.0.0.0:%d", portUint)

	lis, err := net.Listen("tcp4", hostPort)
	if err != nil {
		node.log.Fatalf("cannot listen to %s, %v", hostPort, err)
	}

	s := grpc.NewServer()
	services.RegisterNodeServer(s, node)

	node.log.Infof("serving on %s", hostPort)
	if err := s.Serve(lis); err != nil {
		node.log.Fatalf("can't serve")
	}
}

func BuildNode(pp PublicParameter, logger *logrus.Logger, id int64, primaryIP, myIP string, peerIPs map[NewNodeID]string, initShare *gmp.Int) Node {
	var wgStart sync.WaitGroup
	var wgStop sync.WaitGroup
	wgStart.Add(1)
	wgStop.Add(1)

	nodeLogger := logger.WithFields(
		logrus.Fields{
			"node": id,
		})

	return Node{
		id:                id,
		primaryIP:         primaryIP,
		myIP:              myIP,
		peerIPList:        peerIPs,
		config:            pp,
		waitAdvancedEpoch: &wgStart,
		share:             initShare,
		nodes:             make(map[NewNodeID]services.NodeClient),
		blindedShareChan:  make(chan *services.BlindedShare),
		proposalChan:      make(chan *services.Proposal),
		proposalListChan:  make(chan *services.ProposalHashList),

		log: nodeLogger,
	}
}
