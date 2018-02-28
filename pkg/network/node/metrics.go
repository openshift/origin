package node

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/openshift/origin/pkg/util/ovs"
	"github.com/prometheus/client_golang/prometheus"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

const (
	SDNNamespace = "openshift"
	SDNSubsystem = "sdn"

	OVSFlowsKey                 = "ovs_flows"
	ARPCacheAvailableEntriesKey = "arp_cache_entries"
	PodIPsKey                   = "pod_ips"
	PodOperationsErrorsKey      = "pod_operations_errors"
	PodOperationsLatencyKey     = "pod_operations_latency"
	VnidNotFoundErrorsKey       = "vnid_not_found_errors"

	// Pod Operation types
	PodOperationSetup    = "setup"
	PodOperationTeardown = "teardown"
)

var (
	OVSFlows = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: SDNNamespace,
			Subsystem: SDNSubsystem,
			Name:      OVSFlowsKey,
			Help:      "Number of Open vSwitch flows",
		},
	)

	ARPCacheAvailableEntries = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: SDNNamespace,
			Subsystem: SDNSubsystem,
			Name:      ARPCacheAvailableEntriesKey,
			Help:      "Number of available entries in the ARP cache",
		},
	)

	PodIPs = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: SDNNamespace,
			Subsystem: SDNSubsystem,
			Name:      PodIPsKey,
			Help:      "Number of allocated pod IPs",
		},
	)

	PodOperationsErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: SDNNamespace,
			Subsystem: SDNSubsystem,
			Name:      PodOperationsErrorsKey,
			Help:      "Cumulative number of SDN operation errors by operation type",
		},
		[]string{"operation_type"},
	)

	PodOperationsLatency = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: SDNNamespace,
			Subsystem: SDNSubsystem,
			Name:      PodOperationsLatencyKey,
			Help:      "Latency in microseconds of SDN operations by operation type",
		},
		[]string{"operation_type"},
	)

	VnidNotFoundErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: SDNNamespace,
			Subsystem: SDNSubsystem,
			Name:      VnidNotFoundErrorsKey,
			Help:      "Number of VNID-not-found errors",
		},
	)

	// num stale OVS flows (flows that reference non-existent ports)
	// num vnids (in the master)
	// num netnamespaces (in the master)
	// iptables call time (in upstream kube)
	// iptables call failures (in upstream kube)
	// iptables num rules (in upstream kube)
)

var registerMetrics sync.Once

// Register all node metrics.
func RegisterMetrics() {
	registerMetrics.Do(func() {
		prometheus.MustRegister(OVSFlows)
		prometheus.MustRegister(ARPCacheAvailableEntries)
		prometheus.MustRegister(PodIPs)
		prometheus.MustRegister(PodOperationsErrors)
		prometheus.MustRegister(PodOperationsLatency)
		prometheus.MustRegister(VnidNotFoundErrors)
	})
}

// Gets the time since the specified start in microseconds.
func sinceInMicroseconds(start time.Time) float64 {
	return float64(time.Since(start) / time.Microsecond)
}

func gatherPeriodicMetrics(ovs ovs.Interface) {
	updateOVSMetrics(ovs)
	updateARPMetrics()
	updatePodIPMetrics()
}

func updateOVSMetrics(ovs ovs.Interface) {
	flows, err := ovs.DumpFlows("")
	if err == nil {
		OVSFlows.Set(float64(len(flows)))
	} else {
		utilruntime.HandleError(fmt.Errorf("failed to dump OVS flows for metrics: %v", err))
	}
}

func updateARPMetrics() {
	var used int
	data, err := ioutil.ReadFile("/proc/net/arp")
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("failed to read ARP entries for metrics: %v", err))
		return
	}
	lines := strings.Split(string(data), "\n")
	// Skip the header line
	used = len(lines) - 1

	// gc_thresh2 isn't the absolute max, but it's the level at which
	// garbage collection (and thus problems) could start.
	data, err = ioutil.ReadFile("/proc/sys/net/ipv4/neigh/default/gc_thresh2")
	if err != nil && os.IsNotExist(err) {
		// gc_thresh* may not exist in some cases; don't log an error
		return
	} else if err != nil {
		utilruntime.HandleError(fmt.Errorf("failed to read max ARP entries for metrics: %T %v", err, err))
		return
	}

	max, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err == nil {
		available := max - used
		if available < 0 {
			available = 0
		}
		ARPCacheAvailableEntries.Set(float64(available))
	} else {
		utilruntime.HandleError(fmt.Errorf("failed to parse max ARP entries %q for metrics: %T %v", data, err, err))
	}
}

func updatePodIPMetrics() {
	numAddrs := 0
	items, err := ioutil.ReadDir(hostLocalDataDir + "/openshift-sdn/")
	if err != nil && os.IsNotExist(err) {
		// Don't log an error if the directory doesn't exist (eg, no pods started yet)
		return
	} else if err != nil {
		utilruntime.HandleError(fmt.Errorf("failed to read pod IPs for metrics: %v", err))
	}

	for _, i := range items {
		if net.ParseIP(i.Name()) != nil {
			numAddrs++
		}
	}
	PodIPs.Set(float64(numAddrs))
}
