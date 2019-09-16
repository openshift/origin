package openshift_sdn

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"k8s.io/api/core/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	apiserverflag "k8s.io/apiserver/pkg/util/flag"
	"k8s.io/client-go/kubernetes/scheme"
	kv1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	kubeproxyoptions "k8s.io/kubernetes/cmd/kube-proxy/app"
	proxy "k8s.io/kubernetes/pkg/proxy"
	kubeproxyconfig "k8s.io/kubernetes/pkg/proxy/apis/config"
	pconfig "k8s.io/kubernetes/pkg/proxy/config"
	"k8s.io/kubernetes/pkg/proxy/healthcheck"
	"k8s.io/kubernetes/pkg/proxy/iptables"
	"k8s.io/kubernetes/pkg/proxy/metrics"
	"k8s.io/kubernetes/pkg/proxy/userspace"
	utildbus "k8s.io/kubernetes/pkg/util/dbus"
	utiliptables "k8s.io/kubernetes/pkg/util/iptables"
	utilnode "k8s.io/kubernetes/pkg/util/node"
	utilsysctl "k8s.io/kubernetes/pkg/util/sysctl"
	utilexec "k8s.io/utils/exec"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
	sdnproxy "github.com/openshift/origin/pkg/network/proxy"
	"github.com/openshift/origin/pkg/proxy/hybrid"
	"github.com/openshift/origin/pkg/proxy/unidler"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog"
)

// ProxyConfigFromNodeConfig builds the kube-proxy configuration from the already-parsed nodeconfig.
func ProxyConfigFromNodeConfig(options configapi.NodeConfig) (*kubeproxyconfig.KubeProxyConfiguration, error) {
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
	proxyconfig.MetricsBindAddress = "0.0.0.0:10253"
	if arg := options.ProxyArguments["metrics-bind-address"]; len(arg) > 0 {
		proxyconfig.MetricsBindAddress = arg[0]
	}
	delete(options.ProxyArguments, "metrics-bind-address")

	// OOMScoreAdj, ResourceContainer - clear, we don't run in a container
	oomScoreAdj := int32(0)
	proxyconfig.OOMScoreAdj = &oomScoreAdj
	proxyconfig.ResourceContainer = ""

	// use the same client as the node
	proxyconfig.ClientConnection.Kubeconfig = options.MasterKubeConfig

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
	fss := apiserverflag.NamedFlagSets{}
	proxyOptions.AddFlags(fss.FlagSet("proxy"))
	if err := cmdflags.Resolve(options.ProxyArguments, fss); len(err) > 0 {
		return nil, kerrors.NewAggregate(err)
	}

	if err := proxyOptions.Complete(); err != nil {
		return nil, err
	}

	return proxyconfig, nil
}

// initProxy sets up the proxy process.
func (sdn *OpenShiftSDN) initProxy() error {
	var err error
	sdn.OsdnProxy, err = sdnproxy.New(
		sdn.NodeConfig.NetworkConfig.NetworkPluginName,
		sdn.informers.NetworkClient,
		sdn.informers.KubeClient,
		sdn.informers.NetworkInformers)
	return err
}

