package start

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"strconv"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/master/ports"
	"github.com/spf13/pflag"

	"github.com/openshift/origin/pkg/cmd/server/admin"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

// NodeArgs is a struct that the command stores flag values into.  It holds a partially complete set of parameters for starting the master
// This object should hold the common set values, but not attempt to handle all cases.  The expected path is to use this object to create
// a fully specified config later on.  If you need something not set here, then create a fully specified config file and pass that as argument
// to starting the master.
type NodeArgs struct {
	NodeName string

	AllowDisabledDocker bool
	VolumeDir           string

	DefaultKubernetesURL *url.URL
	ClusterDomain        string
	ClusterDNS           net.IP

	ListenArg          *ListenArg
	ImageFormatArgs    *ImageFormatArgs
	KubeConnectionArgs *KubeConnectionArgs
	CertArgs           *CertArgs
}

// BindNodeArgs binds the options to the flags with prefix + default flag names
func BindNodeArgs(args *NodeArgs, flags *pflag.FlagSet, prefix string) {
	flags.StringVar(&args.VolumeDir, prefix+"volume-dir", "openshift.local.volumes", "The volume storage directory.")
	// TODO rename this node-name and recommend hostname -f
	flags.StringVar(&args.NodeName, prefix+"hostname", args.NodeName, "The hostname to identify this node with the master.")
}

// NewDefaultNodeArgs creates NodeArgs with sub-objects created and default values set.
func NewDefaultNodeArgs() *NodeArgs {
	hostname, err := defaultHostname()
	if err != nil {
		hostname = "localhost"
	}

	var dnsIP net.IP
	if clusterDNS := cmdutil.Env("OPENSHIFT_DNS_ADDR", ""); len(clusterDNS) > 0 {
		dnsIP = net.ParseIP(clusterDNS)
	}

	return &NodeArgs{
		NodeName: hostname,

		ClusterDomain: cmdutil.Env("OPENSHIFT_DNS_DOMAIN", "local"),
		ClusterDNS:    dnsIP,

		ListenArg:          NewDefaultListenArg(),
		ImageFormatArgs:    NewDefaultImageFormatArgs(),
		KubeConnectionArgs: NewDefaultKubeConnectionArgs(),
		CertArgs:           NewDefaultCertArgs(),
	}
}

func (args NodeArgs) Validate() error {
	if err := args.KubeConnectionArgs.Validate(); err != nil {
		return err
	}
	if _, err := args.KubeConnectionArgs.GetKubernetesAddress(args.DefaultKubernetesURL); err != nil {
		return errors.New("--kubeconfig must be set to provide API server connection information")
	}

	return nil
}

// BuildSerializeableNodeConfig takes the NodeArgs (partially complete config) and uses them along with defaulting behavior to create the fully specified
// config object for starting the node
func (args NodeArgs) BuildSerializeableNodeConfig() (*configapi.NodeConfig, error) {
	var dnsIP string
	if len(args.ClusterDNS) > 0 {
		dnsIP = args.ClusterDNS.String()
	}

	config := &configapi.NodeConfig{
		NodeName: args.NodeName,

		ServingInfo: configapi.ServingInfo{
			BindAddress: net.JoinHostPort(args.ListenArg.ListenAddr.Host, strconv.Itoa(ports.KubeletPort)),
		},

		ImageConfig: configapi.ImageConfig{
			Format: args.ImageFormatArgs.ImageTemplate.Format,
			Latest: args.ImageFormatArgs.ImageTemplate.Latest,
		},

		VolumeDirectory:     args.VolumeDir,
		AllowDisabledDocker: args.AllowDisabledDocker,

		DNSDomain: args.ClusterDomain,
		DNSIP:     dnsIP,

		MasterKubeConfig: admin.DefaultNodeKubeConfigFile(args.CertArgs.CertDir, args.NodeName),
	}

	if args.ListenArg.UseTLS() {
		config.ServingInfo.ServerCert = admin.DefaultNodeServingCertInfo(args.CertArgs.CertDir, args.NodeName)
		config.ServingInfo.ClientCA = admin.DefaultKubeletClientCAFile(args.CertArgs.CertDir)
	}

	return config, nil
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
