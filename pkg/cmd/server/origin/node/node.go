package node

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"net"

	"github.com/golang/glog"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	nodeoptions "github.com/openshift/origin/pkg/cmd/server/kubernetes/node/options"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	utilnet "k8s.io/apimachinery/pkg/util/net"
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
	// If DNSIP is configured "0.0.0.0"
        // 1) Use NodeIP as Default
        // 2) If the user has specified an NodeName,
        //    lookup the IP from nodeName and use the first IPv4 address.
        // 3) Try to get the IP from the network interface used as default gateway.
        // 4) If not setting DNSIP through above steps, use the network interface first ip address.	
	if nodeConfig.DNSIP == "0.0.0.0" {
		glog.V(4).Infof("Defaulting to the DNSIP config to the node's IP")
		nodeConfig.DNSIP = nodeConfig.NodeIP
		if len(nodeConfig.NodeName) != 0 {
			glog.V(4).Infof("Setting to the DNSIP config to the nodeName's lookup IP")
			addrs, _ := net.LookupIP(nodeConfig.NodeName)
                        for _, addr := range addrs {
                        	if addr.To4() != nil {
				nodeConfig.DNSIP = addr.String()
				}
			}
                }
		// TODO: the Kubelet should do this defaulting (to the IP it recognizes)
		if len(nodeConfig.DNSIP) == 0 {
			if ip, err := utilnet.ChooseHostInterface(); ip != nil && err == nil {
                                glog.V(4).Infof("Setting to the DNSIP config to the default gateway IP")
                                nodeConfig.DNSIP = ip.String()
                        } else if ip, err := cmdutil.DefaultLocalIP4(); err == nil {
                                glog.V(4).Infof("Setting to the DNSIP config to the first network interface IP")
                                nodeConfig.DNSIP = ip.String()
                        }
		}
	}
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
