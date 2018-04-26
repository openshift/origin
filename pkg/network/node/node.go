// +build linux

package node

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/vishvananda/netlink"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	kubeutilnet "k8s.io/apimachinery/pkg/util/net"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/record"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	kubeletapi "k8s.io/kubernetes/pkg/kubelet/apis/cri"
	kruntimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	knetwork "k8s.io/kubernetes/pkg/kubelet/network"
	ktypes "k8s.io/kubernetes/pkg/kubelet/types"
	"k8s.io/kubernetes/pkg/proxy/apis/kubeproxyconfig"
	kexec "k8s.io/utils/exec"

	"github.com/openshift/origin/pkg/network"
	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	"github.com/openshift/origin/pkg/network/common"
	networkinformers "github.com/openshift/origin/pkg/network/generated/informers/internalversion"
	networkclient "github.com/openshift/origin/pkg/network/generated/internalclientset"
	"github.com/openshift/origin/pkg/network/node/cniserver"
	"github.com/openshift/origin/pkg/util/netutils"
	"github.com/openshift/origin/pkg/util/ovs"
)

const (
	openshiftCNIFile = "80-openshift-network.conf"
	hostLocalDataDir = "/var/lib/cni/networks"
)

type osdnPolicy interface {
	Name() string
	Start(node *OsdnNode) error
	SupportsVNIDs() bool

	AddNetNamespace(netns *networkapi.NetNamespace)
	UpdateNetNamespace(netns *networkapi.NetNamespace, oldNetID uint32)
	DeleteNetNamespace(netns *networkapi.NetNamespace)

	GetVNID(namespace string) (uint32, error)
	GetNamespaces(vnid uint32) []string
	GetMulticastEnabled(vnid uint32) bool

	EnsureVNIDRules(vnid uint32)
	SyncVNIDRules()
}

type OsdnNodeConfig struct {
	PluginName      string
	Hostname        string
	SelfIP          string
	RuntimeEndpoint string
	MTU             uint32
	EnableHostports bool
	CNIBinDir       string
	CNIConfDir      string

	NetworkClient networkclient.Interface
	KClient       kclientset.Interface
	Recorder      record.EventRecorder

	KubeInformers    kinternalinformers.SharedInformerFactory
	NetworkInformers networkinformers.SharedInformerFactory

	IPTablesSyncPeriod time.Duration
	ProxyMode          kubeproxyconfig.ProxyMode
	MasqueradeBit      *int32
}

type OsdnNode struct {
	policy             osdnPolicy
	kClient            kclientset.Interface
	networkClient      networkclient.Interface
	recorder           record.EventRecorder
	oc                 *ovsController
	networkInfo        *common.NetworkInfo
	podManager         *podManager
	localSubnetCIDR    string
	localIP            string
	hostName           string
	useConnTrack       bool
	iptablesSyncPeriod time.Duration
	mtu                uint32
	cniDirPath         string

	// Synchronizes operations on egressPolicies
	egressPoliciesLock sync.Mutex
	egressPolicies     map[uint32][]networkapi.EgressNetworkPolicy
	egressDNS          *common.EgressDNS

	host             knetwork.Host
	kubeletCniPlugin knetwork.NetworkPlugin

	kubeInformers    kinternalinformers.SharedInformerFactory
	networkInformers networkinformers.SharedInformerFactory

	// Holds runtime endpoint shim to make SDN <-> runtime communication
	runtimeEndpoint       string
	runtimeRequestTimeout time.Duration
	runtimeService        kubeletapi.RuntimeService

	egressIP *egressIPWatcher
}

