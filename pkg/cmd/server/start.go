package server

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet"
	kmaster "github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	etcdclient "github.com/coreos/go-etcd/etcd"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/flagtypes"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	"github.com/openshift/origin/pkg/cmd/server/kubernetes"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	"github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/docker"
)

const longCommandDesc = `
Start an OpenShift server

This command helps you launch an OpenShift server. The default mode is all-in-one, which allows
you to run all of the components of an OpenShift system on a server with Docker. Running

    $ openshift start

will start OpenShift listening on all interfaces, launch an etcd server to store persistent
data, and launch the Kubernetes system components. The server will run in the foreground until
you terminate the process.

Note: starting OpenShift without passing the --master address will attempt to find the IP
address that will be visible inside running Docker containers. This is not always successful,
so if you have problems tell OpenShift what public address it will be via --master=<ip>.

You may also pass an optional argument to the start command to start OpenShift in one of the
following roles:

    $ openshift start master --nodes host1,host2,host3,...

      Launches the server and control plane for OpenShift. You must pass a list of the node
      hostnames you want to watch (limitation).

    $ openshift start node --master masterIP

      Launches a new node and attempts to connect to the master on the provided IP.

You may also pass --etcd to connect to an external etcd server instead of running an integrated
instance.
`

// config is a struct that the command stores flag values into.
type config struct {
	Docker *docker.Helper

	MasterAddr     flagtypes.Addr
	BindAddr       flagtypes.Addr
	EtcdAddr       flagtypes.Addr
	KubernetesAddr flagtypes.Addr
	PortalNet      flagtypes.IPNet

	Hostname  string
	VolumeDir string

	EtcdDir string

	StorageVersion string

	NodeList flagtypes.StringList

	CORSAllowedOrigins    flagtypes.StringList
	RequireAuthentication bool
}

func NewCommandStartServer(name string) *cobra.Command {
	hostname, err := defaultHostname()
	if err != nil {
		hostname = "localhost"
		glog.Warningf("Unable to lookup hostname, using %q: %v", hostname, err)
	}

	cfg := &config{
		Docker: docker.NewHelper(),

		MasterAddr:     flagtypes.Addr{Value: "localhost:8080", DefaultScheme: "http", DefaultPort: 8080, AllowPrefix: true}.Default(),
		BindAddr:       flagtypes.Addr{Value: "0.0.0.0:8080", DefaultScheme: "http", DefaultPort: 8080, AllowPrefix: true}.Default(),
		EtcdAddr:       flagtypes.Addr{Value: "0.0.0.0:4001", DefaultScheme: "http", DefaultPort: 4001}.Default(),
		KubernetesAddr: flagtypes.Addr{DefaultScheme: "http", DefaultPort: 8080}.Default(),
		PortalNet:      flagtypes.DefaultIPNet("172.121.17.0/24"),

		Hostname: hostname,
		NodeList: flagtypes.StringList{"127.0.0.1"},
	}

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s [master|node]", name),
		Short: "Launch OpenShift",
		Long:  longCommandDesc,
		Run: func(c *cobra.Command, args []string) {
			if err := start(cfg, args); err != nil {
				glog.Fatal(err)
			}
		},
	}

	flag := cmd.Flags()

	flag.Var(&cfg.BindAddr, "listen", "The address to listen for connections on (host, host:port, or URL).")
	flag.Var(&cfg.MasterAddr, "master", "The address the master can be reached on (host, host:port, or URL).")
	flag.Var(&cfg.EtcdAddr, "etcd", "The address of the etcd server (host, host:port, or URL).")
	flag.Var(&cfg.KubernetesAddr, "kubernetes", "The address of the Kubernetes server (host, host:port, or URL). If specified no Kubernetes components will be started.")
	flag.Var(&cfg.PortalNet, "portal-net", "A CIDR notation IP range from which to assign portal IPs. This must not overlap with any IP ranges assigned to nodes for pods.")

	flag.StringVar(&cfg.VolumeDir, "volume-dir", "openshift.local.volumes", "The volume storage directory.")
	flag.StringVar(&cfg.EtcdDir, "etcd-dir", "openshift.local.etcd", "The etcd data directory.")

	flag.StringVar(&cfg.Hostname, "hostname", cfg.Hostname, "The hostname to identify this node with the master.")
	flag.Var(&cfg.NodeList, "nodes", "The hostnames of each node. This currently must be specified up front. Comma delimited list")
	flag.Var(&cfg.CORSAllowedOrigins, "cors-allowed-origins", "List of allowed origins for CORS, comma separated.  An allowed origin can be a regular expression to support subdomain matching.  If this list is empty CORS will not be enabled.")
	flag.BoolVar(&cfg.RequireAuthentication, "require-authentication", false, "Require authentication token for API access.")

	cfg.Docker.InstallFlags(flag)

	return cmd
}

