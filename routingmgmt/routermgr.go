package routingmgmt

import (
	"bitbucket.es.ad.adp.com/projects/PC/repos/docker-gateway-project/config"
	"bitbucket.es.ad.adp.com/projects/PC/repos/docker-gateway-project/routingmgmt/nginxplus"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
)

type  RouterManager struct {

	lbs []*LBManager  //TODO use this to support multiple LBManager types including haproxy etc.

}

/*
type LBMgrConfig struct {
	client *client.Client
	consulAddr *config.ConsulAddr
	consulTmpl string
} */

type  LBManager interface {
	NewLoadBalancer(*config.Config, *client.Client) LBManager
	GenerateConfigFile() ([]byte, error)
	SaveAndValidateConfig(nConf []byte) error
	ReloadConfig() error
}

func UpdateRouter(cfg *config.Config, client *client.Client) error{

	log.WithFields(log.Fields{"package": "routingmgmt","function": "UpdateRouter",}).Debugf("Received an update request.UpdateRouter in action !!!!")
	ngxMgr, _ := nginxplus.NewLoadBalancer(cfg, client)

     s, _ := ngxMgr.GenerateConfigFile()
	 log.WithFields(log.Fields{"package": "routingmgmt","function": "UpdateRouter",}).Debugf("Printing consul template generate nginx conf: %s", string(s))

     err := ngxMgr.SaveAndValidateConfig(s)
     if err != nil {
		 log.WithFields(log.Fields{"package": "routingmgmt","function": "UpdateRouter",}).Errorf("Error while invoking SaveAndValidate err: %s", err)
     	return err
	 }else{
	 	return ngxMgr.ReloadConfig()
	 }

}