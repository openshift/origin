package plugin

import (
	"encoding/json"
	"fmt"
	"net"
	"sort"

	"github.com/openshift/origin/pkg/sdn/plugin/cniserver"
	"github.com/openshift/origin/pkg/util/netutils"
	"github.com/openshift/origin/pkg/util/ovs"

	"github.com/golang/glog"

	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	knetwork "k8s.io/kubernetes/pkg/kubelet/network"
	kubehostport "k8s.io/kubernetes/pkg/kubelet/network/hostport"

	cnitypes "github.com/containernetworking/cni/pkg/types"
)

type podHandler interface {
	setup(req *cniserver.PodRequest) (*cnitypes.Result, *runningPod, error)
	update(req *cniserver.PodRequest) (*runningPod, error)
	teardown(req *cniserver.PodRequest) error
}

type runningPod struct {
	activePod *kubehostport.ActivePod
	vnid      uint32
	ofport    int
}

type podManager struct {
	// Common stuff used for both live and testing code
	podHandler podHandler
	cniServer  *cniserver.CNIServer
	// Request queue for pod operations incoming from the CNIServer
	requests chan (*cniserver.PodRequest)
	// Tracks pod :: IP address for hostport handling
	runningPods map[string]*runningPod

	// Live pod setup/teardown stuff not used in testing code
	kClient         *kclientset.Clientset
	policy          osdnPolicy
	ipamConfig      []byte
	mtu             uint32
	hostportHandler kubehostport.HostportHandler
	host            knetwork.Host
	ovs             *ovs.Interface
}

// Creates a new live podManager; used by node code
func newPodManager(host knetwork.Host, localSubnetCIDR string, netInfo *NetworkInfo, kClient *kclientset.Clientset, policy osdnPolicy, mtu uint32, ovs *ovs.Interface) (*podManager, error) {
	pm := newDefaultPodManager(host)
	pm.kClient = kClient
	pm.policy = policy
	pm.mtu = mtu
	pm.hostportHandler = kubehostport.NewHostportHandler()
	pm.podHandler = pm
	pm.ovs = ovs

	var err error
	pm.ipamConfig, err = getIPAMConfig(netInfo.ClusterNetwork, localSubnetCIDR)
	if err != nil {
		return nil, err
	}

	return pm, nil
}

// Creates a new basic podManager; used by testcases
func newDefaultPodManager(host knetwork.Host) *podManager {
	return &podManager{
		runningPods: make(map[string]*runningPod),
		requests:    make(chan *cniserver.PodRequest, 20),
		host:        host,
	}
}

// Generates a CNI IPAM config from a given node cluster and local subnet that
// CNI 'host-local' IPAM plugin will use to create an IP address lease for the
// container
func getIPAMConfig(clusterNetwork *net.IPNet, localSubnet string) ([]byte, error) {
	nodeNet, err := cnitypes.ParseCIDR(localSubnet)
	if err != nil {
		return nil, fmt.Errorf("error parsing node network '%s': %v", localSubnet, err)
	}

	type hostLocalIPAM struct {
		Type   string           `json:"type"`
		Subnet cnitypes.IPNet   `json:"subnet"`
		Routes []cnitypes.Route `json:"routes"`
	}

	type cniNetworkConfig struct {
		Name string         `json:"name"`
		Type string         `json:"type"`
		IPAM *hostLocalIPAM `json:"ipam"`
	}

	mcaddr := net.ParseIP("224.0.0.0")
	return json.Marshal(&cniNetworkConfig{
		Name: "openshift-sdn",
		Type: "openshift-sdn",
		IPAM: &hostLocalIPAM{
			Type: "host-local",
			Subnet: cnitypes.IPNet{
				IP:   nodeNet.IP,
				Mask: nodeNet.Mask,
			},
			Routes: []cnitypes.Route{
				{
					Dst: net.IPNet{
						IP:   net.IPv4zero,
						Mask: net.IPMask(net.IPv4zero),
					},
					GW: netutils.GenerateDefaultGateway(nodeNet),
				},
				{Dst: *clusterNetwork},
				{
					// Multicast
					Dst: net.IPNet{
						IP:   mcaddr,
						Mask: net.IPMask(mcaddr),
					},
				},
			},
		},
	})
}

// Start the CNI server and start processing requests from it
func (m *podManager) Start(socketPath string) error {
	go m.processCNIRequests()

	m.cniServer = cniserver.NewCNIServer(socketPath)
	return m.cniServer.Start(m.handleCNIRequest)
}

// Returns a key for use with the runningPods map
func getPodKey(request *cniserver.PodRequest) string {
	return fmt.Sprintf("%s/%s", request.PodNamespace, request.PodName)
}

func (m *podManager) getPod(request *cniserver.PodRequest) *kubehostport.ActivePod {
	if pod := m.runningPods[getPodKey(request)]; pod != nil {
		return pod.activePod
	}
	return nil
}

