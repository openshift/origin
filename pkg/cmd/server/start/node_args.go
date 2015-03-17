package start

import (
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"strconv"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/master/ports"
	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	"github.com/spf13/pflag"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	latestconfigapi "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/cmd/server/certs"
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

	DefaultKubernetesURL url.URL
	ClusterDomain        string
	ClusterDNS           net.IP

	BindAddrArg        *BindAddrArg
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
		glog.Warningf("Unable to lookup hostname, using %q: %v", hostname, err)
	}

	var dnsIP net.IP
	if clusterDNS := cmdutil.Env("OPENSHIFT_DNS_ADDR", ""); len(clusterDNS) > 0 {
		dnsIP = net.ParseIP(clusterDNS)
	}

	return &NodeArgs{
		NodeName: hostname,

		ClusterDomain: cmdutil.Env("OPENSHIFT_DNS_DOMAIN", "local"),
		ClusterDNS:    dnsIP,

		BindAddrArg:        NewDefaultBindAddrArg(),
		ImageFormatArgs:    NewDefaultImageFormatArgs(),
		KubeConnectionArgs: NewDefaultKubeConnectionArgs(),
		CertArgs:           NewDefaultCertArgs(),
	}
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
			BindAddress: net.JoinHostPort(args.BindAddrArg.BindAddr.Host, strconv.Itoa(ports.KubeletPort)),
			ServerCert:  certs.DefaultNodeServingCertInfo(args.CertArgs.CertDir, args.NodeName),
		},

		VolumeDirectory:       args.VolumeDir,
		NetworkContainerImage: args.ImageFormatArgs.ImageTemplate.ExpandOrDie("pod"),
		AllowDisabledDocker:   args.AllowDisabledDocker,

		DNSDomain: args.ClusterDomain,
		DNSIP:     dnsIP,

		MasterKubeConfig: certs.DefaultKubeConfigFilename(args.CertArgs.CertDir, "node-"+args.NodeName),
	}

	return config, nil
}

// WriteNode serializes the config to yaml.
func WriteNode(config *configapi.NodeConfig) ([]byte, error) {
	json, err := latestconfigapi.Codec.Encode(config)
	if err != nil {
		return nil, err
	}
	content, err := yaml.JSONToYAML(json)
	if err != nil {
		return nil, err
	}
	return content, nil
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
