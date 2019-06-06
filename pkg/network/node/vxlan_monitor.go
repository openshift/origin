// +build linux

package node

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"

	"github.com/openshift/origin/pkg/network/common"
	"github.com/openshift/origin/pkg/util/ovs"
)

// egressVXLANMonitor monitors the health of automatic egress IPs by periodically checking
// "ovs-ofctl dump-flows" output and noticing if the number of packets sent over VXLAN to
// an egress node has increased, but the number of packets received over VXLAN from the
// node hasn't. If that happens, it marks the node as being offline.
//
// Specifically, evm.poll() is called every defaultPollInterval and calls evm.check() to
// check the packet counts. If any nodes are seen to be potentially-offline, then
// evm.poll() will call evm.check() again after repollInterval, up to maxRetries more
// times. If the incoming packet count still hasn't increased at that point, then the node
// is considered to be offline. (The retries are needed because it's possible that we
// polled just a few milliseconds after some packets went out, in which case we obviously
// need to give the remote node more time to respond before declaring it offline.)
//
// When the monitor decides a node has gone offline, it alerts its owner via the updates
// channel, and then starts periodically pinging the node's SDN IP address, until the
// incoming packet count increases, at which point it marks the node online again.
//
// The fact that we (normally) use pod-to-egress traffic to do the monitoring rather than
// actively pinging the nodes means that if an egress node falls over while no one is
// using an egress IP on it, we won't notice the problem until someone does try to use it.
// So, eg, the first pod created in a namespace might spend several seconds trying to talk
// to a dead egress node before falling over to its backup egress IP.
type egressVXLANMonitor struct {
	sync.Mutex

	ovsif        ovs.Interface
	tracker      *common.EgressIPTracker
	updates      chan<- *egressVXLANNode
	pollInterval time.Duration

	monitorNodes map[string]*egressVXLANNode
	stop         chan struct{}
}

type egressVXLANNode struct {
	nodeIP  string
	offline bool

	in  uint64
	out uint64

	retries int
}

const (
	// See egressVXLANMonitor docs above for information about these
	defaultPollInterval = 5 * time.Second
	repollInterval      = time.Second
	maxRetries          = 2
)

func newEgressVXLANMonitor(ovsif ovs.Interface, tracker *common.EgressIPTracker, updates chan<- *egressVXLANNode) *egressVXLANMonitor {
	return &egressVXLANMonitor{
		ovsif:        ovsif,
		tracker:      tracker,
		updates:      updates,
		pollInterval: defaultPollInterval,
		monitorNodes: make(map[string]*egressVXLANNode),
	}
}

func (evm *egressVXLANMonitor) AddNode(nodeIP string) {
	evm.Lock()
	defer evm.Unlock()

	if evm.monitorNodes[nodeIP] != nil {
		return
	}
	glog.V(4).Infof("Monitoring node %s", nodeIP)

	evm.monitorNodes[nodeIP] = &egressVXLANNode{nodeIP: nodeIP}
	if len(evm.monitorNodes) == 1 && evm.pollInterval != 0 {
		evm.stop = make(chan struct{})
		go utilwait.PollUntil(evm.pollInterval, evm.poll, evm.stop)
	}
}

func (evm *egressVXLANMonitor) RemoveNode(nodeIP string) {
	evm.Lock()
	defer evm.Unlock()

	if evm.monitorNodes[nodeIP] == nil {
		return
	}
	glog.V(4).Infof("Unmonitoring node %s", nodeIP)

	delete(evm.monitorNodes, nodeIP)
	if len(evm.monitorNodes) == 0 && evm.stop != nil {
		close(evm.stop)
		evm.stop = nil
	}
}

func parseNPackets(of *ovs.OvsFlow) (uint64, error) {
	str, _ := of.FindField("n_packets")
	if str == nil {
		return 0, fmt.Errorf("no packet count")
	}
	nPackets, err := strconv.ParseUint(str.Value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("bad packet count: %v", err)
	}
	return nPackets, nil
}

