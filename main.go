package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strings"
	"syscall"

	log "github.com/golang/glog"
	"github.com/openshift/openshift-sdn/ovssubnet"
	"github.com/openshift/openshift-sdn/ovssubnet/api"
	"github.com/openshift/openshift-sdn/ovssubnet/registry"
)

type NetworkManager interface {
	StartMaster(sync bool, containerNetwork string, containerSubnetLength uint) error
	StartNode(sync, skipsetup bool) error
	Stop()
}

type CmdLineOpts struct {
	containerNetwork      string
	containerSubnetLength uint
	etcdEndpoints         string
	etcdPath              string
	etcdKeyfile           string
	etcdCertfile          string
	etcdCAFile            string
	ip                    string
	hostname              string
	nodePath              string
	master                bool
	node                  bool
	skipsetup             bool
	sync                  bool
	kube                  bool
	multitenant           bool
	help                  bool
	minionPath            string // Deprecated
	minion                bool   // Deprecated
}

var opts CmdLineOpts

func init() {
	flag.StringVar(&opts.containerNetwork, "container-network", "10.1.0.0/16", "container network")
	flag.UintVar(&opts.containerSubnetLength, "container-subnet-length", 8, "container subnet length")
	flag.StringVar(&opts.etcdEndpoints, "etcd-endpoints", "http://127.0.0.1:4001", "a comma-delimited list of etcd endpoints")
	flag.StringVar(&opts.etcdPath, "etcd-path", "/registry/sdn/", "etcd path")
	flag.StringVar(&opts.nodePath, "node-path", "/kubernetes.io/minions/", "etcd path that will be watched for node creation/deletion (Note: -sync flag will override this path with -etcd-path)")
	flag.StringVar(&opts.minionPath, "minion-path", "", "Deprecated, use -node-path instead")
	flag.StringVar(&opts.etcdKeyfile, "etcd-keyfile", "", "SSL key file used to secure etcd communication")
	flag.StringVar(&opts.etcdCertfile, "etcd-certfile", "", "SSL certification file used to secure etcd communication")
	flag.StringVar(&opts.etcdCAFile, "etcd-cafile", "", "SSL Certificate Authority file used to secure etcd communication")

	flag.StringVar(&opts.ip, "public-ip", "", "Publicly reachable IP address of this host (for node mode).")
	flag.StringVar(&opts.hostname, "hostname", "", "Hostname as registered with master (for node mode), will default to 'hostname -f'")

	flag.BoolVar(&opts.master, "master", true, "Run in master mode")
	flag.BoolVar(&opts.node, "node", false, "Run in node mode")
	flag.BoolVar(&opts.minion, "minion", false, "Deprecated, use -node instead")
	flag.BoolVar(&opts.skipsetup, "skip-setup", false, "Skip the setup when in node mode")
	flag.BoolVar(&opts.sync, "sync", false, "Sync the nodes directly to etcd-path (Do not wait for PaaS to do so!)")
	flag.BoolVar(&opts.kube, "kube", false, "Use kubernetes hooks for optimal integration with OVS. This option bypasses the Linux bridge. Any docker containers started manually (not through OpenShift/Kubernetes) will stay local and not connect to the SDN.")
	flag.BoolVar(&opts.multitenant, "multitenant", false, "Same as 'kube' but with multitenant capabilities. This option will only be examined if 'kube' option is 'false'.")

	flag.BoolVar(&opts.help, "help", false, "print this message")
}

func newNetworkManager() (NetworkManager, error) {
	sub, err := newSubnetRegistry()
	if err != nil {
		return nil, err
	}
	host := opts.hostname
	if host == "" {
		output, err := exec.Command("hostname", "-f").CombinedOutput()
		if err != nil {
			return nil, err
		}
		host = strings.TrimSpace(string(output))
	}

	if opts.kube {
		return ovssubnet.NewKubeController(sub, string(host), opts.ip, nil)
	} else {
		if opts.multitenant {
			return ovssubnet.NewMultitenantController(sub, string(host), opts.ip, nil)
		}
	}
	// default OVS controller
	return ovssubnet.NewDefaultController(sub, string(host), opts.ip, nil)
}

func newSubnetRegistry() (api.SubnetRegistry, error) {
	peers := strings.Split(opts.etcdEndpoints, ",")

	subnetPath := path.Join(opts.etcdPath, "subnets")
	subnetConfigPath := path.Join(opts.etcdPath, "config")

	var nodePath string
	if len(opts.minionPath) > 0 {
		nodePath = opts.minionPath
		log.Info("Warning: -minion-path deprecated, use -node-path")
	} else {
		nodePath = opts.nodePath
	}
	if opts.sync {
		nodePath = path.Join(opts.etcdPath, "minions")
	}

	cfg := &registry.EtcdConfig{
		Endpoints:        peers,
		Keyfile:          opts.etcdKeyfile,
		Certfile:         opts.etcdCertfile,
		CAFile:           opts.etcdCAFile,
		SubnetPath:       subnetPath,
		SubnetConfigPath: subnetConfigPath,
		NodePath:         nodePath,
	}

	return registry.NewEtcdSubnetRegistry(cfg)
}

func main() {
	// glog will log to tmp files by default. override so all entries
	// can flow into journald (if running under systemd)
	flag.Set("logtostderr", "true")

	// now parse command line args
	flag.Parse()

	if opts.help {
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTION]...\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(0)
	}

	// Register for SIGINT and SIGTERM and wait for one of them to arrive
	log.Info("Installing signal handlers")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	be, err := newNetworkManager()
	if err != nil {
		log.Fatalf("Failed to create new network manager: %v", err)
	}
	if opts.node || opts.minion {
		if opts.minion {
			log.Info("Warning: -minion deprecated, use -node")
		}
		err := be.StartNode(opts.sync, opts.skipsetup)
		if err != nil {
			log.Fatalf("Failed to start openshift sdn in node mode: %v", err)
		}
	} else if opts.master {
		err := be.StartMaster(opts.sync, opts.containerNetwork, opts.containerSubnetLength)
		if err != nil {
			log.Fatalf("Failed to start openshift sdn in master mode: %v", err)
		}
	}

	select {
	case <-sigs:
		// unregister to get default OS nuke behaviour in case we don't exit cleanly
		signal.Stop(sigs)

		log.Info("Exiting...")
		be.Stop()
	}
}
