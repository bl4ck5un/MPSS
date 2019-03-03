package cmd

import (
	"fmt"
	"os"
	"sync"

	"../../src/protocols/schultz"
	"github.com/docopt/docopt-go"
	"github.com/ncw/gmp"
)

func main() {
	usage := `Main node in MPSS Protocol.

Usage:
  node --config=<cfg> --id=<id> [options]

Options:
  -h --help     		Show this screen.
  --version     		Show version.
  -c, --config=<cfg>  	Path to the configuration file.
  --round=<round>  		set the maxEpoch [default: 1].
  --logdir=<dir>  		set the maxEpoch [default: ./log-node].
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

	logger, pp, systemConfig, _, secretSharePoly := Init(cmdOpt.Id, cmdOpt)

	myConfig := systemConfig.Peers[cmdOpt.Id]

	peerIPs := make(map[schultz.NewNodeID]string)
	for _, otherConfig := range systemConfig.Peers {
		if otherConfig.Id == myConfig.Id {
			continue
		}

		peerIPs[schultz.NewNodeID(otherConfig.Id)] = otherConfig.Url
	}

	share := gmp.NewInt(0)
	secretSharePoly.EvalMod(gmp.NewInt(myConfig.Id), pp.GetPrime(), share)

	logger.Infof("starting node %d", myConfig.Id)
	myNode := schultz.BuildNode(pp, logger, myConfig.Id, systemConfig.Primary.Url, myConfig.Url, peerIPs, share)

	go myNode.Serve()

	if err := myNode.ConnectPrimary(); err != nil {
		logger.Fatalf("cannot connect to the primary: %s", err.Error())
	}

	// must use epoch zero to kick off the protocol
	myNode.SubmitShareToPrimary(0)

	// start the main thread
	var waitGoRoutines sync.WaitGroup
	waitGoRoutines.Add(1)

	go myNode.StartProtocol(&waitGoRoutines, schultz.Epoch(cmdOpt.Round))

	waitGoRoutines.Wait()
}