// Assumes the mutex is held
func (evm *egressVXLANMonitor) check(retryOnly bool) bool {
	inFlows, err := evm.ovsif.DumpFlows("table=10")
	if err != nil {
		utilruntime.HandleError(err)
		return false
	}
	outFlows, err := evm.ovsif.DumpFlows("table=100")
	if err != nil {
		utilruntime.HandleError(err)
		return false
	}

	inTraffic := make(map[string]uint64)
	for _, flow := range inFlows {
		parsed, err := ovs.ParseFlow(ovs.ParseForDump, flow)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Error parsing VXLAN input flow: %v", err))
			continue
		}
		tunSrc, _ := parsed.FindField("tun_src")
		if tunSrc == nil {
			continue
		}
		if evm.monitorNodes[tunSrc.Value] == nil {
			continue
		}
		nPackets, err := parseNPackets(parsed)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Could not parse %q: %v", flow, err))
			continue
		}
		inTraffic[tunSrc.Value] = nPackets
	}

	outTraffic := make(map[string]uint64)
	for _, flow := range outFlows {
		parsed, err := ovs.ParseFlow(ovs.ParseForDump, flow)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Error parsing VXLAN output flow: %v", err))
			continue
		}
		tunDst := ""
		for _, act := range parsed.Actions {
			if act.Name == "set_field" && strings.HasSuffix(act.Value, "->tun_dst") {
				tunDst = strings.TrimSuffix(act.Value, "->tun_dst")
				break
			}
		}
		if tunDst == "" {
			continue
		}
		if evm.monitorNodes[tunDst] == nil {
			continue
		}
		nPackets, err := parseNPackets(parsed)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Could not parse %q: %v", flow, err))
			continue
		}
		outTraffic[tunDst] += nPackets
	}

	retry := false
	for _, node := range evm.monitorNodes {
		if retryOnly && node.retries == 0 {
			continue
		}

		in := inTraffic[node.nodeIP]
		out := outTraffic[node.nodeIP]

		// If `in` was missing from the OVS output then `out` should be missing
		// too, so both variables will be 0 and we won't end up doing anything
		// below. If `out` is missing but `in` isn't then that means we know about
		// the node but aren't currently routing egress traffic to it. If it is
		// currently marked offline, then we'll keep monitoring for it to come
		// back online (by watching `in`) but if it is currently marked online
		// then we *won't* notice if it goes offline.
		//
		// Note also that the code doesn't have to worry about n_packets
		// overflowing and rolling over; the worst that can happen is that if
		// `out` rolls over at the same time as the node goes offline then we
		// won't notice the node being offline until the next poll.

		if node.offline {
			if in > node.in {
				glog.Infof("Node %s is back online", node.nodeIP)
				node.offline = false
				evm.updates <- node
			} else if evm.tracker != nil {
				// We can ignore the return value because if the node responds
				// (with either success or "connection refused") we'll see it
				// in the OVS packet counts.
				go evm.tracker.Ping(node.nodeIP, defaultPollInterval)
			}
		} else {
			if out > node.out && in == node.in {
				node.retries++
				if evm.tracker != nil {
					// Start a ping probe as early as we can
					go evm.tracker.Ping(node.nodeIP, repollInterval)

					// For the first occurrence skip logging if we can
					// start pinging
					if node.retries == 1 {
						retry = true
						continue
					}
				}

				if node.retries > maxRetries {
					glog.Warningf("Node %s is offline", node.nodeIP)
					node.retries = 0
					node.offline = true
					evm.updates <- node
				} else {
					glog.V(2).Infof("Node %s may be offline... retrying", node.nodeIP)
					retry = true
					continue
				}
			} else {
				node.retries = 0
			}
		}

		node.in = in
		node.out = out
	}

	return retry
}

func (evm *egressVXLANMonitor) poll() (bool, error) {
	evm.Lock()
	defer evm.Unlock()

	retry := evm.check(false)
	for retry {
		time.Sleep(repollInterval)
		retry = evm.check(true)
	}
	return false, nil
}
