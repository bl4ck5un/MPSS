package Schultz

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"os"
	"strconv"

	"../../utils/interpolation"
	"github.com/ncw/gmp"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)
import "./services"

type BulletinBoard struct {
	config PublicParameter

	proposalHashChan chan *services.ProposalHash
	proposalHashFull chan struct{}
	shareChan        chan *services.Share
	killChan         chan struct{}

	myIP       string
	peerIPList []string
	nodes      []services.NodeClient

	// logging
	log *logrus.Entry

	// hack
	allowSuicide bool
}

func (bb *BulletinBoard) SubmitProposalHash(ctx context.Context, hash *services.ProposalHash) (*services.Empty, error) {
	select {
	case bb.proposalHashChan <- hash:
	case <-bb.proposalHashFull:
	}
	return &services.Empty{}, nil
}

func (bb *BulletinBoard) consensusOnProposalHash(epoch Epoch) {
	// just need 2t+1 proposals
	proposalHash := make([]*services.ProposalHash, 2*bb.config.degree+1)

	i := 0
	for {
		hashMsg := <-bb.proposalHashChan

		if Epoch(hashMsg.Epoch) < epoch {
			bb.log.Infof("[primary] ignoring hashMsg from a previous epoch: %d (at epoch %d)", hashMsg.Epoch, epoch)
			continue
		} else if Epoch(hashMsg.Epoch) > epoch {
			panic("this can not happen in out setting.")
		}

		if len(hashMsg.Hash) != sha256.Size {
			panic(fmt.Sprintf("wrong size: Wanted %d. Got %d", sha256.Size, len(hashMsg.Hash)))
		}

		bb.log.Debugf("[primary] receiving hash from %d", hashMsg.Proposer)
		proposalHash[i] = hashMsg

		i += 1
		bb.log.Debugf("[primary] %d hash received", i)
		if i >= len(proposalHash) {
			break
		}
	}

	// close to notify the upstream
	close(bb.proposalHashFull)

	bb.log.Info("primary enough hashes received")

	// send out the list to all nodes
	ctx := context.Background()
	for _, node := range bb.nodes {
		msg := services.ProposalHashList{
			Epoch: int32(epoch),
			List:  proposalHash,
		}
		_, err := node.StartCheckingProposals(ctx, &msg)
		if err != nil {
			bb.log.Fatalf(err.Error())
		}
	}
}

func (bb *BulletinBoard) AssembleShare(ctx context.Context, in *services.Share) (*services.Empty, error) {
	tmp := gmp.NewInt(0)
	tmp.SetBytes(in.Share)
	bb.log.Debugf("from=%d, share=%s", in.From, tmp.String())

	bb.shareChan <- in

	return &services.Empty{}, nil
}

func (bb *BulletinBoard) assembleSecret(epoch Epoch) {
	degree := bb.config.degree
	prime := bb.config.prime

	Xs := make([]*gmp.Int, len(bb.config.newGroup))
	Ys := make([]*gmp.Int, len(bb.config.newGroup))

	for i := range Xs {
		Xs[i] = gmp.NewInt(0)
		Ys[i] = gmp.NewInt(0)
	}

	i := 0
	for {
		share := <-bb.shareChan

		if Epoch(share.Epoch) < epoch {
			bb.log.Infof("[primary] ignoring share from a previous epoch: %d (at epoch %d)", share.Epoch, epoch)
			continue
		} else if Epoch(share.Epoch) > epoch {
			panic("this can not happen in out setting.")
		}

		bb.log.Debugf("worker gets a share")
		Xs[i].SetInt64(int64(share.From))
		Ys[i].SetBytes(share.Share)

		i += 1
		if i >= len(Xs) {
			break
		}
	}

	poly, err := interpolation.LagrangeInterpolate(degree, Xs, Ys, prime)
	if err != nil {
		panic("can't recover the secret")
	}

	secret := gmp.NewInt(0)
	poly.EvalMod(gmp.NewInt(0), prime, secret)

	bb.log.WithField("secret", secret.String()).Warnf("finishing epoch %d", epoch)

	// connect to all nodes if firstRun is true
	if epoch == 0 {
		bb.log.Debugf("connect to all peers")
		bb.ConnectToPeers()
	}

	// notify nodes to advance the epoch
	ctx := context.Background()
	for i := range bb.nodes {
		go func(dst int) {
			_, err := bb.nodes[dst].AdvanceEpoch(ctx, &services.Empty{})
			if err != nil {
				bb.log.Fatalf(err.Error())
			}
		}(i)
	}
}

func (bb *BulletinBoard) Kill(ctx context.Context, empty *services.Empty) (*services.Empty, error) {
	bb.killChan <- struct{}{}

	return &services.Empty{}, nil
}

func (bb *BulletinBoard) suicide() {
	i := 0
	for {
		<-bb.killChan
		i += 1

		if bb.allowSuicide && i >= len(bb.config.oldGroup) {
			bb.log.Infof("killing myself...")
			os.Exit(0)
		}
	}
}

func (bb *BulletinBoard) ConnectToPeers() {
	for _, peer := range bb.peerIPList {
		conn, err := grpc.Dial(peer, grpc.WithInsecure())
		if err != nil {
			bb.log.Fatalf("cannot connect to: %v", err)
		}

		bb.log.Debugf("primary connect to %s", peer)
		bb.nodes = append(bb.nodes, services.NewNodeClient(conn))
	}
}

func (bb *BulletinBoard) Serve() {
	_, port, err := net.SplitHostPort(bb.myIP)
	if err != nil {
		bb.log.Fatalf("can't serve: %s", err.Error())
	}
	portUint, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		panic(err.Error())
	}

	hostPort := fmt.Sprintf("0.0.0.0:%d", portUint)

	lis, err := net.Listen("tcp4", hostPort)
	if err != nil {
		bb.log.Fatalf("cannot listen to %s, %v", hostPort, err)
	}

	s := grpc.NewServer()
	services.RegisterBulletinBoardServiceServer(s, bb)

	bb.log.Infof("primary serving on %s", hostPort)
	if err := s.Serve(lis); err != nil {
		bb.log.Fatalf("can't serve")
	}
}

func (bb *BulletinBoard) StartProtocol() {
	// prepare to suicide
	go bb.suicide()

	// always running
	epoch := Epoch(0)
	// HACK: wait to receive initial shares from everyone and start the protocol.
	bb.assembleSecret(epoch)

	for {
		epoch += 1
		bb.log.Warnf("primary entering epoch %d", epoch)

		// blocks
		bb.consensusOnProposalHash(epoch)
		bb.assembleSecret(epoch)

		// restore the notificator
		bb.proposalHashFull = make(chan struct{})
	}
}

func (bb *BulletinBoard) SetSuicideOption(opt bool) {
	bb.allowSuicide = opt
}

func BuildBulletinBoard(logger *logrus.Logger, myIP string, nodesIPList []string, cryptoConfig PublicParameter) BulletinBoard {
	logEntry := logger.WithFields(
		logrus.Fields{
			"name": "primary",
		})

	return BulletinBoard{
		config:     cryptoConfig,
		myIP:       myIP,
		peerIPList: nodesIPList,

		shareChan: make(chan *services.Share),
		killChan:  make(chan struct{}),

		proposalHashChan: make(chan *services.ProposalHash),
		proposalHashFull: make(chan struct{}),

		log:          logEntry,
		allowSuicide: true,
	}
}