// Called by higher layers to create the plugin SDN node instance
func New(c *OsdnNodeConfig) (*OsdnNode, error) {
	var policy osdnPolicy
	var pluginId int
	var minOvsVersion string
	var useConnTrack bool
	switch strings.ToLower(c.PluginName) {
	case network.SingleTenantPluginName:
		policy = NewSingleTenantPlugin()
		pluginId = 0
	case network.MultiTenantPluginName:
		policy = NewMultiTenantPlugin()
		pluginId = 1
	case network.NetworkPolicyPluginName:
		policy = NewNetworkPolicyPlugin()
		pluginId = 2
		minOvsVersion = "2.6.0"
		useConnTrack = true
	default:
		// Not an OpenShift plugin
		return nil, nil
	}
	glog.Infof("Initializing SDN node of type %q with configured hostname %q (IP %q), iptables sync period %q", c.PluginName, c.Hostname, c.SelfIP, c.IPTablesSyncPeriod.String())

	if useConnTrack && c.ProxyMode != kubeproxyconfig.ProxyModeIPTables {
		return nil, fmt.Errorf("%q plugin is not compatible with proxy-mode %q", c.PluginName, c.ProxyMode)
	}

	// If our CNI config file exists, remove it so that kubelet doesn't think
	// we're ready yet
	os.Remove(filepath.Join(c.CNIConfDir, openshiftCNIFile))

	if err := c.setNodeIP(); err != nil {
		return nil, err
	}

	ovsif, err := ovs.New(kexec.New(), Br0, minOvsVersion)
	if err != nil {
		return nil, err
	}
	oc := NewOVSController(ovsif, pluginId, useConnTrack, c.SelfIP)

	plugin := &OsdnNode{
		policy:             policy,
		kClient:            c.KClient,
		networkClient:      c.NetworkClient,
		recorder:           c.Recorder,
		oc:                 oc,
		podManager:         newPodManager(c.KClient, policy, c.MTU, c.CNIBinDir, oc, c.EnableHostports),
		localIP:            c.SelfIP,
		hostName:           c.Hostname,
		useConnTrack:       useConnTrack,
		iptablesSyncPeriod: c.IPTablesSyncPeriod,
		mtu:                c.MTU,
		egressPolicies:     make(map[uint32][]networkapi.EgressNetworkPolicy),
		egressDNS:          common.NewEgressDNS(),
		kubeInformers:      c.KubeInformers,
		networkInformers:   c.NetworkInformers,
		egressIP:           newEgressIPWatcher(oc, c.SelfIP, c.MasqueradeBit),
		cniDirPath:         c.CNIConfDir,

		runtimeEndpoint: c.RuntimeEndpoint,
		// 2 minutes is the current default value used in kubelet
		runtimeRequestTimeout: 2 * time.Minute,
		// populated on demand
		runtimeService: nil,
	}

	RegisterMetrics()

	return plugin, nil
}

// Set node IP if required
func (c *OsdnNodeConfig) setNodeIP() error {
	if len(c.Hostname) == 0 {
		output, err := kexec.New().Command("uname", "-n").CombinedOutput()
		if err != nil {
			return err
		}
		c.Hostname = strings.TrimSpace(string(output))
		glog.Infof("Resolved hostname to %q", c.Hostname)
	}

	if len(c.SelfIP) == 0 {
		var err error
		c.SelfIP, err = netutils.GetNodeIP(c.Hostname)
		if err != nil {
			glog.V(5).Infof("Failed to determine node address from hostname %s; using default interface (%v)", c.Hostname, err)
			var defaultIP net.IP
			defaultIP, err = kubeutilnet.ChooseHostInterface()
			if err != nil {
				return err
			}
			c.SelfIP = defaultIP.String()
			glog.Infof("Resolved IP address to %q", c.SelfIP)
		}
	}

	if _, _, err := GetLinkDetails(c.SelfIP); err != nil {
		if err == ErrorNetworkInterfaceNotFound {
			err = fmt.Errorf("node IP %q is not a local/private address (hostname %q)", c.SelfIP, c.Hostname)
		}
		utilruntime.HandleError(fmt.Errorf("Unable to find network interface for node IP; some features will not work! (%v)", err))
	}

	return nil
}

var (
	ErrorNetworkInterfaceNotFound = fmt.Errorf("could not find network interface")
)

func GetLinkDetails(ip string) (netlink.Link, *net.IPNet, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, nil, err
	}

	for _, link := range links {
		addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
		if err != nil {
			glog.Warningf("Could not get addresses of interface %q: %v", link.Attrs().Name, err)
			continue
		}

		for _, addr := range addrs {
			if addr.IP.String() == ip {
				_, ipNet, err := net.ParseCIDR(addr.IPNet.String())
				if err != nil {
					return nil, nil, fmt.Errorf("could not parse CIDR network from address %q: %v", ip, err)
				}
				return link, ipNet, nil
			}
		}
	}

	return nil, nil, ErrorNetworkInterfaceNotFound
}

func (node *OsdnNode) killUpdateFailedPods(pods []kapi.Pod) error {
	for _, pod := range pods {
		// Get the sandbox ID for this pod from the runtime
		filter := &kruntimeapi.PodSandboxFilter{
			LabelSelector: map[string]string{ktypes.KubernetesPodUIDLabel: string(pod.UID)},
		}
		sandboxID, err := node.getPodSandboxID(filter)
		if err != nil {
			return err
		}

		// Make an event that the pod is going to be killed
		podRef := &v1.ObjectReference{Kind: "Pod", Name: pod.Name, Namespace: pod.Namespace, UID: pod.UID}
		node.recorder.Eventf(podRef, v1.EventTypeWarning, "NetworkFailed", "The pod's network interface has been lost and the pod will be stopped.")

		glog.V(5).Infof("Killing pod '%s/%s' sandbox due to failed restart", pod.Namespace, pod.Name)
		if err := node.runtimeService.StopPodSandbox(sandboxID); err != nil {
			glog.Warningf("Failed to kill pod '%s/%s' sandbox: %v", pod.Namespace, pod.Name, err)
		}
	}
	return nil
}

