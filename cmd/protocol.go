package cmd

import (
	"fmt"
	"os"
	"runtime/pprof"
	"sync"

	"../../src/protocols/schultz"
	"github.com/docopt/docopt-go"
	"github.com/ncw/gmp"
)

func main() {
	usage := `MPSS Protocol simulation (local).

Usage:
  protocol --config=<cfg> [options]

Options:
  -h --help     		Show this screen.
  --version     		Show version.
  -c, --config=<cfg>  	Path to the configuration file.
  --round=<round>  		set the maxEpoch [default: 1].
  --logdir=<dir>  		set the maxEpoch [default: .].
  -v, --verbose  		Verbose output [default: false].
  --debug  				Super verbose output [default: false].
`

	arguments, err := docopt.ParseDoc(usage)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	var cmdOpt CmdOpt
	err = arguments.Bind(&cmdOpt)
	if err != nil {
		panic(err.Error())
	}

	logger, pp, systemConfig, nodeIPList, secretSharePoly := Init("protocol", cmdOpt)

	// profiling seti[
	f, err := os.Create("cpu_profiling.log")
	if err != nil {
		logger.Fatal(err)
	}
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()

	// build the primary
	primary := schultz.BuildBulletinBoard(logger, systemConfig.Primary.Url, nodeIPList, pp)

	// build all the nodes
	var nodes []schultz.Node

	for name, nodeConfig := range systemConfig.Peers {
		ip := nodeConfig.Url
		// get everyone else's IP in a list
		peerIPs := make(map[schultz.NewNodeID]string)
		for otherName, otherConfig := range systemConfig.Peers {
			if otherName == name {
				continue
			}

			peerIPs[schultz.NewNodeID(otherConfig.Id)] = otherConfig.Url
		}

		share := gmp.NewInt(0)
		secretSharePoly.EvalMod(gmp.NewInt(nodeConfig.Id), pp.GetPrime(), share)

		nodes = append(nodes, schultz.BuildNode(pp, logger, nodeConfig.Id, systemConfig.Primary.Url, ip, peerIPs, share))
	}

	for i := range nodes {
		logger.Infof("starting %d th node", i)
		go func(node schultz.Node) {
			node.Serve()
		}(nodes[i])
	}

	go primary.Serve()
	go primary.StartProtocol()

	// prevent the primary from getting killed
	// so we can collect some profiling
	primary.SetSuicideOption(false)

	for i := range nodes {
		if err := nodes[i].ConnectPrimary(); err != nil {
			logger.Fatalf("cannot connect to the primary")
		}
	}

	// must use epoch zero to kick off the protocol
	for i := range nodes {
		go nodes[i].SubmitShareToPrimary(0)
	}

	var waitGoRoutines sync.WaitGroup
	waitGoRoutines.Add(len(nodes))
	logger.Infof("%d added to the waiting group", len(nodes))

	for i := range nodes {
		go nodes[i].StartProtocol(&waitGoRoutines, schultz.Epoch(cmdOpt.Round))
	}

	waitGoRoutines.Wait()
}
