package nginxplus

import (
	log "github.com/sirupsen/logrus"
	"github.com/docker/docker/client"
	"github.com/docker/docker/api/types"
	"bitbucket.es.ad.adp.com/projects/PC/repos/docker-gateway-project/config"
	dgwclient "bitbucket.es.ad.adp.com/projects/PC/repos/docker-gateway-project/client"
	"github.com/docker/docker/api/types/filters"
	"os/exec"
	"strconv"
	"bytes"
	"archive/tar"
	"context"
	"github.com/pkg/errors"
)

type NginxPlusLBMgr struct {

	client *client.Client
	consulAddr *config.ConsulAddr
	nginxplusTemplate string
	consulTemplateLoc string
	containers []types.Container
	nginxplusCfg *config.NginxPlusConfig
}

func NewLoadBalancer(cfg *config.Config, client *client.Client) (*NginxPlusLBMgr, error) {

	consulTmpl := cfg.ConsulTemplate
	consulAddr := &config.ConsulAddr{cfg.ConsulHost,cfg.ConsulPort}
	log.WithFields(log.Fields{"package": "nginxplus","function": "NewNginxPlusLoadBalancer",}).Debugf("Creating new nginxplus load balance manager")
	log.WithFields(log.Fields{"package": "nginxplus","function": "NewNginxPlusLoadBalancer",}).Debugf("Consul template location to be used -> %s", consulTmpl)
	optFilters := filters.NewArgs()
	optFilters.Add("label", "dockergateway.router.type=nginxplus")

	containers, err := dgwclient.GetRunningContainerListByFilters(client, &optFilters)
	log.WithFields(log.Fields{"package": "nginxplus","function": "NewNginxPlusLoadBalancer",}).Debugf("Total of (%d) Docker gateway nginx plus container detected containerList: %s", len(containers), containers)
	if err != nil {
		log.WithFields(log.Fields{"package": "nginxplus","function": "NewNginxPlusLoadBalancer",}).Errorf("Error while attempting to detect docker gateway nginx plus containers err: %s", err)
		return nil,err

	} else {
		nginxPlusmgr := NginxPlusLBMgr{
			containers: containers,
			client: client,
			consulAddr: consulAddr,
			consulTemplateLoc: consulTmpl,
			nginxplusCfg: cfg.NginxPlusconfig,
		}
		return  &nginxPlusmgr,nil
	}
}

func (n *NginxPlusLBMgr) GenerateConfigFile() ([]byte, error) {

  templateLoc := n.nginxplusCfg.TemplatePath
  log.WithFields(log.Fields{"package": "nginxplus","function": "GenerateConfigFile",}).Debugf("ConsulTemplate Command: %s", n.consulTemplateLoc, "-consul-addr="+n.consulAddr.Host+":"+strconv.Itoa(n.consulAddr.Port),"-dry", "-once", "-template="+templateLoc)
  out, err := exec.Command(n.consulTemplateLoc, "-consul-addr="+n.consulAddr.Host+":"+strconv.Itoa(n.consulAddr.Port),"-dry", "-once", "-template="+templateLoc).Output()
  if err != nil {
	  log.WithFields(log.Fields{"package": "nginxplus","function": "GenerateConfigFile",}).Errorf("Error while executing consul template command err: %s", err)
  	  return nil, err
  }
  return out[3:], nil  //:3 is required since consul template to stdout adds a '>'
}