// Return a list of Kubernetes RunningPod objects for hostport operations
func (m *podManager) getRunningPods() []*kubehostport.ActivePod {
	pods := make([]*kubehostport.ActivePod, 0)
	for _, runningPod := range m.runningPods {
		pods = append(pods, runningPod.activePod)
	}
	return pods
}

// Add a request to the podManager CNI request queue
func (m *podManager) addRequest(request *cniserver.PodRequest) {
	m.requests <- request
}

// Wait for and return the result of a pod request
func (m *podManager) waitRequest(request *cniserver.PodRequest) *cniserver.PodResult {
	return <-request.Result
}

// Enqueue incoming pod requests from the CNI server, wait on the result,
// and return that result to the CNI client
func (m *podManager) handleCNIRequest(request *cniserver.PodRequest) ([]byte, error) {
	glog.V(5).Infof("Dispatching pod network request %v", request)
	m.addRequest(request)
	result := m.waitRequest(request)
	glog.V(5).Infof("Returning pod network request %v, result %s err %v", request, string(result.Response), result.Err)
	return result.Response, result.Err
}

type runningPodsSlice []*runningPod

func (l runningPodsSlice) Len() int           { return len(l) }
func (l runningPodsSlice) Less(i, j int) bool { return l[i].ofport < l[j].ofport }
func (l runningPodsSlice) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }

// FIXME: instead of calculating all this ourselves, figure out a way to pass
// the old VNID through the Update() call (or get it from somewhere else).
func updateMulticastFlows(runningPods map[string]*runningPod, ovs *ovs.Interface, podKey string, changedPod *runningPod) error {
	// FIXME: prevents TestPodUpdate() from crashing. (We separately test this function anyway.)
	if ovs == nil {
		return nil
	}

	// Build map of pods by their VNID, excluding the changed pod
	podsByVNID := make(map[uint32]runningPodsSlice)
	for key, runningPod := range runningPods {
		if key != podKey {
			podsByVNID[runningPod.vnid] = append(podsByVNID[runningPod.vnid], runningPod)
		}
	}

	// Figure out what two VNIDs changed so we can update only those two flows
	changedVNIDs := make([]uint32, 0)
	oldPod, exists := runningPods[podKey]
	if changedPod != nil {
		podsByVNID[changedPod.vnid] = append(podsByVNID[changedPod.vnid], changedPod)
		changedVNIDs = append(changedVNIDs, changedPod.vnid)
		if exists {
			// VNID changed
			changedVNIDs = append(changedVNIDs, oldPod.vnid)
		}
	} else if exists {
		// Pod deleted
		changedVNIDs = append(changedVNIDs, oldPod.vnid)
	}

	if len(changedVNIDs) == 0 {
		// Shouldn't happen, but whatever
		return fmt.Errorf("Multicast update requested but not required!")
	}

	otx := ovs.NewTransaction()
	for _, vnid := range changedVNIDs {
		// Sort pod array to ensure consistent ordering for testcases and readability
		pods := podsByVNID[vnid]
		sort.Sort(pods)

		// build up list of ports on this VNID
		outputs := ""
		for _, pod := range pods {
			if len(outputs) > 0 {
				outputs += ","
			}
			outputs += fmt.Sprintf("output:%d", pod.ofport)
		}

		// Update or delete the flows for the vnid
		if len(outputs) > 0 {
			otx.AddFlow("table=120, priority=100, reg0=%d, actions=%s", vnid, outputs)
		} else {
			otx.DeleteFlows("table=120, reg0=%d", vnid)
		}
	}
	return otx.EndTransaction()
}

// Process all CNI requests from the request queue serially.  Our OVS interaction
// and scripts currently cannot run in parallel, and doing so greatly complicates
// setup/teardown logic
func (m *podManager) processCNIRequests() {
	for request := range m.requests {
		pk := getPodKey(request)

		var pod *runningPod
		var ipamResult *cnitypes.Result

		glog.V(5).Infof("Processing pod network request %v", request)
		result := &cniserver.PodResult{}
		switch request.Command {
		case cniserver.CNI_ADD:
			ipamResult, pod, result.Err = m.podHandler.setup(request)
			if ipamResult != nil {
				result.Response, result.Err = json.Marshal(ipamResult)
			}
		case cniserver.CNI_UPDATE:
			pod, result.Err = m.podHandler.update(request)
		case cniserver.CNI_DEL:
			result.Err = m.podHandler.teardown(request)
		default:
			result.Err = fmt.Errorf("unhandled CNI request %v", request.Command)
		}

		if result.Err == nil {
			if err := updateMulticastFlows(m.runningPods, m.ovs, pk, pod); err != nil {
				glog.Warningf("Failed to update multicast flows: %v", err)
			}
			if pod != nil {
				m.runningPods[pk] = pod
			} else {
				delete(m.runningPods, pk)
			}
		}

		glog.V(5).Infof("Processed pod network request %v, result %s err %v", request, string(result.Response), result.Err)
		request.Result <- result
	}
	panic("stopped processing CNI pod requests!")
}
