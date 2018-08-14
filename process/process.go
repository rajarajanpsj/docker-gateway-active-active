package process

import (
	"time"
	"sort"
	"encoding/json"
	"crypto/sha256"
	"encoding/hex"
	log "github.com/sirupsen/logrus"
	"cozysystems.net/projects/CloudNative/repos/docker-gateway-project/config"
	"cozysystems.net/projects/CloudNative/repos/docker-gateway-project/commonutils"
	dgwclient "cozysystems.net/projects/CloudNative/repos/docker-gateway-project/client"
	"cozysystems.net/projects/CloudNative/repos/docker-gateway-project/routingmgmt"
	"github.com/docker/docker/client"
	"github.com/docker/docker/api/types"
	consulapi  "github.com/hashicorp/consul/api"
	"fmt"
)

const (
	defaultPollingInterval = time.Millisecond * 15000
)

var (
	updateServiceRegistryChan chan []types.Container
	updateRouterMgr chan (bool)

)
type DGWProcess struct {
	cfg           *config.Config
	client        *client.Client
	servicesHash   string
	remoteDGWHash  string
	consul *consulapi.Client
	persistentActvActvSrvcs map[string] *consulapi.KVPair
}

type ActiveActiveService struct {

	Label_com_docker_compose_service string
	Label_com_docker_compose_project string
	Label_interlock_hostname string
	Label_interlock_domain string
	Label_health_check_uri string       `default:"/health"`
	Label_health_check_interval string  `default:"10"`
	Label_health_check_fails string     `default:"1"`
	Label_health_check_passes string    `default:"1"`
	Timestamp string
}

type RemoteDGWCfg struct {
	port string
}

func NewProcess(cfg *config.Config, engineClient *client.Client) (*DGWProcess, error){

	var cnsl *consulapi.Client
	if c, err := commonutils.GetConsulApiClient(cfg.ConsulHost, cfg.ConsulPort,); err !=nil{
		log.WithFields(log.Fields{"package": "process","function": "NewProcess",}).Error("Error while getting Consul api client!")
		return  nil, err
	}else {
		log.WithFields(log.Fields{"package": "process","function": "NewProcess",}).Debug("Successfully procured the Consul api client")
		cnsl = c
	}

	dgw := &DGWProcess{
		      cfg,
		      engineClient,
		      "",
		      "",
		      cnsl,
		      make(map[string]*consulapi.KVPair),

	}

	// Set up the Channel to look for change and trigger extensions

	updateServiceRegistryChan = make(chan []types.Container)
	updateRouterMgr = make(chan bool)

	go func() {
			log.WithFields(log.Fields{"package": "process","function": "NewProcess",}).Debug("Starting the updateServiceRegistryChan channel to receive updates and update config on nginx plus")

		    for b := range updateServiceRegistryChan {
				log.WithFields(log.Fields{"package": "process","function": "NewProcess",}).Debug("Update received on CHANNEL updateServiceRegistryChan")
				log.WithFields(log.Fields{"package": "process","function": "NewProcess",}).Debug("Attempting to trigger config reload on plugin extensions")

		    	for _, x := range b {
					s := ActiveActiveService {
						Label_com_docker_compose_service: x.Labels["com.docker.compose.service"],
						Label_com_docker_compose_project: x.Labels["com.docker.compose.project"],
						Label_interlock_hostname: x.Labels["interlock.hostname"],
						Label_interlock_domain: x.Labels["interlock.domain"],
						Label_health_check_uri: x.Labels["dockergateway.health_check_uri"],
						Label_health_check_interval: x.Labels["dockergateway.health_check_interval"],
						Label_health_check_fails: x.Labels["dockergateway.health_check_fails"],
						Label_health_check_passes: x.Labels["dockergateway.health_check_passes"],
						Timestamp: time.Now().Format(time.RFC850),
					}
					s.setDefaults(cfg)
					log.WithFields(log.Fields{"package": "process","function": "NewProcess",}).Debugf("Updating ActiveActiveService to Consul KV. Service")
					fmt.Println(s)
					b, err := json.Marshal(s)
					if err != nil {
						log.WithFields(log.Fields{"package": "process","function": "NewProcess",}).Errorf("Error marshalling service err: %s", err)
					}
					// saving to a map[string] so that we can always get the unique values i.e unique active active service list without any duplicates
					dgw.persistentActvActvSrvcs[cfg.ActiveActiveSrvcsCnslPath+"/"+s.Label_interlock_hostname+"."+s.Label_interlock_domain] =
						                        &consulapi.KVPair{Key: cfg.ActiveActiveSrvcsCnslPath+"/"+s.Label_interlock_hostname+"."+s.Label_interlock_domain, Value: b}
				}

				// retrieving the latest active active service list which will include the previous list of persistent services i.e even the services that
				// are stopped after the process came up.
				kvpair := make([]*consulapi.KVPair,0)
				for  _, value := range dgw.persistentActvActvSrvcs {
					kvpair = append(kvpair, value)
				}
				ok, err := commonutils.UpdateKVTreeConsul(cfg.ActiveActiveSrvcsCnslPath, kvpair,cnsl)

				if err != nil {
					log.WithFields(log.Fields{"package": "process","function": "NewProcess",}).Errorf("Error updating consul through batch txn err: %s  ok value is: %s", err, ok)

				} else {
					log.WithFields(log.Fields{"package": "process","function": "NewProcess",}).Debugf("Consul txn update ok value is -> %s", ok)
					log.WithFields(log.Fields{"package": "process","function": "NewProcess",}).Debug("Consul update is successful! Updating updateRouterMgr channel to trigger update on Router manager")
					updateRouterMgr <- true
				}
			}

	}()

	go func() {
		log.WithFields(log.Fields{"package": "process","function": "NewProcess",}).Debug("Starting the updateRouterMgr channel to receive updates")
		for range updateRouterMgr {
			log.WithFields(log.Fields{"package": "process","function": "NewProcess",}).Debug("Update received on CHANNEL updateRouterMgr")
			err := routingmgmt.UpdateRouter(dgw.cfg, dgw.client)
			if err != nil {
				log.WithFields(log.Fields{"package": "process","function": "NewProcess",}).Errorf("Error received while trying to Update Router err: %s", err)
				//Invalidate the Hash so that retry happens automatically
				dgw.servicesHash = ""
			}
		}
	}()

	return  dgw, nil
}