func saveConfig(nConf []byte, n *NginxPlusLBMgr, loc string) error {

	log.WithFields(log.Fields{"package": "nginxplus","function": "saveConfig",}).Debugf("save request to loc: %s", loc)

	for _,container := range n.containers {
		buf := new(bytes.Buffer)
		tw := tar.NewWriter(buf)
		hdr := &tar.Header{
			Name: loc,
			Mode: 0644,
			Size: int64(len(nConf)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			log.WithFields(log.Fields{"package": "nginxplus","function": "saveConfig",}).Errorf("Error writing proxy config header err: %s", err)
			return err
		}

		if _, err := tw.Write(nConf); err != nil {
			log.WithFields(log.Fields{"package": "nginxplus","function": "saveConfig",}).Errorf("Error writing proxy config err: %s", err)
			return err
		}

		if err := tw.Close(); err != nil {
			log.WithFields(log.Fields{"package": "nginxplus","function": "saveConfig",}).Errorf("Error closing tar writer config err: %s", err)
			return err
		}

		opts := types.CopyToContainerOptions{
			AllowOverwriteDirWithFile: true,
		}
		log.WithFields(log.Fields{"package": "nginxplus","function": "saveConfig",}).Debugf("Attempting to save nginx conf.validateFailed to location: %s inside the container: %s", loc,container.ID[:12])
		//n.client.CopyToContainer(context.Background(), "ecc9130a63a41f0ac6096be3dbe79a6b59661fdfc8a6bc105f0a37a3acc24f97", "/" , buf, opts)
		if err :=  n.client.CopyToContainer(context.Background(), container.ID, "/" , buf, opts); err != nil {
			log.WithFields(log.Fields{"package": "nginxplus","function": "saveConfig",}).Errorf("Error saving nginx conf for container= %s err: %s", container.ID[:12], err)
			return  err
		}else{
			log.WithFields(log.Fields{"package": "nginxplus","function": "saveConfig",}).Debugf("Successfully saved nginx conf.validateFailed to location: %s inside the container: %s", loc,container.ID[:12])
		}
	}
	return nil
}

func (n *NginxPlusLBMgr) SaveAndValidateConfig(nConf []byte) error {

	iterCountChan := make(chan int)

	var errorValidatingConf error
	tmpLocation := "/etc/nginx/nginx.conf.validateFailed"
	origLocation := "/etc/nginx/nginx.conf"
	err := saveConfig(nConf, n, tmpLocation)

	if err == nil {
		i := 0
		log.WithFields(log.Fields{"package": "nginxplus","function": "SaveAndValidateConfig",}).Debugf("Successfully saved nginx conf to %s", tmpLocation)
		log.WithFields(log.Fields{"package": "nginxplus","function": "SaveAndValidateConfig",}).Debugf("Attempting to validate the nginx configuration file that was just saved")
		for _,cnt := range n.containers {
			go func(c types.Container) {
				out, rc, err := dgwclient.DockerExecHelper(n.client,[]string{
					"nginx",
					"-t",
					"-c",
					tmpLocation,
				} , c)
				if err != nil || rc != 0{
					log.WithFields(log.Fields{"package": "nginxplus","function": "SaveAndValidateConfig",}).Errorf("%s -> Error while validating nginx conf. returnCode: %s, err: %s", c.ID[:12], rc, err)
					log.WithFields(log.Fields{"package": "nginxplus","function": "SaveAndValidateConfig",}).Errorf("%s -> Validation output: %s",c.ID[:12], out)
					errorValidatingConf = errors.New("Error while performing SaveAndValidateConfig")
					i++
					iterCountChan <- i
					log.WithFields(log.Fields{"package": "nginxplus","function": "SaveAndValidateConfig",}).Debugf("%s -> Iteration count i: %d",c.ID[:12], i)
				} else {
					log.WithFields(log.Fields{"package": "nginxplus","function": "SaveAndValidateConfig",}).Debugf("%s -> No error found err: %s | returncode: %d",c.ID[:12],err, rc)
					log.WithFields(log.Fields{"package": "nginxplus","function": "SaveAndValidateConfig",}).Debugf("%s -> Validation successful! output: %s",c.ID[:12], out)
					out1, rc1, err1 := dgwclient.DockerExecHelper(n.client,[]string{
						"mv",
						tmpLocation,
						origLocation,
					} , c)
					if err1 != nil || rc1 != 0{
						log.WithFields(log.Fields{"package": "nginxplus","function": "SaveAndValidateConfig",}).Errorf("%s -> Error while renaming nginx conf back to %s. returnCode: %s, err: %s",c.ID[:12],origLocation, rc1, err1)
						log.WithFields(log.Fields{"package": "nginxplus","function": "SaveAndValidateConfig",}).Errorf("%s -> Command Output %s",c.ID[:12], out1)
						errorValidatingConf = errors.New("Error while performing SaveAndValidateConfig")
					}
					log.WithFields(log.Fields{"package": "nginxplus","function": "SaveAndValidateConfig",}).Debugf("%s -> No error found err: %s | returncode: %d",c.ID[:12],err1, rc1)
					log.WithFields(log.Fields{"package": "nginxplus","function": "SaveAndValidateConfig",}).Debugf("%s -> Renaming successful! Output: %s",c.ID[:12], out1)
					i++
					log.WithFields(log.Fields{"package": "nginxplus","function": "SaveAndValidateConfig",}).Debugf("%s -> Iteration count i: %d",c.ID[:12], i)
					iterCountChan <- i
				}
			}(cnt)
		}
		l := len(n.containers)
		for j := range iterCountChan {
			if j == l{
				log.WithFields(log.Fields{"package": "nginxplus","function": "SaveAndValidateConfig",}).Debugf("Iteration complete. Closing iterCountChan channel j:%s  l=%s ", j, l)
				close(iterCountChan)
				return  errorValidatingConf
				break
			}
		}
	}else {
		log.WithFields(log.Fields{"package": "nginxplus","function": "SaveAndValidateConfig",}).Errorf("Error while saving nginx conf to 1 or more containers err: %s", err)
		return err
	}

	return nil
}

func (n  *NginxPlusLBMgr) ReloadConfig() error  {

	var errorReloadingConf error = nil
	for _,container := range n.containers {
		if err := n.client.ContainerKill(context.Background(), container.ID, "HUP"); err != nil {
			log.WithFields(log.Fields{"package": "nginxplus","function": "ReloadConfig",}).Errorf("Error while sending reload (HUP) signal container: %s err: %s",container.ID[:12], err)
			errorReloadingConf = err
			continue
		}else{
			log.WithFields(log.Fields{"package": "nginxplus","function": "ReloadConfig",}).Debugf("Successfully sent reload (HUP) signal container: %s",container.ID[:12])
		}
	}
	return errorReloadingConf

}
