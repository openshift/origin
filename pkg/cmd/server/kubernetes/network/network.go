package network

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"

	v1 "k8s.io/api/core/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	kv1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/apis/core"
	proxy "k8s.io/kubernetes/pkg/proxy"
	"k8s.io/kubernetes/pkg/proxy/apis/kubeproxyconfig"
	pconfig "k8s.io/kubernetes/pkg/proxy/config"
	"k8s.io/kubernetes/pkg/proxy/healthcheck"
	"k8s.io/kubernetes/pkg/proxy/iptables"
	"k8s.io/kubernetes/pkg/proxy/metrics"
	"k8s.io/kubernetes/pkg/proxy/userspace"
	utiliptables "k8s.io/kubernetes/pkg/util/iptables"
	utilnode "k8s.io/kubernetes/pkg/util/node"
	utilsysctl "k8s.io/kubernetes/pkg/util/sysctl"
	utilexec "k8s.io/utils/exec"

	"github.com/openshift/origin/pkg/proxy/hybrid"
	"github.com/openshift/origin/pkg/proxy/unidler"
)

// RunSDN starts the SDN, if the OpenShift SDN network plugin is enabled in configuration.
func (c *NetworkConfig) RunSDN() {
	if c.SDNNode == nil {
		return
	}

	if err := c.SDNNode.Start(); err != nil {
		glog.Fatalf("SDN node startup failed: %v", err)
	}
}

// RunDNS starts the DNS server as soon as services are loaded.
func (c *NetworkConfig) RunDNS(stopCh <-chan struct{}) {
	go func() {
		glog.Infof("Starting DNS on %s", c.DNSServer.Config.DnsAddr)
		err := c.DNSServer.ListenAndServe(stopCh)
		glog.Fatalf("DNS server failed to start: %v", err)
	}()
}

