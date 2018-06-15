package node

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

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
	if nodeConfig.DNSIP == "0.0.0.0" {
		glog.V(4).Infof("Defaulting to the DNSIP config to the node's IP")
		nodeConfig.DNSIP = nodeConfig.NodeIP
		// TODO: the Kubelet should do this defaulting (to the IP it recognizes)
		if len(nodeConfig.DNSIP) == 0 {
			if ip, err := cmdutil.DefaultLocalIP4(); err == nil {
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
