package server

import (
	"fmt"
	"net"
	_ "net/http/pprof"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"

	etcdclient "github.com/coreos/go-etcd/etcd"
	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/cmd/flagtypes"
	"github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/docker"
	"github.com/openshift/origin/pkg/cmd/util/variable"
)

// config is a struct that the command stores flag values into.
type Config struct {
	Docker *docker.Helper

	WriteConfigOnly bool

	StartNode   bool
	StartMaster bool
	StartKube   bool
	StartEtcd   bool

	MasterAddr     flagtypes.Addr
	BindAddr       flagtypes.Addr
	EtcdAddr       flagtypes.Addr
	KubernetesAddr flagtypes.Addr
	PortalNet      flagtypes.IPNet
	// addresses for external clients
	MasterPublicAddr     flagtypes.Addr
	KubernetesPublicAddr flagtypes.Addr

	ImageFormat         string
	LatestReleaseImages bool

	ImageTemplate variable.ImageTemplate

	Hostname  string
	VolumeDir string

	EtcdDir string

	CertDir string

	StorageVersion string

	NodeList flagtypes.StringList

	// ClientConfig is used when connecting to Kubernetes from the master, or
	// when connecting to the master from a detached node. If the server is an
	// all-in-one, this value is not used.
	ClientConfig clientcmd.ClientConfig

	CORSAllowedOrigins flagtypes.StringList
}

func NewDefaultConfig() *Config {
	hostname, err := defaultHostname()
	if err != nil {
		hostname = "localhost"
		glog.Warningf("Unable to lookup hostname, using %q: %v", hostname, err)
	}

	return &Config{
		Docker: docker.NewHelper(),

		MasterAddr:           flagtypes.Addr{Value: "localhost:8443", DefaultScheme: "https", DefaultPort: 8443, AllowPrefix: true}.Default(),
		BindAddr:             flagtypes.Addr{Value: "0.0.0.0:8443", DefaultScheme: "https", DefaultPort: 8443, AllowPrefix: true}.Default(),
		EtcdAddr:             flagtypes.Addr{Value: "0.0.0.0:4001", DefaultScheme: "http", DefaultPort: 4001}.Default(),
		KubernetesAddr:       flagtypes.Addr{DefaultScheme: "https", DefaultPort: 8443}.Default(),
		PortalNet:            flagtypes.DefaultIPNet("172.30.17.0/24"),
		MasterPublicAddr:     flagtypes.Addr{Value: "localhost:8443", DefaultScheme: "https", DefaultPort: 8443, AllowPrefix: true}.Default(),
		KubernetesPublicAddr: flagtypes.Addr{Value: "localhost:8443", DefaultScheme: "https", DefaultPort: 8443, AllowPrefix: true}.Default(),

		ImageTemplate: variable.NewDefaultImageTemplate(),

		Hostname: hostname,
		NodeList: flagtypes.StringList{"127.0.0.1"},
	}
}

// GetMasterAddress checks for an unset master address and then attempts to use the first
// public IPv4 non-loopback address registered on this host.
// TODO: make me IPv6 safe
func (cfg Config) GetMasterAddress() (*url.URL, error) {
	if cfg.MasterAddr.Provided {
		return cfg.MasterAddr.URL, nil
	}

	if cfg.StartMaster {
		// If the user specifies a bind address, and the master is not provided, use the bind port by default
		port := cfg.MasterAddr.Port
		if cfg.BindAddr.Provided {
			port = cfg.BindAddr.Port
		}

		// If the user specifies a bind address, and the master is not provided, use the bind scheme by default
		scheme := cfg.MasterAddr.URL.Scheme
		if cfg.BindAddr.Provided {
			scheme = cfg.BindAddr.URL.Scheme
		}

		// use the default ip address for the system
		addr, err := util.DefaultLocalIP4()
		if err != nil {
			return nil, fmt.Errorf("Unable to find the public address of this master: %v", err)
		}

		masterAddr := scheme + "://" + net.JoinHostPort(addr.String(), strconv.Itoa(port))
		return url.Parse(masterAddr)
	}

	// if we didn't specify and we aren't starting the master, read .kubeconfig to locate the master
	// TODO client config currently doesn't let you override the defaults
	// so it is defaulting to https://localhost:8443 for MasterAddr if
	// it isn't set by --master or --kubeconfig
	config, err := cfg.ClientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	return url.Parse(config.Host)
}

