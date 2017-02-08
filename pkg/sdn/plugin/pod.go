package plugin

import (
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"sync"

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
	update(req *cniserver.PodRequest) (uint32, error)
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
	runningPods     map[string]*runningPod
	runningPodsLock sync.Mutex

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

	_, mcnet, _ := net.ParseCIDR("224.0.0.0/4")
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
					// Default route
					Dst: net.IPNet{
						IP:   net.IPv4zero,
						Mask: net.IPMask(net.IPv4zero),
					},
					GW: netutils.GenerateDefaultGateway(nodeNet),
				},
				{
					// Cluster network
					Dst: *clusterNetwork,
				},
				{
					// Multicast
					Dst: *mcnet,
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

func localMulticastOutputs(runningPods map[string]*runningPod, vnid uint32) string {
	var ofports []int
	for _, pod := range runningPods {
		if pod.vnid == vnid {
			ofports = append(ofports, pod.ofport)
		}
	}
	if len(ofports) == 0 {
		return ""
	}

	sort.Ints(ofports)
	outputs := ""
	for _, ofport := range ofports {
		if len(outputs) > 0 {
			outputs += ","
		}
		outputs += fmt.Sprintf("output:%d", ofport)
	}
	return outputs
}

func (m *podManager) updateLocalMulticastRulesWithLock(vnid uint32) {
	var outputs string
	otx := m.ovs.NewTransaction()
	if m.policy.GetMulticastEnabled(vnid) {
		outputs = localMulticastOutputs(m.runningPods, vnid)
		otx.AddFlow("table=110, reg0=%d, actions=goto_table:111", vnid)
	} else {
		otx.DeleteFlows("table=110, reg0=%d", vnid)
	}
	if len(outputs) > 0 {
		otx.AddFlow("table=120, priority=100, reg0=%d, actions=%s", vnid, outputs)
	} else {
		otx.DeleteFlows("table=120, reg0=%d", vnid)
	}
	if err := otx.EndTransaction(); err != nil {
		glog.Errorf("Error updating OVS multicast flows for VNID %d: %v", vnid, err)
	}
}

// Update multicast OVS rules for the given vnid
func (m *podManager) UpdateLocalMulticastRules(vnid uint32) {
	m.runningPodsLock.Lock()
	defer m.runningPodsLock.Unlock()
	m.updateLocalMulticastRulesWithLock(vnid)
}

// Process all CNI requests from the request queue serially.  Our OVS interaction
// and scripts currently cannot run in parallel, and doing so greatly complicates
// setup/teardown logic
func (m *podManager) processCNIRequests() {
	for request := range m.requests {
		glog.V(5).Infof("Processing pod network request %v", request)
		result := m.processRequest(request)
		glog.V(5).Infof("Processed pod network request %v, result %s err %v", request, string(result.Response), result.Err)
		request.Result <- result
	}
	panic("stopped processing CNI pod requests!")
}

func (m *podManager) processRequest(request *cniserver.PodRequest) *cniserver.PodResult {
	m.runningPodsLock.Lock()
	defer m.runningPodsLock.Unlock()

	pk := getPodKey(request)
	result := &cniserver.PodResult{}
	switch request.Command {
	case cniserver.CNI_ADD:
		ipamResult, runningPod, err := m.podHandler.setup(request)
		if ipamResult != nil {
			result.Response, err = json.Marshal(ipamResult)
			if result.Err == nil {
				m.runningPods[pk] = runningPod
				if m.ovs != nil {
					m.updateLocalMulticastRulesWithLock(runningPod.vnid)
				}
			}
		}
		if err != nil {
			result.Err = err
		}
	case cniserver.CNI_UPDATE:
		vnid, err := m.podHandler.update(request)
		if err == nil {
			if runningPod, exists := m.runningPods[pk]; exists {
				runningPod.vnid = vnid
			}
		}
		result.Err = err
	case cniserver.CNI_DEL:
		if runningPod, exists := m.runningPods[pk]; exists {
			delete(m.runningPods, pk)
			if m.ovs != nil {
				m.updateLocalMulticastRulesWithLock(runningPod.vnid)
			}
		}
		result.Err = m.podHandler.teardown(request)
	default:
		result.Err = fmt.Errorf("unhandled CNI request %v", request.Command)
	}
	return result
}
