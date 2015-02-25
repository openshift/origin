package server

import (
	"errors"
	"fmt"
	_ "net/http/pprof"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
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

    $ openshift start master --nodes=<host1,host2,host3,...>

      Launches the server and control plane for OpenShift. You may pass a list of the node
      hostnames you want to use, or create nodes via the REST API or 'openshift kube'.

    $ openshift start node --master=<masterIP>

      Launches a new node and attempts to connect to the master on the provided IP.

You may also pass --etcd=<address> to connect to an external etcd server instead of running an
integrated instance, or --kubernetes=<addr> and --kubeconfig=<path> to connect to an existing
Kubernetes cluster.
`

// NewCommandStartServer provides a CLI handler for 'start' command
func NewCommandStartServer(name string) (*cobra.Command, *Config) {
	cfg := NewDefaultConfig()

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s [master|node]", name),
		Short: "Launch OpenShift",
		Long:  longCommandDesc,
		Run: func(c *cobra.Command, args []string) {
			if err := cfg.Validate(c.Flags().Args()); err != nil {
				glog.Fatal(err)
			}

			cfg.Complete(args)

			if err := start(*cfg, c.Flags().Args()); err != nil {
				glog.Fatal(err)
			}
		},
	}

	flag := cmd.Flags()

	flag.BoolVar(&cfg.WriteConfigOnly, "write-config-and-walk-away", false, "Indicates that the command should write the config that would be used to start openshift and do nothing else.  This is not yet implemented.")

	flag.Var(&cfg.BindAddr, "listen", "The address to listen for connections on (host, host:port, or URL).")
	flag.Var(&cfg.MasterAddr, "master", "The master address for use by OpenShift components (host, host:port, or URL). Scheme and port default to the --listen scheme and port.")
	flag.Var(&cfg.MasterPublicAddr, "public-master", "The master address for use by public clients, if different (host, host:port, or URL). Defaults to same as --master.")
	flag.Var(&cfg.EtcdAddr, "etcd", "The address of the etcd server (host, host:port, or URL). If specified, no built-in etcd will be started.")
	flag.Var(&cfg.KubernetesAddr, "kubernetes", "The address of the Kubernetes server (host, host:port, or URL). If specified, no Kubernetes components will be started.")
	flag.Var(&cfg.KubernetesPublicAddr, "public-kubernetes", "The Kubernetes server address for use by public clients, if different. (host, host:port, or URL). Defaults to same as --kubernetes.")
	flag.Var(&cfg.PortalNet, "portal-net", "A CIDR notation IP range from which to assign portal IPs. This must not overlap with any IP ranges assigned to nodes for pods.")

	flag.StringVar(&cfg.ImageTemplate.Format, "images", cfg.ImageTemplate.Format, "When fetching images used by the cluster for important components, use this format on both master and nodes. The latest release will be used by default.")
	flag.BoolVar(&cfg.ImageTemplate.Latest, "latest-images", cfg.ImageTemplate.Latest, "If true, attempt to use the latest images for the cluster instead of the latest release.")

	flag.StringVar(&cfg.VolumeDir, "volume-dir", "openshift.local.volumes", "The volume storage directory.")
	flag.StringVar(&cfg.EtcdDir, "etcd-dir", "openshift.local.etcd", "The etcd data directory.")
	flag.StringVar(&cfg.CertDir, "cert-dir", "openshift.local.certificates", "The certificate data directory.")

	flag.StringVar(&cfg.Hostname, "hostname", cfg.Hostname, "The hostname to identify this node with the master.")
	flag.Var(&cfg.NodeList, "nodes", "The hostnames of each node. This currently must be specified up front. Comma delimited list")
	flag.Var(&cfg.CORSAllowedOrigins, "cors-allowed-origins", "List of allowed origins for CORS, comma separated.  An allowed origin can be a regular expression to support subdomain matching.  CORS is enabled for localhost, 127.0.0.1, and the asset server by default.")

	cfg.ClientConfig = cmdutil.DefaultClientConfig(flag)

	cfg.Docker.InstallFlags(flag)

	return cmd, cfg
}

const startMaster = "master"
const startNode = "node"

func (cfg Config) Validate(args []string) error {
	switch len(args) {
	case 1:
		switch args[0] {
		case startMaster: // allowed case
		case startNode: // allowed case
		default:
			return errors.New("You may start an OpenShift all-in-one server with no arguments, or pass 'master' or 'node' to run in that role.")
		}
	case 0:
		// do nothing, this starts an all in one

	default:
		return errors.New("You may start an OpenShift all-in-one server with no arguments, or pass 'master' or 'node' to run in that role.")
	}

	return nil
}

// Complete takes the args and fills in information for the start config
func (cfg *Config) Complete(args []string) {
	cfg.StartMaster = (len(args) == 0) || (args[0] == startMaster)
	cfg.StartNode = (len(args) == 0) || (args[0] == startNode)

	if cfg.StartMaster {
		// if we've explicitly called out a kube location, don't start one in process
		cfg.StartKube = !cfg.KubernetesAddr.Provided
		// if we've explicitly called out an etcd location, don't start one in process
		cfg.StartEtcd = !cfg.EtcdAddr.Provided
	}
}
