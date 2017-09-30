package network

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	kv1core "k8s.io/client-go/kubernetes/typed/core/v1"
	kclientv1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	kclientsetcorev1 "k8s.io/kubernetes/pkg/client/clientset_generated/clientset/typed/core/v1"
	proxy "k8s.io/kubernetes/pkg/proxy"
	pconfig "k8s.io/kubernetes/pkg/proxy/config"
	"k8s.io/kubernetes/pkg/proxy/healthcheck"
	"k8s.io/kubernetes/pkg/proxy/iptables"
	"k8s.io/kubernetes/pkg/proxy/userspace"
	utildbus "k8s.io/kubernetes/pkg/util/dbus"
	kexec "k8s.io/kubernetes/pkg/util/exec"
	utilexec "k8s.io/kubernetes/pkg/util/exec"
	utiliptables "k8s.io/kubernetes/pkg/util/iptables"
	utilnode "k8s.io/kubernetes/pkg/util/node"
	utilsysctl "k8s.io/kubernetes/pkg/util/sysctl"

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
func (c *NetworkConfig) RunDNS() {
	go func() {
		glog.Infof("Starting DNS on %s", c.DNSServer.Config.DnsAddr)
		err := c.DNSServer.ListenAndServe()
		glog.Fatalf("DNS server failed to start: %v", err)
	}()
}

// RunProxy starts the proxy
func (c *NetworkConfig) RunProxy() {
	protocol := utiliptables.ProtocolIpv4
	bindAddr := net.ParseIP(c.ProxyConfig.BindAddress)
	if bindAddr.To4() == nil {
		protocol = utiliptables.ProtocolIpv6
	}

	portRange := utilnet.ParsePortRangeOrDie(c.ProxyConfig.PortRange)

	hostname := utilnode.GetHostname(c.ProxyConfig.HostnameOverride)

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&kv1core.EventSinkImpl{Interface: c.KubeClientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, kclientv1.EventSource{Component: "kube-proxy", Host: hostname})

	execer := kexec.New()
	dbus := utildbus.New()
	iptInterface := utiliptables.New(execer, dbus, protocol)

	var proxier proxy.ProxyProvider
	var servicesHandler pconfig.ServiceHandler
	var endpointsHandler pconfig.EndpointsHandler
	var healthzServer *healthcheck.HealthzServer
	if len(c.ProxyConfig.HealthzBindAddress) > 0 {
		healthzServer = healthcheck.NewDefaultHealthzServer(c.ProxyConfig.HealthzBindAddress, 2*c.ProxyConfig.IPTables.SyncPeriod.Duration)
	}

	switch c.ProxyConfig.Mode {
	case componentconfig.ProxyModeIPTables:
		glog.V(0).Info("Using iptables Proxier.")
		if bindAddr.Equal(net.IPv4zero) {
			bindAddr = getNodeIP(c.ExternalKubeClientset.CoreV1(), hostname)
		}
		if c.ProxyConfig.IPTables.MasqueradeBit == nil {
			// IPTablesMasqueradeBit must be specified or defaulted.
			glog.Fatalf("Unable to read IPTablesMasqueradeBit from config")
		}
		proxierIptables, err := iptables.NewProxier(
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
		)
		iptables.RegisterMetrics()

		if err != nil {
			glog.Fatalf("error: Could not initialize Kubernetes Proxy. You must run this process as root (and if containerized, in the host network namespace as privileged) to use the service proxy: %v", err)
		}
		proxier = proxierIptables
		endpointsHandler = proxierIptables
		servicesHandler = proxierIptables
		// No turning back. Remove artifacts that might still exist from the userspace Proxier.
		glog.V(0).Info("Tearing down userspace rules.")
		userspace.CleanupLeftovers(iptInterface)
	case componentconfig.ProxyModeUserspace:
		glog.V(0).Info("Using userspace Proxier.")
		// This is a proxy.LoadBalancer which NewProxier needs but has methods we don't need for
		// our config.EndpointsHandler.
		loadBalancer := userspace.NewLoadBalancerRR()
		// set EndpointsHandler to our loadBalancer
		endpointsHandler = loadBalancer

		execer := utilexec.New()
		proxierUserspace, err := userspace.NewProxier(
			loadBalancer,
			bindAddr,
			iptInterface,
			execer,
			*portRange,
			c.ProxyConfig.IPTables.SyncPeriod.Duration,
			c.ProxyConfig.IPTables.MinSyncPeriod.Duration,
			c.ProxyConfig.UDPIdleTimeout.Duration,
		)
		if err != nil {
			glog.Fatalf("error: Could not initialize Kubernetes Proxy. You must run this process as root (and if containerized, in the host network namespace as privileged) to use the service proxy: %v", err)
		}
		proxier = proxierUserspace
		servicesHandler = proxierUserspace
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
		unidlingLoadBalancer := userspace.NewLoadBalancerRR()
		signaler := unidler.NewEventSignaler(recorder)
		unidlingUserspaceProxy, err := unidler.NewUnidlerProxier(unidlingLoadBalancer, bindAddr, iptInterface, execer, *portRange, c.ProxyConfig.IPTables.SyncPeriod.Duration, c.ProxyConfig.IPTables.MinSyncPeriod.Duration, c.ProxyConfig.UDPIdleTimeout.Duration, signaler)
		if err != nil {
			glog.Fatalf("error: Could not initialize Kubernetes Proxy. You must run this process as root (and if containerized, in the host network namespace as privileged) to use the service proxy: %v", err)
		}
		hybridProxier, err := hybrid.NewHybridProxier(
			unidlingLoadBalancer,
			unidlingUserspaceProxy,
			endpointsHandler,
			servicesHandler,
			proxier,
			unidlingUserspaceProxy,
			c.ProxyConfig.IPTables.SyncPeriod.Duration,
			c.InternalKubeInformers.Core().InternalVersion().Services().Lister(),
		)
		if err != nil {
			glog.Fatalf("error: Could not initialize Kubernetes Proxy. You must run this process as root (and if containerized, in the host network namespace as privileged) to use the service proxy: %v", err)
		}
		endpointsHandler = hybridProxier
		servicesHandler = hybridProxier
		proxier = hybridProxier
	}

	iptInterface.AddReloadFunc(proxier.Sync)
	serviceConfig.RegisterEventHandler(servicesHandler)
	go serviceConfig.Run(utilwait.NeverStop)

	endpointsConfig := pconfig.NewEndpointsConfig(
		c.InternalKubeInformers.Core().InternalVersion().Endpoints(),
		c.ProxyConfig.ConfigSyncPeriod.Duration,
	)
	// customized handling registration that inserts a filter if needed
	if c.SDNProxy != nil {
		if err := c.SDNProxy.Start(endpointsHandler); err != nil {
			glog.Fatalf("error: node proxy plugin startup failed: %v", err)
		}
		endpointsHandler = c.SDNProxy
	}
	endpointsConfig.RegisterEventHandler(endpointsHandler)
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
func getNodeIP(client kclientsetcorev1.CoreV1Interface, hostname string) net.IP {
	var nodeIP net.IP
	node, err := client.Nodes().Get(hostname, metav1.GetOptions{})
	if err != nil {
		glog.Warningf("Failed to retrieve node info: %v", err)
		return nil
	}
	nodeIP, err = utilnode.GetNodeHostIP(node)
	if err != nil {
		glog.Warningf("Failed to retrieve node IP: %v", err)
		return nil
	}
	return nodeIP
}