func (a *ActiveActiveService) setDefaults(cfg *config.Config) {

	if a.Label_health_check_uri == "" {
		a.Label_health_check_uri = cfg.DefaultHealthChkURI
	}

	if a.Label_health_check_interval == "" {
		a.Label_health_check_interval = cfg.DefaultHealthChkInterval
	}

	if a.Label_health_check_fails == "" {
		a.Label_health_check_fails = cfg.DefaultHealthChkFails
	}


	if a.Label_health_check_passes == "" {
		a.Label_health_check_passes = cfg.DefaultHealthChkPasses
	}
}

func (dgw *DGWProcess) runPoller(d time.Duration){

	t := time.NewTicker(d)

	go func() {
		log.WithFields(log.Fields{"package": "process","function": "runPoller",}).Debugf("Starting the polling routine. runPoller() -> %s", t)
		for range t.C {
			log.WithFields(log.Fields{"package": "process","function": "runPoller",}).Debug("Poll...tick...tock...tick...tock")
			log.WithFields(log.Fields{"package": "process","function": "runPoller",}).Debug("Retrieving active active containers from the swarm using label=dockergateway.activeactive=true")
			containers, err := dgwclient.GetDGWActiveActiveContainers(dgw.client)

			if err != nil {
				log.WithFields(log.Fields{"package": "process","function": "runPoller",}).Error("Error getting the active active container list from swarm, needs attention!")
				continue
			}

			projectServiceMap := make(map[string] bool)
			projectServiceStr := []string{}
			cntList := make([]types.Container, 0)

			for _, c := range containers {
				project := c.Labels["com.docker.compose.project"]
				service := c.Labels["com.docker.compose.service"]

				if _, ok := projectServiceMap[project+service]; !ok {
					projectServiceMap[project+service] = true
					projectServiceStr=append(projectServiceStr, project+service)
					cntList = append(cntList, c)
				}
			}

			sort.Strings(projectServiceStr)
			log.WithFields(log.Fields{"package": "process","function": "runPoller",}).Debugf("Active Active service/container list -> %s", projectServiceStr)
			pSData, err := json.Marshal(projectServiceStr)
			if err != nil {
				log.WithFields(log.Fields{"package": "process","function": "runPoller",}).Errorf("Unable to marshal the container list err: %s", err)
				continue
			}
			h := sha256.New()
			h.Write(pSData)
			calculatedHash := hex.EncodeToString(h.Sum(nil))

			if calculatedHash != dgw.servicesHash{
				log.WithFields(log.Fields{"package": "process","function": "runPoller",}).Debugf("Change detected in Service registry cntList : %s", cntList)
				log.WithFields(log.Fields{"package": "process","function": "runPoller",}).Debugf("Service Hash comparison. calculatedHash: %s		previousHash: %s", calculatedHash, dgw.servicesHash)
				dgw.servicesHash = calculatedHash
				updateServiceRegistryChan <- cntList
			} else if remoteChng := dgw.didRemoteDGWChange(); remoteChng {
				log.WithFields(log.Fields{"package": "process","function": "runPoller",}).Debug("Change detected in Remote Gateway remoteChng Value:%s",remoteChng)
				log.WithFields(log.Fields{"package": "process","function": "runPoller",}).Debugf("cntList value : %s", cntList)
				updateServiceRegistryChan <- cntList
			}else {
				log.WithFields(log.Fields{"package": "process","function": "runPoller",}).Debug("NO CHANGE DETECTED, sleeping ssshhhhhh.........!")
			}
		}

	}()
}