func (node *OsdnNode) Start() error {
	glog.V(2).Infof("Starting openshift-sdn network plugin")

	if err := validateNetworkPluginName(node.networkClient, node.policy.Name()); err != nil {
		return fmt.Errorf("failed to validate network configuration: %v", err)
	}

	var err error
	node.networkInfo, err = common.GetNetworkInfo(node.networkClient)
	if err != nil {
		return fmt.Errorf("failed to get network information: %v", err)
	}

	hostIPNets, _, err := netutils.GetHostIPNetworks([]string{Tun0})
	if err != nil {
		return fmt.Errorf("failed to get host network information: %v", err)
	}
	if err := node.networkInfo.CheckHostNetworks(hostIPNets); err != nil {
		// checkHostNetworks() errors *should* be fatal, but we didn't used to check this, and we can't break (mostly-)working nodes on upgrade.
		utilruntime.HandleError(fmt.Errorf("Local networks conflict with SDN; this will eventually cause problems: %v", err))
	}

	node.localSubnetCIDR, err = node.getLocalSubnet()
	if err != nil {
		return err
	}

	var cidrList []string
	for _, cn := range node.networkInfo.ClusterNetworks {
		cidrList = append(cidrList, cn.ClusterCIDR.String())
	}
	nodeIPTables := newNodeIPTables(cidrList, node.iptablesSyncPeriod, !node.useConnTrack)

	if err = nodeIPTables.Setup(); err != nil {
		return fmt.Errorf("failed to set up iptables: %v", err)
	}

	networkChanged, err := node.SetupSDN()
	if err != nil {
		return fmt.Errorf("node SDN setup failed: %v", err)
	}

	hsw := newHostSubnetWatcher(node.oc, node.localIP, node.networkInfo)
	hsw.Start(node.networkInformers)

	if err = node.policy.Start(node); err != nil {
		return err
	}
	if node.policy.SupportsVNIDs() {
		if err := node.SetupEgressNetworkPolicy(); err != nil {
			return err
		}
		if err := node.egressIP.Start(node.networkInformers, nodeIPTables); err != nil {
			return err
		}
	}
	if !node.useConnTrack {
		node.watchServices()
	}

	glog.V(2).Infof("Starting openshift-sdn pod manager")
	if err := node.podManager.Start(cniserver.CNIServerRunDir, node.localSubnetCIDR, node.networkInfo.ClusterNetworks); err != nil {
		return err
	}

	if networkChanged {
		var pods, podsToKill []kapi.Pod

		pods, err = node.GetLocalPods(metav1.NamespaceAll)
		if err != nil {
			return err
		}
		for _, p := range pods {
			// Ignore HostNetwork pods since they don't go through OVS
			if p.Spec.SecurityContext != nil && p.Spec.SecurityContext.HostNetwork {
				continue
			}
			if err := node.UpdatePod(p); err != nil {
				glog.Warningf("will restart pod '%s/%s' due to update failure on restart: %s", p.Namespace, p.Name, err)
				podsToKill = append(podsToKill, p)
			} else if vnid, err := node.policy.GetVNID(p.Namespace); err == nil {
				node.policy.EnsureVNIDRules(vnid)
			}
		}

		// Kill pods we couldn't recover; they will get restarted and then
		// we'll be able to set them up correctly
		if len(podsToKill) > 0 {
			if err := node.killUpdateFailedPods(podsToKill); err != nil {
				glog.Warningf("failed to restart pods that failed to update at startup: %v", err)
			}
		}
	}

	if err := os.MkdirAll(node.cniDirPath, 0755); err != nil {
		return err
	}

	go kwait.Forever(node.policy.SyncVNIDRules, time.Hour)
	go kwait.Forever(func() {
		gatherPeriodicMetrics(node.oc.ovs)
	}, time.Minute*2)

	glog.V(2).Infof("openshift-sdn network plugin ready")

	// Make an event that openshift-sdn started
	node.recorder.Eventf(&v1.ObjectReference{Kind: "Node", Name: node.hostName}, v1.EventTypeNormal, "Starting", "Starting openshift-sdn.")

	// Write our CNI config file out to disk to signal to kubelet that
	// our network plugin is ready
	return ioutil.WriteFile(filepath.Join(node.cniDirPath, openshiftCNIFile), []byte(`
{
  "cniVersion": "0.2.0",
  "name": "openshift-sdn",
  "type": "openshift-sdn"
}
`), 0644)
}

