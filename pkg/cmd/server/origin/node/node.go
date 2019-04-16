package node

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	utilnet "k8s.io/apimachinery/pkg/util/net"

	"github.com/golang/glog"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	nodeoptions "github.com/openshift/origin/pkg/cmd/server/kubernetes/node/options"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

// safeArgRegexp matches only characters that are known safe. DO NOT add to this list
// without fully considering whether that new character can be used to break shell escaping
// rules.
var safeArgRegexp = regexp.MustCompile(`^[\da-zA-Z\-=_\.,/\:]+$`)

// shellEscapeArg quotes an argument if it contains characters that my cause a shell
// interpreter to split the single argument into multiple.
func shellEscapeArg(s string) string {
	if safeArgRegexp.MatchString(s) {
		return s
	}
	return strconv.Quote(s)
}

// FinalizeNodeConfig controls the node configuration before it is used by the Kubelet
func FinalizeNodeConfig(nodeConfig *configapi.NodeConfig) error {
	return SetDNSIP(nodeConfig)
}

// SetDNSIP sets DNSIP for the node if it is not already set (i.e. '0.0.0.0').
// To determine the reasonable default DNSIP, it follows the same steps used by kubelet
// that gets the routable IP address for the node. This also ensures consistent and
// sane default IP address for DNS in case of multiple NICs on the node.
func SetDNSIP(nodeConfig *configapi.NodeConfig) error {
	if nodeConfig.DNSIP != "0.0.0.0" {
		return nil
	}

	// 1) Use nodeIP if set
	// 2) If the user has specified an IP to HostnameOverride, use it
	// 3) Lookup the IP from node name by DNS and use the first valid IPv4 address.
	//    If the node does not have a valid IPv4 address, use the first valid IPv6 address.
	// 4) Try to get the IP from the network interface used as default gateway
	if len(nodeConfig.NodeIP) != 0 {
		nodeConfig.DNSIP = nodeConfig.NodeIP
		glog.V(4).Infof("defaulting DNSIP config to the node's IP")
		return nil
	}

	hostname := cmdutil.GetHostname(nodeConfig.NodeName)
	if ipAddr := net.ParseIP(hostname); ipAddr != nil {
		nodeConfig.DNSIP = ipAddr.String()
		glog.V(4).Infof("defaulting DNSIP config to the IP parsed from node's hostname")
		return nil
	}

	var ipAddr net.IP
	addrs, _ := net.LookupIP(nodeConfig.NodeName)
	for _, addr := range addrs {
		if err := cmdutil.ValidateNodeIP(addr); err == nil {
			if addr.To4() != nil {
				ipAddr = addr
				break
			}
			if addr.To16() != nil && ipAddr == nil {
				ipAddr = addr
			}
		}
	}
	if ipAddr != nil {
		nodeConfig.DNSIP = ipAddr.String()
		glog.V(4).Infof("defaulting DNSIP config to the IP derived from DNS lookup of node's name")
		return nil
	}

	ipAddr, err := utilnet.ChooseHostInterface()
	if err != nil {
		return fmt.Errorf("failed to get IP address from node's host interface: %v", err)
	}
	nodeConfig.DNSIP = ipAddr.String()
	glog.V(4).Infof("defaulting DNSIP config to the IP address from node's host interface")
	return nil
}

// WriteKubeletFlags writes the correct set of flags to start a Kubelet from the provided node config to
// stdout, instead of launching anything.
func WriteKubeletFlags(nodeConfig configapi.NodeConfig) error {
	kubeletArgs, err := nodeoptions.ComputeKubeletFlags(nodeConfig.KubeletArguments, nodeConfig)
	if err != nil {
		return fmt.Errorf("cannot create kubelet args: %v", err)
	}
	if err := nodeoptions.CheckFlags(kubeletArgs); err != nil {
		return err
	}
	var outputArgs []string
	for _, s := range kubeletArgs {
		outputArgs = append(outputArgs, shellEscapeArg(s))
	}
	fmt.Println(strings.Join(outputArgs, " "))
	return nil
}