// RunProxy starts the proxy and closes the provided channel when the proxy
// has initialized
func (c *NetworkConfig) RunProxy(waitChan chan<- bool) {
	protocol := utiliptables.ProtocolIpv4
	bindAddr := net.ParseIP(c.ProxyConfig.BindAddress)
	if bindAddr.To4() == nil {
		protocol = utiliptables.ProtocolIpv6
	}

	portRange := utilnet.ParsePortRangeOrDie(c.ProxyConfig.PortRange)

	hostname := utilnode.GetHostname(c.ProxyConfig.HostnameOverride)

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&kv1core.EventSinkImpl{Interface: c.KubeClientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "kube-proxy", Host: hostname})

	execer := utilexec.New()
	iptInterface := utiliptables.New(execer, protocol)

	var proxier proxy.ProxyProvider
	var err error
	var healthzServer *healthcheck.HealthzServer
	if len(c.ProxyConfig.HealthzBindAddress) > 0 {
		nodeRef := &v1.ObjectReference{
			Kind:      "Node",
			Name:      hostname,
			UID:       types.UID(hostname),
			Namespace: "",
		}
		healthzServer = healthcheck.NewDefaultHealthzServer(c.ProxyConfig.HealthzBindAddress, 2*c.ProxyConfig.IPTables.SyncPeriod.Duration, recorder, nodeRef)
	}

	switch c.ProxyConfig.Mode {
	case kubeproxyconfig.ProxyModeIPTables:
		glog.V(0).Info("Using iptables Proxier.")
		if bindAddr.Equal(net.IPv4zero) {
			bindAddr, err = getNodeIP(c.ExternalKubeClientset.CoreV1(), hostname)
			if err != nil {
				glog.Fatalf("Unable to get a bind address: %v", err)
			}
		}
		if c.ProxyConfig.IPTables.MasqueradeBit == nil {
			// IPTablesMasqueradeBit must be specified or defaulted.
			glog.Fatalf("Unable to read IPTablesMasqueradeBit from config")
		}
		proxier, err = iptables.NewProxier(
			iptInterface,
			utilsysctl.New(),
			execer,
			c.ProxyConfig.IPTables.SyncPeriod.Duration,
			c.ProxyConfig.IPTables.MinSyncPeriod.Duration,
			c.ProxyConfig.IPTables.MasqueradeAll,
			int(*c.ProxyConfig.IPTables.MasqueradeBit),
			c.ProxyConfig.ClusterCIDR,
			hostname,
			bindAddr,
			recorder,
			healthzServer,
			c.ProxyConfig.NodePortAddresses,
		)
		metrics.RegisterMetrics()

		if err != nil {
			glog.Fatalf("error: Could not initialize Kubernetes Proxy. You must run this process as root (and if containerized, in the host network namespace as privileged) to use the service proxy: %v", err)
		}
		// No turning back. Remove artifacts that might still exist from the userspace Proxier.
		glog.V(0).Info("Tearing down userspace rules.")
		userspace.CleanupLeftovers(iptInterface)
	case kubeproxyconfig.ProxyModeUserspace:
		glog.V(0).Info("Using userspace Proxier.")
		execer := utilexec.New()
		proxier, err = userspace.NewProxier(
			userspace.NewLoadBalancerRR(),
			bindAddr,
			iptInterface,
			execer,
			*portRange,
			c.ProxyConfig.IPTables.SyncPeriod.Duration,
			c.ProxyConfig.IPTables.MinSyncPeriod.Duration,
			c.ProxyConfig.UDPIdleTimeout.Duration,
			c.ProxyConfig.NodePortAddresses,
		)
		if err != nil {
			glog.Fatalf("error: Could not initialize Kubernetes Proxy. You must run this process as root (and if containerized, in the host network namespace as privileged) to use the service proxy: %v", err)
		}
		// Remove artifacts from the pure-iptables Proxier.
		glog.V(0).Info("Tearing down pure-iptables proxy rules.")
		iptables.CleanupLeftovers(iptInterface)
	default:
		glog.Fatalf("Unknown proxy mode %q", c.ProxyConfig.Mode)
	}

	// Create configs (i.e. Watches for Services and Endpoints)
	// Note: RegisterHandler() calls need to happen before creation of Sources because sources
	// only notify on changes, and the initial update (on process start) may be lost if no handlers
	// are registered yet.
	serviceConfig := pconfig.NewServiceConfig(
		c.InternalKubeInformers.Core().InternalVersion().Services(),
		c.ProxyConfig.ConfigSyncPeriod.Duration,
	)

	if c.EnableUnidling {
		signaler := unidler.NewEventSignaler(recorder)
		unidlingUserspaceProxy, err := unidler.NewUnidlerProxier(userspace.NewLoadBalancerRR(), bindAddr, iptInterface, execer, *portRange, c.ProxyConfig.IPTables.SyncPeriod.Duration, c.ProxyConfig.IPTables.MinSyncPeriod.Duration, c.ProxyConfig.UDPIdleTimeout.Duration, c.ProxyConfig.NodePortAddresses, signaler)
		if err != nil {
			glog.Fatalf("error: Could not initialize Kubernetes Proxy. You must run this process as root (and if containerized, in the host network namespace as privileged) to use the service proxy: %v", err)
		}
		hp, ok := proxier.(hybrid.RunnableProxy)
		if !ok {
			// unreachable
			glog.Fatalf("unidling proxy must be used in iptables mode")
		}
		proxier, err = hybrid.NewHybridProxier(
			hp,
			unidlingUserspaceProxy,
			c.ProxyConfig.IPTables.MinSyncPeriod.Duration,
			c.ProxyConfig.IPTables.SyncPeriod.Duration,
			c.InternalKubeInformers.Core().InternalVersion().Services().Lister(),
		)
		if err != nil {
			glog.Fatalf("error: Could not initialize Kubernetes Proxy. You must run this process as root (and if containerized, in the host network namespace as privileged) to use the service proxy: %v", err)
		}
	}

	endpointsConfig := pconfig.NewEndpointsConfig(
		c.InternalKubeInformers.Core().InternalVersion().Endpoints(),
		c.ProxyConfig.ConfigSyncPeriod.Duration,
	)
	// customized handling registration that inserts a filter if needed
	if c.SDNProxy != nil {
		if err := c.SDNProxy.Start(proxier); err != nil {
			glog.Fatalf("error: node proxy plugin startup failed: %v", err)
		}
		proxier = c.SDNProxy
	}

	waitingProxy := newWaitingProxyHandler(proxier, proxier, waitChan)

	//register serviceconfig event handler
	serviceConfig.RegisterEventHandler(waitingProxy)
	go serviceConfig.Run(utilwait.NeverStop)
	//register endpointsconfig event handler
	endpointsConfig.RegisterEventHandler(waitingProxy)
	go endpointsConfig.Run(utilwait.NeverStop)

	// Start up healthz server
	if len(c.ProxyConfig.HealthzBindAddress) > 0 {
		healthzServer.Run()
	}

	// Start up a metrics server if requested
	if len(c.ProxyConfig.MetricsBindAddress) > 0 {
		mux := http.NewServeMux()
		mux.HandleFunc("/proxyMode", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "%s", c.ProxyConfig.Mode)
		})
		mux.Handle("/metrics", prometheus.Handler())
		go utilwait.Until(func() {
			err := http.ListenAndServe(c.ProxyConfig.MetricsBindAddress, mux)
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("starting metrics server failed: %v", err))
			}
		}, 5*time.Second, utilwait.NeverStop)
	}

	// periodically sync k8s iptables rules
	go utilwait.Forever(proxier.SyncLoop, 0)
	glog.Infof("Started Kubernetes Proxy on %s", c.ProxyConfig.BindAddress)
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
			glog.Warningf("waiting for node %q to be registered with master...", hostname)
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

	serviceChild    pconfig.ServiceHandler
	serviceSynced   bool
	endpointsChild  pconfig.EndpointsHandler
	endpointsSynced bool
}