// run launches the appropriate startup modes or returns an error.
func start(cfg *config, args []string) error {
	if len(args) > 1 {
		return errors.New("You may start an OpenShift all-in-one server with no arguments, or pass 'master' or 'node' to run in that role.")
	}

	var startEtcd, startNode, startMaster bool
	if len(args) == 1 {
		switch args[0] {
		case "master":
			startMaster = true
			startEtcd = !cfg.EtcdAddr.Provided
			if err := defaultMasterAddress(cfg); err != nil {
				return err
			}
			glog.Infof("Starting an OpenShift master, reachable at %s (etcd: %s)", cfg.MasterAddr.String(), cfg.EtcdAddr.String())

		case "node":
			startNode = true
			if err := defaultMasterAddress(cfg); err != nil {
				return err
			}
			glog.Infof("Starting an OpenShift node, connecting to %s (etcd: %s)", cfg.MasterAddr.String(), cfg.EtcdAddr.String())

		default:
			return errors.New("You may start an OpenShift all-in-one server with no arguments, or pass 'master' or 'node' to run in that role.")
		}

	} else {
		startMaster = true
		startEtcd = !cfg.EtcdAddr.Provided
		startNode = true
		if err := defaultMasterAddress(cfg); err != nil {
			return err
		}

		glog.Infof("Starting an OpenShift all-in-one, reachable at %s (etcd: %s)", cfg.MasterAddr.String(), cfg.EtcdAddr.String())
	}

	startKube := !cfg.KubernetesAddr.Provided

	if startMaster {
		if len(cfg.NodeList) == 1 && cfg.NodeList[0] == "127.0.0.1" {
			cfg.NodeList[0] = cfg.Hostname
		}
		for _, s := range cfg.NodeList {
			glog.Infof("  Node: %s", s)
		}

		if startEtcd {
			etcdConfig := &etcd.Config{
				BindAddr:     cfg.BindAddr.Host,
				PeerBindAddr: cfg.BindAddr.Host,
				MasterAddr:   cfg.EtcdAddr.URL.Host,
				EtcdDir:      cfg.EtcdDir,
			}
			etcdConfig.Run()
		}

		// Connect and setup etcd interfaces
		etcdClient, err := getEtcdClient(cfg)
		if err != nil {
			return err
		}
		etcdHelper, err := origin.NewEtcdHelper(cfg.StorageVersion, etcdClient)
		if err != nil {
			return fmt.Errorf("Error setting up server storage: %v", err)
		}
		ketcdHelper, err := kmaster.NewEtcdHelper(etcdClient, klatest.Version)
		if err != nil {
			return fmt.Errorf("Error setting up Kubernetes server storage: %v", err)
		}

		assetAddr := net.JoinHostPort(cfg.MasterAddr.Host, strconv.Itoa(cfg.BindAddr.Port+1))

		osmaster := &origin.MasterConfig{
			BindAddr:              cfg.BindAddr.URL.Host,
			MasterAddr:            cfg.MasterAddr.URL.String(),
			AssetAddr:             assetAddr,
			EtcdHelper:            etcdHelper,
			RequireAuthentication: cfg.RequireAuthentication,
		}

		// pick an appropriate Kube client
		if startKube {
			osmaster.EnsureKubernetesClient()
		} else {
			kubeClient, err := kclient.New(&kclient.Config{Host: cfg.KubernetesAddr.URL.String(), Version: klatest.Version})
			if err != nil {
				return fmt.Errorf("Unable to configure Kubernetes client: %v", err)
			}
			osmaster.KubeClient = kubeClient
		}

		osmaster.EnsureOpenShiftClient()
		osmaster.EnsureCORSAllowedOrigins(cfg.CORSAllowedOrigins)

		auth := &origin.AuthConfig{
			MasterAddr:     cfg.MasterAddr.URL.String(),
			SessionSecrets: []string{"secret"},
			EtcdHelper:     etcdHelper,
		}

		if startKube {
			portalNet := net.IPNet(cfg.PortalNet)
			kmaster := &kubernetes.MasterConfig{
				MasterHost: cfg.MasterAddr.Host,
				MasterPort: cfg.MasterAddr.Port,
				NodeHosts:  cfg.NodeList,
				PortalNet:  &portalNet,
				EtcdHelper: ketcdHelper,
				KubeClient: osmaster.KubeClient,
			}
			kmaster.EnsurePortalFlags()

			osmaster.RunAPI(kmaster, auth)

			kmaster.RunScheduler()
			kmaster.RunReplicationController()
			kmaster.RunEndpointController()
			kmaster.RunMinionController()

		} else {
			osmaster.RunAPI(auth)
		}

		osmaster.RunAssetServer()
		osmaster.RunBuildController()
		osmaster.RunDeploymentController()
		osmaster.RunDeploymentConfigController()
		osmaster.RunDeploymentConfigChangeController()
		osmaster.RunDeploymentImageChangeTriggerController()
	}

	if startNode {
		etcdClient, err := getEtcdClient(cfg)
		if err != nil {
			return err
		}

		nodeConfig := &kubernetes.NodeConfig{
			BindHost:   cfg.BindAddr.Host,
			NodeHost:   cfg.Hostname,
			MasterHost: cfg.MasterAddr.URL.String(),

			VolumeDir: cfg.VolumeDir,

			NetworkContainerImage: env("KUBERNETES_NETWORK_CONTAINER_IMAGE", kubelet.NetworkContainerImage),

			EtcdClient: etcdClient,
		}

		nodeConfig.EnsureVolumeDir()
		nodeConfig.EnsureDocker(cfg.Docker)

		nodeConfig.RunProxy()
		nodeConfig.RunKubelet()
	}

	select {}

	return nil
}