// runProxy starts the configured proxy process and closes the provided channel
// when the proxy has initialized
func (sdn *OpenShiftSDN) runProxy(waitChan chan<- bool) {
	protocol := utiliptables.ProtocolIpv4
	bindAddr := net.ParseIP(sdn.ProxyConfig.BindAddress)
	if bindAddr.To4() == nil {
		protocol = utiliptables.ProtocolIpv6
	}

	portRange := utilnet.ParsePortRangeOrDie(sdn.ProxyConfig.PortRange)

	hostname, err := utilnode.GetHostname(sdn.ProxyConfig.HostnameOverride)
	if err != nil {
		klog.Fatalf("Unable to get hostname: %v", err)
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&kv1core.EventSinkImpl{Interface: sdn.informers.KubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "kube-proxy", Host: hostname})

	execer := utilexec.New()
	dbus := utildbus.New()
	iptInterface := utiliptables.New(execer, dbus, protocol)

	var proxier proxy.ProxyProvider
	var healthzServer *healthcheck.HealthzServer
	if len(sdn.ProxyConfig.HealthzBindAddress) > 0 {
		nodeRef := &v1.ObjectReference{
			Kind:      "Node",
			Name:      hostname,
			UID:       types.UID(hostname),
			Namespace: "",
		}
		healthzServer = healthcheck.NewDefaultHealthzServer(sdn.ProxyConfig.HealthzBindAddress, 2*sdn.ProxyConfig.IPTables.SyncPeriod.Duration, recorder, nodeRef)
	}

	switch sdn.ProxyConfig.Mode {
	case kubeproxyconfig.ProxyModeIPTables:
		klog.V(0).Info("Using iptables Proxier.")
		if bindAddr.Equal(net.IPv4zero) {
			var err error
			bindAddr, err = getNodeIP(sdn.informers.KubeClient.CoreV1(), hostname)
			if err != nil {
				klog.Fatalf("Unable to get a bind address: %v", err)
			}
		}
		if sdn.ProxyConfig.IPTables.MasqueradeBit == nil {
			// IPTablesMasqueradeBit must be specified or defaulted.
			klog.Fatalf("Unable to read IPTablesMasqueradeBit from config")
		}
		proxier, err = iptables.NewProxier(
			iptInterface,
			utilsysctl.New(),
			execer,
			sdn.ProxyConfig.IPTables.SyncPeriod.Duration,
			sdn.ProxyConfig.IPTables.MinSyncPeriod.Duration,
			sdn.ProxyConfig.IPTables.MasqueradeAll,
			int(*sdn.ProxyConfig.IPTables.MasqueradeBit),
			sdn.ProxyConfig.ClusterCIDR,
			hostname,
			bindAddr,
			recorder,
			healthzServer,
			sdn.ProxyConfig.NodePortAddresses,
		)
		metrics.RegisterMetrics()

		if err != nil {
			klog.Fatalf("error: Could not initialize Kubernetes Proxy. You must run this process as root (and if containerized, in the host network namespace as privileged) to use the service proxy: %v", err)
		}
		// No turning back. Remove artifacts that might still exist from the userspace Proxier.
		klog.V(0).Info("Tearing down userspace rules.")
		userspace.CleanupLeftovers(iptInterface)
	case kubeproxyconfig.ProxyModeUserspace:
		klog.V(0).Info("Using userspace Proxier.")

		execer := utilexec.New()
		proxier, err = userspace.NewProxier(
			userspace.NewLoadBalancerRR(),
			bindAddr,
			iptInterface,
			execer,
			*portRange,
			sdn.ProxyConfig.IPTables.SyncPeriod.Duration,
			sdn.ProxyConfig.IPTables.MinSyncPeriod.Duration,
			sdn.ProxyConfig.UDPIdleTimeout.Duration,
			sdn.ProxyConfig.NodePortAddresses,
		)
		if err != nil {
			klog.Fatalf("error: Could not initialize Kubernetes Proxy. You must run this process as root (and if containerized, in the host network namespace as privileged) to use the service proxy: %v", err)
		}
		// Remove artifacts from the pure-iptables Proxier.
		klog.V(0).Info("Tearing down pure-iptables proxy rules.")
		iptables.CleanupLeftovers(iptInterface)
	default:
		klog.Fatalf("Unknown proxy mode %q", sdn.ProxyConfig.Mode)
	}

	// Create configs (i.e. Watches for Services and Endpoints)
	// Note: RegisterHandler() calls need to happen before creation of Sources because sources
	// only notify on changes, and the initial update (on process start) may be lost if no handlers
	// are registered yet.
	serviceConfig := pconfig.NewServiceConfig(
		sdn.informers.KubeInformers.Core().V1().Services(),
		sdn.ProxyConfig.ConfigSyncPeriod.Duration,
	)

	if sdn.NodeConfig.EnableUnidling {
		signaler := unidler.NewEventSignaler(recorder)
		unidlingUserspaceProxy, err := unidler.NewUnidlerProxier(userspace.NewLoadBalancerRR(), bindAddr, iptInterface, execer, *portRange, sdn.ProxyConfig.IPTables.SyncPeriod.Duration, sdn.ProxyConfig.IPTables.MinSyncPeriod.Duration, sdn.ProxyConfig.UDPIdleTimeout.Duration, sdn.ProxyConfig.NodePortAddresses, signaler)
		if err != nil {
			klog.Fatalf("error: Could not initialize Kubernetes Proxy. You must run this process as root (and if containerized, in the host network namespace as privileged) to use the service proxy: %v", err)
		}
		hp, ok := proxier.(hybrid.RunnableProxy)
		if !ok {
			// unreachable
			klog.Fatalf("unidling proxy must be used in iptables mode")
		}
		proxier, err = hybrid.NewHybridProxier(
			hp,
			unidlingUserspaceProxy,
			sdn.ProxyConfig.IPTables.SyncPeriod.Duration,
			sdn.ProxyConfig.IPTables.MinSyncPeriod.Duration,
			sdn.informers.KubeInformers.Core().V1().Services().Lister(),
		)
		if err != nil {
			klog.Fatalf("error: Could not initialize Kubernetes Proxy. You must run this process as root (and if containerized, in the host network namespace as privileged) to use the service proxy: %v", err)
		}
	}

	endpointsConfig := pconfig.NewEndpointsConfig(
		sdn.informers.KubeInformers.Core().V1().Endpoints(),
		sdn.ProxyConfig.ConfigSyncPeriod.Duration,
	)
	// customized handling registration that inserts a filter if needed
	if err := sdn.OsdnProxy.Start(proxier); err != nil {
		klog.Fatalf("error: node proxy plugin startup failed: %v", err)
	}
	proxier = sdn.OsdnProxy

	// Wrap the proxy to know when it finally initializes
	waitingProxy := newWaitingProxyHandler(proxier, waitChan)

	iptInterface.AddReloadFunc(proxier.Sync)
	serviceConfig.RegisterEventHandler(waitingProxy)
	go serviceConfig.Run(utilwait.NeverStop)

	endpointsConfig.RegisterEventHandler(waitingProxy)
	go endpointsConfig.Run(utilwait.NeverStop)

	// Start up healthz server
	if len(sdn.ProxyConfig.HealthzBindAddress) > 0 {
		healthzServer.Run()
	}

	// Start up a metrics server if requested
	if len(sdn.ProxyConfig.MetricsBindAddress) > 0 {
		mux := http.NewServeMux()
		mux.HandleFunc("/proxyMode", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "%s", sdn.ProxyConfig.Mode)
		})
		mux.Handle("/metrics", prometheus.Handler())
		go utilwait.Until(func() {
			err := http.ListenAndServe(sdn.ProxyConfig.MetricsBindAddress, mux)
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("starting metrics server failed: %v", err))
			}
		}, 5*time.Second, utilwait.NeverStop)
	}

	// periodically sync k8s iptables rules
	go utilwait.Forever(proxier.SyncLoop, 0)
	klog.Infof("Started Kubernetes Proxy on %s", sdn.ProxyConfig.BindAddress)
}

