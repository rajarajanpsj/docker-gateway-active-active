package client

import (
	"github.com/docker/docker/client"
	"github.com/docker/docker/api/types/filters"
	log "github.com/sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"strings"
	"context"
)


func  GetContainerList(engineClient *client.Client, options types.ContainerListOptions) ([]types.Container, error){

	return  engineClient.ContainerList(context.Background(), options)
}

func GetRunningContainerListByFilters(engineClient *client.Client, optFilters *filters.Args) ([]types.Container, error){

	optFilters.Add("status", "running")

	opts := types.ContainerListOptions{
		All:     false,
		Size:    false,
		Filters: *optFilters,
	}

	return GetContainerList(engineClient,opts)
}

func GetDGWActiveActiveContainers(engineClient *client.Client) ([]types.Container, error){

	optFilters := filters.NewArgs()
	optFilters.Add("status", "running")
	optFilters.Add("label", "dockergateway.activeactive=true")

	return  GetRunningContainerListByFilters(engineClient, &optFilters)
}

func DockerExecHelper(engineClient *client.Client, cmd []string, container types.Container) (string, int, error){

   var execOutput string
   log.WithFields(log.Fields{"package": "client","function": "DockerExecHelper",}).Debugf("Docker exec request  cmd: %s  container= %s", strings.Join(cmd," "), container.ID[:12])
   resp, err := engineClient.ContainerExecCreate(context.Background(), container.ID, types.ExecConfig{
		   User: "root",
		   Cmd: cmd,
		   Detach:       false,
		   AttachStdout: true,
		   AttachStderr: true,
	   })
   if err != nil {
	   log.WithFields(log.Fields{"package": "client","function": "DockerExecHelper",}).Errorf("Error while calling ContainerExecCreate for container= %s  err: %s", container.ID[:12], err)
      	return "", 1, err
   }
	aResp, err := engineClient.ContainerExecAttach(context.Background(), resp.ID, types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		log.WithFields(log.Fields{"package": "client","function": "DockerExecHelper",}).Errorf("Error while calling ContainerExecAttach for container= %s  err: %s", container.ID[:12], err)
		return "", 1, err
	}
	defer aResp.Conn.Close()
	res, err := WaitForExec(engineClient,resp.ID)
	if err != nil {
		log.WithFields(log.Fields{"package": "client","function": "DockerExecHelper",}).Errorf("Error while Exec attach waiting to finish for container= %s  err: %s", container.ID[:12], err)
		return "", 1, err
	}

	if res.ExitCode != 0 {
		log.WithFields(log.Fields{"package": "client","function": "DockerExecHelper",}).Errorf("Docker exec non-zero return code  res.ExitCode: %s container= %s", res.ExitCode,container.ID[:12])
		execOutput, err := aResp.Reader.ReadString('\n')
		log.WithFields(log.Fields{"package": "client","function": "DockerExecHelper",}).Errorf("Docker exec output  execOutput: %s err: %s  container= %s", execOutput, err,container.ID[:12])
		return strings.TrimSpace(execOutput), 1, err
	}

	return strings.TrimSpace(execOutput), 0, nil
}

func WaitForExec(engineClient *client.Client, execID string) (types.ContainerExecInspect, error) {

	var res types.ContainerExecInspect
	for {
		r, err := engineClient.ContainerExecInspect(context.Background(), execID)
		if err != nil {
			return res, err
		}

		if !r.Running {
			res = r
			break
		}
	}

	return res, nil

}