func (cfg Config) GetMasterPublicAddress() (*url.URL, error) {
	if cfg.MasterPublicAddr.Provided {
		return cfg.MasterPublicAddr.URL, nil
	}

	return cfg.GetMasterAddress()
}

func (cfg Config) GetEtcdAddress() (*url.URL, error) {
	if cfg.EtcdAddr.Provided {
		return cfg.EtcdAddr.URL, nil
	}

	// Etcd should be reachable on the same address that the master is (for simplicity)
	masterAddr, err := cfg.GetMasterAddress()
	if err != nil {
		return nil, err
	}

	etcdAddr := net.JoinHostPort(getHost(*masterAddr), strconv.Itoa(cfg.EtcdAddr.DefaultPort))
	return url.Parse("http://" + etcdAddr)
}

func (cfg Config) GetKubernetesAddress() (*url.URL, error) {
	if cfg.KubernetesAddr.Provided {
		return cfg.KubernetesAddr.URL, nil
	}

	return cfg.GetMasterAddress()
}

func (cfg Config) GetKubernetesPublicAddress() (*url.URL, error) {
	if cfg.KubernetesPublicAddr.Provided {
		return cfg.KubernetesPublicAddr.URL, nil
	}
	if cfg.KubernetesAddr.Provided {
		return cfg.KubernetesAddr.URL, nil
	}

	return cfg.GetMasterPublicAddress()
}

func (cfg Config) GetAssetPublicAddress() (*url.URL, error) {
	// Derive the asset public address by incrementing the master public address port by 1
	t, err := cfg.GetMasterPublicAddress()
	if err != nil {
		return nil, err
	}
	assetPublicAddr := *t
	assetPublicAddr.Host = net.JoinHostPort(getHost(assetPublicAddr), strconv.Itoa(getPort(assetPublicAddr)+1))

	return &assetPublicAddr, nil
}

func (cfg Config) GetAssetBindAddress() string {
	// Derive the asset bind address by incrementing the master bind address port by 1
	return net.JoinHostPort(cfg.BindAddr.Host, strconv.Itoa(cfg.BindAddr.Port+1))
}

func (cfg Config) GetNodeList() []string {
	nodeList := []string{}
	for _, curr := range cfg.NodeList {
		nodeList = append(nodeList, curr)
	}

	if len(nodeList) == 1 && nodeList[0] == "127.0.0.1" {
		nodeList[0] = cfg.Hostname
	}
	for i, s := range nodeList {
		s = strings.ToLower(s)
		nodeList[i] = s
	}

	return nodeList
}

// getEtcdClient creates an etcd client based on the provided config and waits
// until etcd server is reachable. It errors out and exits if the server cannot
// be reached for a certain amount of time.
func (cfg Config) getEtcdClient() (*etcdclient.Client, error) {
	address, err := cfg.GetEtcdAddress()
	if err != nil {
		return nil, err
	}
	etcdServers := []string{address.String()}
	etcdClient := etcdclient.NewClient(etcdServers)

	for i := 0; ; i++ {
		_, err := etcdClient.Get("/", false, false)
		if err == nil || tools.IsEtcdNotFound(err) {
			break
		}
		if i > 100 {
			return nil, fmt.Errorf("Could not reach etcd: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	return etcdClient, nil
}

// newOpenShiftEtcdHelper returns an EtcdHelper for the provided arguments or an error if the version
// is incorrect.
func (cfg Config) newOpenShiftEtcdHelper() (helper tools.EtcdHelper, err error) {
	// Connect and setup etcd interfaces
	client, err := cfg.getEtcdClient()
	if err != nil {
		return tools.EtcdHelper{}, err
	}

	version := cfg.StorageVersion
	if len(version) == 0 {
		version = latest.Version
	}
	interfaces, err := latest.InterfacesFor(version)
	if err != nil {
		return helper, err
	}
	return tools.EtcdHelper{client, interfaces.Codec, tools.RuntimeVersionAdapter{interfaces.MetadataAccessor}}, nil
}

// defaultHostname returns the default hostname for this system.
func defaultHostname() (string, error) {
	// Note: We use exec here instead of os.Hostname() because we
	// want the FQDN, and this is the easiest way to get it.
	fqdn, err := exec.Command("hostname", "-f").Output()
	if err != nil {
		return "", fmt.Errorf("Couldn't determine hostname: %v", err)
	}
	return strings.TrimSpace(string(fqdn)), nil
}
