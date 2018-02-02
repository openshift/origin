package node

import (
	"fmt"
	"net"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	kubeproxyoptions "k8s.io/kubernetes/cmd/kube-proxy/app"
	"k8s.io/kubernetes/pkg/proxy/apis/kubeproxyconfig"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
)

// Build creates the network Kubernetes component configs for a given NodeConfig, or returns
// an error
func Build(options configapi.NodeConfig) (*kubeproxyconfig.KubeProxyConfiguration, error) {
	proxyOptions := kubeproxyoptions.NewOptions()
	// get default config
	proxyconfig := proxyOptions.GetConfig()
	defaultedProxyConfig, err := proxyOptions.ApplyDefaults(proxyconfig)
	if err != nil {
		return nil, err
	}
	*proxyconfig = *defaultedProxyConfig

	proxyconfig.HostnameOverride = options.NodeName

	// BindAddress - Override default bind address from our config
	addr := options.ServingInfo.BindAddress
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("The provided value to bind to must be an ip:port %q", addr)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return nil, fmt.Errorf("The provided value to bind to must be an ip:port: %q", addr)
	}
	proxyconfig.BindAddress = ip.String()
	// MetricsBindAddress - disable by default but allow enablement until we switch to
	// reading proxy config directly
	proxyconfig.MetricsBindAddress = ""
	if arg := options.ProxyArguments["metrics-bind-address"]; len(arg) > 0 {
		proxyconfig.MetricsBindAddress = arg[0]
	}
	delete(options.ProxyArguments, "metrics-bind-address")

	// OOMScoreAdj, ResourceContainer - clear, we don't run in a container
	oomScoreAdj := int32(0)
	proxyconfig.OOMScoreAdj = &oomScoreAdj
	proxyconfig.ResourceContainer = ""

	// use the same client as the node
	proxyconfig.ClientConnection.KubeConfigFile = options.MasterKubeConfig

	// ProxyMode, set to iptables
	proxyconfig.Mode = "iptables"

	// IptablesSyncPeriod, set to our config value
	syncPeriod, err := time.ParseDuration(options.IPTablesSyncPeriod)
	if err != nil {
		return nil, fmt.Errorf("Cannot parse the provided ip-tables sync period (%s) : %v", options.IPTablesSyncPeriod, err)
	}
	proxyconfig.IPTables.SyncPeriod = metav1.Duration{
		Duration: syncPeriod,
	}
	masqueradeBit := int32(0)
	proxyconfig.IPTables.MasqueradeBit = &masqueradeBit

	// PortRange, use default
	// HostnameOverride, use default
	// ConfigSyncPeriod, use default
	// MasqueradeAll, use default
	// CleanupAndExit, use default
	// KubeAPIQPS, use default, doesn't apply until we build a separate client
	// KubeAPIBurst, use default, doesn't apply until we build a separate client
	// UDPIdleTimeout, use default

	// Resolve cmd flags to add any user overrides
	if err := cmdflags.Resolve(options.ProxyArguments, proxyOptions.AddFlags); len(err) > 0 {
		return nil, kerrors.NewAggregate(err)
	}

	if err := proxyOptions.Complete(); err != nil {
		return nil, err
	}

	return proxyconfig, nil
}
