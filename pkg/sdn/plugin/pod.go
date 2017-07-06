package plugin

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"

	"github.com/openshift/origin/pkg/sdn/plugin/cniserver"
	"github.com/openshift/origin/pkg/util/netutils"

	"github.com/golang/glog"

	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kubehostport "k8s.io/kubernetes/pkg/kubelet/network/hostport"

	cnitypes "github.com/containernetworking/cni/pkg/types"
)

type podHandler interface {
	setup(req *cniserver.PodRequest) (cnitypes.Result, *runningPod, error)
	update(req *cniserver.PodRequest) (uint32, error)
	teardown(req *cniserver.PodRequest) error
}

type runningPod struct {
	podPortMapping *kubehostport.PodPortMapping
	vnid           uint32
	ofport         int
}

type podManager struct {
	// Common stuff used for both live and testing code
	podHandler podHandler
	cniServer  *cniserver.CNIServer
	// Request queue for pod operations incoming from the CNIServer
	requests chan (*cniserver.PodRequest)
	// Tracks pod :: IP address for hostport and multicast handling
	runningPods     map[string]*runningPod
	runningPodsLock sync.Mutex

	// Live pod setup/teardown stuff not used in testing code
	kClient kclientset.Interface
	policy  osdnPolicy
	mtu     uint32
	ovs     *ovsController

	// Things only accessed through the processCNIRequests() goroutine
	// and thus can be set from Start()
	ipamConfig     []byte
	hostportSyncer kubehostport.HostportSyncer
}

// Creates a new live podManager; used by node code0
func newPodManager(kClient kclientset.Interface, policy osdnPolicy, mtu uint32, ovs *ovsController) *podManager {
	pm := newDefaultPodManager()
	pm.kClient = kClient
	pm.policy = policy
	pm.mtu = mtu
	pm.podHandler = pm
	pm.ovs = ovs
	return pm
}

// Creates a new basic podManager; used by testcases
func newDefaultPodManager() *podManager {
	return &podManager{
		runningPods: make(map[string]*runningPod),
		requests:    make(chan *cniserver.PodRequest, 20),
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
		CNIVersion string         `json:"cniVersion"`
		Name       string         `json:"name"`
		Type       string         `json:"type"`
		IPAM       *hostLocalIPAM `json:"ipam"`
	}

	_, mcnet, _ := net.ParseCIDR("224.0.0.0/4")
	return json.Marshal(&cniNetworkConfig{
		// TODO: update to 0.3.0 spec
		CNIVersion: "0.2.0",
		Name:       "openshift-sdn",
		Type:       "openshift-sdn",
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
func (m *podManager) Start(socketPath string, localSubnetCIDR string, clusterNetwork *net.IPNet) error {
	m.hostportSyncer = kubehostport.NewHostportSyncer()

	var err error
	if m.ipamConfig, err = getIPAMConfig(clusterNetwork, localSubnetCIDR); err != nil {
		return err
	}

	go m.processCNIRequests()

	m.cniServer = cniserver.NewCNIServer(socketPath)
	return m.cniServer.Start(m.handleCNIRequest)
}

// Returns a key for use with the runningPods map
func getPodKey(request *cniserver.PodRequest) string {
	return fmt.Sprintf("%s/%s", request.PodNamespace, request.PodName)
}

func (m *podManager) getPod(request *cniserver.PodRequest) *kubehostport.PodPortMapping {
	if pod := m.runningPods[getPodKey(request)]; pod != nil {
		return pod.podPortMapping
	}
	return nil
}

// Return a list of Kubernetes RunningPod objects for hostport operations
func (m *podManager) getRunningPods() []*kubehostport.PodPortMapping {
	pods := make([]*kubehostport.PodPortMapping, 0)
	for _, runningPod := range m.runningPods {
		pods = append(pods, runningPod.podPortMapping)
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

func (m *podManager) updateLocalMulticastRulesWithLock(vnid uint32) {
	var ofports []int
	enabled := m.policy.GetMulticastEnabled(vnid)
	if enabled {
		for _, pod := range m.runningPods {
			if pod.vnid == vnid {
				ofports = append(ofports, pod.ofport)
			}
		}
	}

	if err := m.ovs.UpdateLocalMulticastFlows(vnid, enabled, ofports); err != nil {
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
