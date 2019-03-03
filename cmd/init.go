package cmd

import (
	"fmt"
	"math/rand"
	"path"

	"../../src/protocols/schultz"
	polycommit "../../src/utils/polycommit/pbc"
	"../../src/utils/polyring"
	"github.com/rifflock/lfshook"
	"github.com/sirupsen/logrus"
)

var Log *logrus.Logger

func NewLogger(nodeName string, logDir string) *logrus.Logger {
	if Log != nil {
		return Log
	}

	pathMap := lfshook.PathMap{
		logrus.WarnLevel:  path.Join(logDir, fmt.Sprintf("%s-benchmark.log", nodeName)),
		logrus.DebugLevel: path.Join(logDir, fmt.Sprintf("%s-debug.log", nodeName)),
		logrus.InfoLevel:  path.Join(logDir, fmt.Sprintf("%s-debug.log", nodeName)),
		logrus.ErrorLevel: path.Join(logDir, fmt.Sprintf("%s-error.log", nodeName)),
		logrus.FatalLevel: path.Join(logDir, fmt.Sprintf("%s-fatal.log", nodeName)),
	}

	Log = logrus.New()
	Log.Hooks.Add(lfshook.NewHook(
		pathMap,
		&logrus.JSONFormatter{},
	))
	return Log
}

type CmdOpt struct {
	Config  string
	Verbose bool
	Debug   bool
	Round   int32
	LogDir  string `docopt:"--logdir"`
	Id      string // ignored by the primary
}

func Init(nodeName string, opt CmdOpt) (*logrus.Logger, schultz.PublicParameter, schultz.SystemConfig, []string, polyring.Polynomial) {
	systemConfig, err := schultz.ParseConfigFile(opt.Config)
	if err != nil {
		panic(err.Error())
	}

	logger := NewLogger(nodeName, opt.LogDir)

	// only output warnings by default
	logger.SetLevel(logrus.WarnLevel)
	if opt.Verbose {
		logger.SetLevel(logrus.InfoLevel)
	}
	if opt.Debug {
		logger.SetLevel(logrus.DebugLevel)
	}

	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		ForceColors:   true,
	})

	var nodeIdList []int64
	for _, cf := range systemConfig.Peers {
		nodeIdList = append(nodeIdList, cf.Id)
	}

	var nodeIPList []string
	for _, cf := range systemConfig.Peers {
		nodeIPList = append(nodeIPList, cf.Url)
	}

	if len(nodeIdList) < 3*systemConfig.Degree+1 {
		logger.Fatalf("N >= 3t+1 is required. N=%d, t=%d", len(nodeIdList), systemConfig.Degree)
	}

	pp := schultz.BuildConfig(
		systemConfig.Degree,
		polycommit.Curve.Ngmp,
		nodeIdList,
		nodeIdList,
	)

	// make sure all nodes start with the same polynomial
	rng := rand.New(rand.NewSource(0))
	secretSharePoly, err := polyring.NewRand(pp.GetDegree(), rng, pp.GetPrime())
	if err != nil {
		panic(err.Error())
	}

	// hard code the secret as 6666666666666666666666666
	secretSharePoly.GetPtrToConstant().SetString("6666666666666666666666666", 10)

	return logger, pp, systemConfig, nodeIPList, secretSharePoly
}
