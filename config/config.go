package config

import (
)

type Config struct {

	DockerURL string
	TLSCACert string
	TLSCert string
	TLSKey string
	ActiveActiveSrvcsCnslPath string
	BuddyCluster string
	RemoteGateWayCnslPath string
	AllowInsecure bool
	PollInterval string
	ConsulHost string
	ConsulPort int
	ConsulTemplate string
	DefaultHealthChkURI string
	DefaultHealthChkInterval string
	DefaultHealthChkFails string
	DefaultHealthChkPasses string

	NginxPlusconfig *NginxPlusConfig
}

type  ConsulAddr struct {
	Host string
	Port int
}

type NginxPlusConfig struct {
	Name                      string
	ConfigPath                string
	PidPath                   string
	TemplatePath              string
	BackendOverrideAddress    string
	ConnectTimeout            int
	ServerTimeout             int
	ClientTimeout             int
	MaxConn                   int
	Port                      int
	SyslogAddr                string
	AdminUser                 string
	AdminPass                 string
	SSLCertPath               string
	SSLCert                   string
	SSLPort                   int
	SSLOpts                   string
	User                      string
	WorkerProcesses           int
	RLimitNoFile              int
	ProxyConnectTimeout       int
	ProxySendTimeout          int
	ProxyReadTimeout          int
	SendTimeout               int
	SSLCiphers                string
	SSLProtocols              string

}