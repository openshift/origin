package dns

import (
	"fmt"
	"sync"
	"time"

	godbus "github.com/godbus/dbus"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	utildbus "k8s.io/kubernetes/pkg/util/dbus"
)

const (
	// dnsmasqRetryInterval is the duration between attempts to register and listen to DBUS.
	dnsmasqRetryInterval = 2 * time.Second
	// dnsmasqRefreshInterval is the maximum time between refreshes of the current dnsmasq configuration.
	dnsmasqRefreshInterval = 30 * time.Second
	dbusDnsmasqPath        = "/uk/org/thekelleys/dnsmasq"
	dbusDnsmasqInterface   = "uk.org.thekelleys.dnsmasq"
)

type dnsmasqMonitor struct {
	// metricsName is the prefix to apply to registered prometheus metrics. If unset no
	// metrics will be registered.
	metricsName   string
	metricError   *prometheus.CounterVec
	metricRestart prometheus.Counter

	ready func() bool

	// dnsIP is the IP address this DNS server is reachable at from dnsmasq
	dnsIP string
	// dnsDomain is the domain name for this DNS server that dnsmasq should forward to
	dnsDomain string
	// lock controls sending a dnsmasq refresh
	lock sync.Mutex
}

func (m *dnsmasqMonitor) initMetrics() {
	m.metricError = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: m.metricsName,
		Subsystem: "dnsmasq_sync",
		Name:      "error_count_total",
		Help:      "Counter of sync failures with dnsmasq.",
	}, []string{"type"})
	m.metricRestart = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: m.metricsName,
		Subsystem: "dnsmasq_sync",
		Name:      "restart_count_total",
		Help:      "Counter of restarts detected from dnsmasq.",
	})
	if len(m.metricsName) > 0 {
		prometheus.MustRegister(m.metricError)
		prometheus.MustRegister(m.metricRestart)
	}
}

func (m *dnsmasqMonitor) Start(stopCh <-chan struct{}) error {
	if m.ready == nil {
		m.ready = func() bool { return true }
	}
	m.initMetrics()
	conn, err := utildbus.New().SystemBus()
	if err != nil {
		return fmt.Errorf("cannot connect to DBus: %v", err)
	}
	if err := conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, fmt.Sprintf("type='signal',path='%s',interface='%s'", dbusDnsmasqPath, dbusDnsmasqInterface)).Store(); err != nil {
		return fmt.Errorf("unable to add a match rule to the system DBus: %v", err)
	}
	go m.run(conn, stopCh)
	return nil
}

func (m *dnsmasqMonitor) run(conn utildbus.Connection, stopCh <-chan struct{}) {
	ch := make(chan *godbus.Signal, 20)
	defer func() {
		utilruntime.HandleCrash()
		glog.V(2).Infof("dnsmasq monitor shutting down")
		// clear our configuration on shutdown
		m.refresh(conn, false)
		// unregister the handler
		conn.Signal(ch)
	}()
	conn.Signal(ch)

	// watch for dnsmasq restart
	go utilwait.Until(func() {
		for s := range ch {
			if s.Path != dbusDnsmasqPath {
				continue
			}
			switch s.Name {
			case "uk.org.thekelleys.dnsmasq.Up":
				m.metricRestart.Inc()
				glog.V(2).Infof("dnsmasq restarted, refreshing server configuration")
				if err := m.refresh(conn, m.ready()); err != nil {
					utilruntime.HandleError(fmt.Errorf("unable to refresh dnsmasq status on dnsmasq startup: %v", err))
					m.metricError.WithLabelValues("restart").Inc()
				} else {
					m.metricError.WithLabelValues("restart").Add(0)
				}
			}
		}
	}, dnsmasqRetryInterval, stopCh)

	// no matter what, always keep trying to refresh dnsmasq
	go utilwait.Until(func() {
		for !m.ready() {
			glog.V(4).Infof("Waiting for DNS data to be available to update dnsmasq")
			time.Sleep(time.Second)
		}
		if err := m.refresh(conn, true); err != nil {
			utilruntime.HandleError(fmt.Errorf("unable to periodically refresh dnsmasq status: %v", err))
			m.metricError.WithLabelValues("periodic").Inc()
		} else {
			m.metricError.WithLabelValues("periodic").Add(0)
		}
	}, dnsmasqRefreshInterval, stopCh)

	<-stopCh
}

// refresh invokes dnsmasq with the requested configuration
func (m *dnsmasqMonitor) refresh(conn utildbus.Connection, ready bool) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	var addresses []string
	if ready {
		addresses = []string{
			fmt.Sprintf("/in-addr.arpa/%s", m.dnsIP),
			fmt.Sprintf("/%s/%s", m.dnsDomain, m.dnsIP),
		}
		glog.V(4).Infof("Instructing dnsmasq to set the following servers: %v", addresses)
	} else {
		glog.V(2).Infof("DNS data is not ready, removing configuration from dnsmasq")
	}
	return conn.Object(dbusDnsmasqInterface, dbusDnsmasqPath).
		Call("uk.org.thekelleys.SetDomainServers", 0, addresses).
		Store()
}