func (dgw *DGWProcess) didRemoteDGWChange() (bool){

	kv := dgw.consul.KV()
	remoteDGWstr := []string{}
	kvpairs,_,err := kv.List(dgw.cfg.RemoteGateWayCnslPath+"/",nil)

	if err == nil {
		for _, i := range kvpairs {
			remoteDGWstr = append(remoteDGWstr, i.Key)
		}
	}else {
		log.WithFields(log.Fields{"package": "process","function": "didRemoteDGWChange",}).Errorf("Error retrieving kvpairs for remoteDGW: err=%s", err)
		return  false
	}

	pSData, err := json.Marshal(remoteDGWstr)
	if err != nil {
		log.WithFields(log.Fields{"package": "process","function": "didRemoteDGWChange",}).Errorf("Unable to marshal containers for DGW change detection: err=%s", err)
		return false
	}
	h := sha256.New()
	h.Write(pSData)
	if x := hex.EncodeToString(h.Sum(nil)) ; x != dgw.remoteDGWHash{
		dgw.remoteDGWHash = x
		log.WithFields(log.Fields{"package": "process","function": "didRemoteDGWChange",}).Debugf("RemoteDGWChange Hash comparison. calculatedHash: %s	previousHash: %s", x, dgw.remoteDGWHash)
		log.WithFields(log.Fields{"package": "process","function": "didRemoteDGWChange",}).Debug("Remote DGW change detected, returning true")
		return true
	} else {
		log.WithFields(log.Fields{"package": "process","function": "didRemoteDGWChange",}).Debugf("RemoteDGWChange Hash comparison. calculatedHash: %s	previousHash: %s", x, dgw.remoteDGWHash)
		log.WithFields(log.Fields{"package": "process","function": "didRemoteDGWChange",}).Debug("No remote DGW change detected, returning false")
		return false
	}
}


func (dgw *DGWProcess) Run() (error) {

	usrPollInterval := dgw.cfg.PollInterval
	log.WithFields(log.Fields{"package": "process","function": "Run",}).Debugf("User supplied polling interval: PollInterval=%s",usrPollInterval)

	if usrPollInterval != ""{
		d, err := time.ParseDuration(usrPollInterval)
       		if err != nil {
				log.WithFields(log.Fields{"package": "process","function": "Run",}).Errorf("Error while parsing time duration with the value usrPollInterval=%s",usrPollInterval)
				return err
			}
		if d < defaultPollingInterval {
			log.WithFields(log.Fields{"package": "process","function": "Run",}).Debugf("User supplied polling interval is lesser than the default. default=%s", defaultPollingInterval)
			dgw.cfg.PollInterval = defaultPollingInterval.String()
			d = defaultPollingInterval
			log.WithFields(log.Fields{"package": "process","function": "Run",}).Debug("Changed user supplied interval to the default")
		}
		dgw.runPoller(d)
	}else {
		log.WithFields(log.Fields{"package": "process","function": "Run",}).Debugf("User supplied poll interval seems to be null, setting it to the default %s", defaultPollingInterval)
		dgw.runPoller(defaultPollingInterval)
	}
	return nil
}