// getNodeIP is copied from the upstream proxy config to retrieve the IP of a node.
func getNodeIP(client kv1core.CoreV1Interface, hostname string) (net.IP, error) {
	var node *v1.Node
	var nodeErr error

	// We may beat the thread that causes the node object to be created,
	// so if we can't get it, then we need to wait.
	// This will wait 0, 2, 4, 8, ... 64 seconds, for a total of ~2 mins
	nodeWaitBackoff := utilwait.Backoff{
		Duration: 2 * time.Second,
		Factor:   2,
		Steps:    7,
	}
	utilwait.ExponentialBackoff(nodeWaitBackoff, func() (bool, error) {
		node, nodeErr = client.Nodes().Get(hostname, metav1.GetOptions{})
		if nodeErr == nil {
			return true, nil
		} else if kapierrors.IsNotFound(nodeErr) {
			klog.Warningf("waiting for node %q to be registered with master...", hostname)
			return false, nil
		} else {
			return false, nodeErr
		}
	})
	if nodeErr != nil {
		return nil, fmt.Errorf("failed to retrieve node info (after waiting): %v", nodeErr)
	}

	nodeIP, err := utilnode.GetNodeHostIP(node)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve node IP: %v", err)
	}

	return nodeIP, nil
}

type waitingProxyHandler struct {
	sync.Mutex

	// waitChan will be closed when both services and endpoints have
	// been synced in the proxy
	waitChan    chan<- bool
	initialized bool

	proxier         proxy.ProxyProvider
	serviceSynced   bool
	endpointsSynced bool
}

func newWaitingProxyHandler(proxier proxy.ProxyProvider, waitChan chan<- bool) *waitingProxyHandler {
	return &waitingProxyHandler{
		proxier:  proxier,
		waitChan: waitChan,
	}
}

func (wph *waitingProxyHandler) checkInitialized() {
	if !wph.initialized && wph.serviceSynced && wph.endpointsSynced {
		klog.V(2).Info("openshift-sdn proxy services and endpoints initialized")
		wph.initialized = true
		close(wph.waitChan)
	}
}

func (wph *waitingProxyHandler) OnServiceAdd(service *v1.Service) {
	wph.proxier.OnServiceAdd(service)
}

func (wph *waitingProxyHandler) OnServiceUpdate(oldService, service *v1.Service) {
	wph.proxier.OnServiceUpdate(oldService, service)
}

func (wph *waitingProxyHandler) OnServiceDelete(service *v1.Service) {
	wph.proxier.OnServiceDelete(service)
}

func (wph *waitingProxyHandler) OnServiceSynced() {
	wph.proxier.OnServiceSynced()
	wph.Lock()
	defer wph.Unlock()
	wph.serviceSynced = true
	wph.checkInitialized()
}

func (wph *waitingProxyHandler) OnEndpointsAdd(endpoints *v1.Endpoints) {
	wph.proxier.OnEndpointsAdd(endpoints)
}

func (wph *waitingProxyHandler) OnEndpointsUpdate(oldEndpoints, endpoints *v1.Endpoints) {
	wph.proxier.OnEndpointsUpdate(oldEndpoints, endpoints)
}

func (wph *waitingProxyHandler) OnEndpointsDelete(endpoints *v1.Endpoints) {
	wph.proxier.OnEndpointsDelete(endpoints)
}

func (wph *waitingProxyHandler) OnEndpointsSynced() {
	wph.proxier.OnEndpointsSynced()
	wph.Lock()
	defer wph.Unlock()
	wph.endpointsSynced = true
	wph.checkInitialized()
}
