package cmd

import (
	"fmt"
	"os"

	"../../src/protocols/schultz"
	"github.com/docopt/docopt-go"
)

func main() {
	usage := `Primary node in MPSS Protocol.

Usage:
  primary --config=<cfg> [options]

Options:
  -h --help     		Show this screen.
  --version     		Show version.
  -c, --config=<cfg>  	Path to the configuration file.
  --round=<round>  		set the maxEpoch [default: 1].
  --logdir=<dir>  		set the maxEpoch [default: .].
  -v, --verbose  		Verbose output [default: false].
  --debug  				Super verbose output [default: false].`

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

	logger, pp, systemConfig, nodeIPList, _ := Init("primary", cmdOpt)

	logger.Infof("using config file %s", cmdOpt.Config)

	// build the primary
	primary := schultz.BuildBulletinBoard(logger, systemConfig.Primary.Url, nodeIPList, pp)

	go primary.StartProtocol()

	// blocks
	primary.Serve()
}