func newWaitingProxyHandler(serviceChild pconfig.ServiceHandler, endpointsChild pconfig.EndpointsHandler, waitChan chan<- bool) *waitingProxyHandler {
	return &waitingProxyHandler{
		serviceChild:   serviceChild,
		endpointsChild: endpointsChild,
		waitChan:       waitChan,
	}
}

func (wph *waitingProxyHandler) checkInitialized() {
	if !wph.initialized && wph.serviceSynced && wph.endpointsSynced {
		glog.Info("openshift-sdn proxy services and endpoints initialized")
		wph.initialized = true
		close(wph.waitChan)
	}
}

func (wph *waitingProxyHandler) OnServiceAdd(service *core.Service) {
	wph.serviceChild.OnServiceAdd(service)
}

func (wph *waitingProxyHandler) OnServiceUpdate(oldService, service *core.Service) {
	wph.serviceChild.OnServiceUpdate(oldService, service)
}

func (wph *waitingProxyHandler) OnServiceDelete(service *core.Service) {
	wph.serviceChild.OnServiceDelete(service)
}

func (wph *waitingProxyHandler) OnServiceSynced() {
	wph.serviceChild.OnServiceSynced()
	wph.Lock()
	defer wph.Unlock()
	wph.serviceSynced = true
	wph.checkInitialized()
}

func (wph *waitingProxyHandler) OnEndpointsAdd(endpoints *core.Endpoints) {
	wph.endpointsChild.OnEndpointsAdd(endpoints)
}

func (wph *waitingProxyHandler) OnEndpointsUpdate(oldEndpoints, endpoints *core.Endpoints) {
	wph.endpointsChild.OnEndpointsUpdate(oldEndpoints, endpoints)
}

func (wph *waitingProxyHandler) OnEndpointsDelete(endpoints *core.Endpoints) {
	wph.endpointsChild.OnEndpointsDelete(endpoints)
}

func (wph *waitingProxyHandler) OnEndpointsSynced() {
	wph.endpointsChild.OnEndpointsSynced()
	wph.Lock()
	defer wph.Unlock()
	wph.endpointsSynced = true
	wph.checkInitialized()
}