// getEtcdClient creates an etcd client based on the provided config and waits
// until etcd server is reachable. It errors out and exits if the server cannot
// be reached for a certain amount of time.
func getEtcdClient(cfg *config) (*etcdclient.Client, error) {
	etcdServers := []string{cfg.EtcdAddr.URL.String()}
	etcdClient := etcdclient.NewClient(etcdServers)

	for i := 0; ; i += 1 {
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

// defaultMasterAddress checks for an unset master address and then attempts to use the first
// public IPv4 non-loopback address registered on this host. It will also update the
// EtcdAddr after if it was not provided.
// TODO: make me IPv6 safe
func defaultMasterAddress(cfg *config) error {
	if !cfg.MasterAddr.Provided {
		// If the user specifies a bind address, and the master is not provided, use
		// the bind port by default
		port := cfg.MasterAddr.Port
		if cfg.BindAddr.Provided {
			port = cfg.BindAddr.Port
		}

		// use the default ip address for the system
		addr, err := util.DefaultLocalIP4()
		if err != nil {
			return fmt.Errorf("Unable to find the public address of this master: %v", err)
		}

		masterAddr := net.JoinHostPort(addr.String(), strconv.Itoa(port))
		if err := cfg.MasterAddr.Set(masterAddr); err != nil {
			return fmt.Errorf("Unable to set public address of this master: %v", err)
		}

		// Prefer to use the MasterAddr for etcd because some clients still connect to it.
		if !cfg.EtcdAddr.Provided {
			etcdAddr := net.JoinHostPort(addr.String(), strconv.Itoa(cfg.EtcdAddr.DefaultPort))
			if err := cfg.EtcdAddr.Set(etcdAddr); err != nil {
				return fmt.Errorf("Unable to set public address of etcd: %v", err)
			}
		}
	} else if !cfg.EtcdAddr.Provided {
		// Etcd should be reachable on the same address that the master is (for simplicity)
		etcdAddr := net.JoinHostPort(cfg.MasterAddr.Host, strconv.Itoa(cfg.EtcdAddr.DefaultPort))
		if err := cfg.EtcdAddr.Set(etcdAddr); err != nil {
			return fmt.Errorf("Unable to set public address of etcd: %v", err)
		}
	}
	return nil
}

// env returns an environment variable or a default value if not specified.
func env(key string, defaultValue string) string {
	val := os.Getenv(key)
	if len(val) == 0 {
		return defaultValue
	}
	return val
}