// FIXME: this should eventually go into kubelet via a CNI UPDATE/CHANGE action
// See https://github.com/containernetworking/cni/issues/89
func (node *OsdnNode) UpdatePod(pod kapi.Pod) error {
	filter := &kruntimeapi.PodSandboxFilter{
		LabelSelector: map[string]string{ktypes.KubernetesPodUIDLabel: string(pod.UID)},
	}
	sandboxID, err := node.getPodSandboxID(filter)
	if err != nil {
		return err
	}

	req := &cniserver.PodRequest{
		Command:      cniserver.CNI_UPDATE,
		PodNamespace: pod.Namespace,
		PodName:      pod.Name,
		SandboxID:    sandboxID,
		Result:       make(chan *cniserver.PodResult),
	}

	// Send request and wait for the result
	_, err = node.podManager.handleCNIRequest(req)
	return err
}

func (node *OsdnNode) GetLocalPods(namespace string) ([]kapi.Pod, error) {
	fieldSelector := fields.Set{"spec.nodeName": node.hostName}.AsSelector()
	opts := metav1.ListOptions{
		LabelSelector: labels.Everything().String(),
		FieldSelector: fieldSelector.String(),
	}
	podList, err := node.kClient.Core().Pods(namespace).List(opts)
	if err != nil {
		return nil, err
	}

	// Filter running pods
	pods := make([]kapi.Pod, 0, len(podList.Items))
	for _, pod := range podList.Items {
		if pod.Status.Phase == kapi.PodRunning {
			pods = append(pods, pod)
		}
	}
	return pods, nil
}

func isServiceChanged(oldsvc, newsvc *kapi.Service) bool {
	if len(oldsvc.Spec.Ports) == len(newsvc.Spec.Ports) {
		for i := range oldsvc.Spec.Ports {
			if oldsvc.Spec.Ports[i].Protocol != newsvc.Spec.Ports[i].Protocol ||
				oldsvc.Spec.Ports[i].Port != newsvc.Spec.Ports[i].Port {
				return true
			}
		}
		return false
	}
	return true
}

func (node *OsdnNode) watchServices() {
	funcs := common.InformerFuncs(&kapi.Service{}, node.handleAddOrUpdateService, node.handleDeleteService)
	node.kubeInformers.Core().InternalVersion().Services().Informer().AddEventHandler(funcs)
}

func (node *OsdnNode) handleAddOrUpdateService(obj, oldObj interface{}, eventType watch.EventType) {
	serv := obj.(*kapi.Service)
	// Ignore headless/external services
	if !kapihelper.IsServiceIPSet(serv) {
		return
	}

	glog.V(5).Infof("Watch %s event for Service %q", eventType, serv.Name)
	oldServ, exists := oldObj.(*kapi.Service)
	if exists {
		if !isServiceChanged(oldServ, serv) {
			return
		}
		node.DeleteServiceRules(oldServ)
	}

	netid, err := node.policy.GetVNID(serv.Namespace)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Skipped adding service rules for serviceEvent: %v, Error: %v", eventType, err))
		return
	}

	node.AddServiceRules(serv, netid)
	node.policy.EnsureVNIDRules(netid)
}

func (node *OsdnNode) handleDeleteService(obj interface{}) {
	serv := obj.(*kapi.Service)
	// Ignore headless/external services
	if !kapihelper.IsServiceIPSet(serv) {
		return
	}

	glog.V(5).Infof("Watch %s event for Service %q", watch.Deleted, serv.Name)
	node.DeleteServiceRules(serv)
}

func validateNetworkPluginName(networkClient networkclient.Interface, pluginName string) error {
	// Detect any plugin mismatches between node and master
	clusterNetwork, err := networkClient.Network().ClusterNetworks().Get(networkapi.ClusterNetworkDefault, metav1.GetOptions{})
	switch {
	case errors.IsNotFound(err):
		return fmt.Errorf("master has not created a default cluster network, network plugin %q can not start", pluginName)
	case err != nil:
		return fmt.Errorf("cannot fetch %q cluster network: %v", networkapi.ClusterNetworkDefault, err)
	}
	if clusterNetwork.PluginName != strings.ToLower(pluginName) {
		return fmt.Errorf("detected network plugin mismatch between OpenShift node(%q) and master(%q)", pluginName, clusterNetwork.PluginName)
	}
	return nil
}
