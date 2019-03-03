package Schultz

import (
	"github.com/BurntSushi/toml"
	log "github.com/sirupsen/logrus"
)

type PrimaryConfig struct {
	Url string
}

type PeerConfig struct {
	Id  int64
	Url string
}

type SystemConfig struct {
	Degree  int
	Primary PrimaryConfig
	Peers   map[string]PeerConfig
}

func ParseConfigFile(tomlPath string) (SystemConfig, error) {
	config := SystemConfig{}
	md, err := toml.DecodeFile(tomlPath, &config)
	if err != nil {
		log.Fatal("Can't parse toml config", tomlPath, err.Error())
		return SystemConfig{}, err
	}

	if len(md.Undecoded()) > 0 {
		log.Fatal("configuration file not fully parsed")
	}

	return config, nil
}
