package main

import (
	"flag"
	"os"
	log "github.com/sirupsen/logrus"
	"bitbucket.es.ad.adp.com/projects/PC/repos/docker-gateway-project/config"
	"bitbucket.es.ad.adp.com/projects/PC/repos/docker-gateway-project/client"
	"bitbucket.es.ad.adp.com/projects/PC/repos/docker-gateway-project/process"
	"io/ioutil"
	"github.com/BurntSushi/toml"
)

func main() {
	log.SetLevel(log.DebugLevel)
	//log.SetFormatter(&log.JSONFormatter{})

	configTomlFile := flag.String("config","USER_HAS_NOT_SUPPLIED_A_VALUE  Usage: docker-gateway-project -config=/home/whatever/config.toml","Config file supplied as input")
	flag.Parse()
	configPath := *configTomlFile

	log.WithFields(log.Fields{"package": "main","function": "main",}).Debugf("configTomlFile supplied by user -> %s", *configTomlFile)
	var data string

	if  configPath != "" && data == "" {
		log.WithFields(log.Fields{"package": "main","function": "main",}).Debugf("Loading DGW configuration: file=%s", configPath)
		d, err := ioutil.ReadFile(configPath)
		switch {
		case os.IsNotExist(err):
			log.WithFields(log.Fields{"package": "main","function": "main",}).Errorf("Configuration file not found!: file=%s", configPath)
		case err == nil:
			data = string(d)
			log.WithFields(log.Fields{"package": "main","function": "main",}).Debug("Sucessfully loaded the config file")
		default:
			log.WithFields(log.Fields{"package": "main","function": "main",}).Fatal(err)
		}
	}

	var cfg config.Config
	if _, err := toml.Decode(data, &cfg); err != nil {
	   log.WithFields(log.Fields{"package": "main","function": "main",}).Error("Error doing TOML parsing")
	   log.WithFields(log.Fields{"package": "main","function": "main",}).Fatal(err)
	}
	log.WithFields(log.Fields{"package": "main","function": "main",}).Debugf("DGW Config values: cfg=%s", cfg)
	log.WithFields(log.Fields{"package": "main","function": "main",}).Debugf("DGW Nginx plus config values: cfg.NginxPlusconfig=%s", cfg.NginxPlusconfig)
	log.WithFields(log.Fields{"package": "main","function": "main",}).Debugf("Attempting to get docker client for swarm:%s", cfg.DockerURL)

	engineClient, err := client.GetDockerClient(
		cfg.DockerURL,
		cfg.TLSCACert,
		cfg.TLSCert,
		cfg.TLSKey,
		cfg.AllowInsecure,
	)
	if err != nil {
		log.WithFields(log.Fields{"package": "main","function": "main",}).Debug("Error while getting docker client!")
		log.WithFields(log.Fields{"package": "main","function": "main",}).Fatal(err)
	}
	log.WithFields(log.Fields{"package": "main","function": "main",}).Debug("Creating a new DGW process to monitor and update DGW plugins like Nginx plus")
	dgw, err := process.NewProcess(&cfg, engineClient)

	if err != nil {
		log.WithFields(log.Fields{"package": "main","function": "main",}).Debug("Error while creating the DGW process, exiting...!")
		log.WithFields(log.Fields{"package": "main","function": "main",}).Fatal(err)
	}
	log.WithFields(log.Fields{"package": "main","function": "main",}).Debug("Starting the DGW process")
	error := dgw.Run()
	if error != nil {
		log.WithFields(log.Fields{"package": "main","function": "main",}).Debug("Error while starting the DGW process, exiting...!")
		log.WithFields(log.Fields{"package": "main","function": "main",}).Fatal(err)
	}
	select {}
}


