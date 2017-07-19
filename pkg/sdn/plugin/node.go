package plugin

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	log "github.com/golang/glog"

	"github.com/openshift/origin/pkg/sdn/plugin/cniserver"

	osclient "github.com/openshift/origin/pkg/client"
	osapi "github.com/openshift/origin/pkg/sdn/apis/network"
	"github.com/openshift/origin/pkg/util/ipcmd"
	"github.com/openshift/origin/pkg/util/netutils"
	"github.com/openshift/origin/pkg/util/ovs"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	kubeutilnet "k8s.io/apimachinery/pkg/util/net"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/api"
	kapihelper "k8s.io/kubernetes/pkg/api/helper"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	kubeletapi "k8s.io/kubernetes/pkg/kubelet/apis/cri"
	kruntimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
	dockertools "k8s.io/kubernetes/pkg/kubelet/dockershim/libdocker"
	knetwork "k8s.io/kubernetes/pkg/kubelet/network"
	ktypes "k8s.io/kubernetes/pkg/kubelet/types"
	kexec "k8s.io/kubernetes/pkg/util/exec"
)

const (
	cniDirPath       = "/etc/cni/net.d"
	openshiftCNIFile = "80-openshift-sdn.conf"
)

type osdnPolicy interface {
	Name() string
	Start(node *OsdnNode) error

	AddNetNamespace(netns *osapi.NetNamespace)
	UpdateNetNamespace(netns *osapi.NetNamespace, oldNetID uint32)
	DeleteNetNamespace(netns *osapi.NetNamespace)

	GetVNID(namespace string) (uint32, error)
	GetNamespaces(vnid uint32) []string
	GetMulticastEnabled(vnid uint32) bool

	EnsureVNIDRules(vnid uint32)
	SyncVNIDRules()
}

type OsdnNode struct {
	policy             osdnPolicy
	kClient            kclientset.Interface
	osClient           *osclient.Client
	oc                 *ovsController
	networkInfo        *NetworkInfo
	podManager         *podManager
	localSubnetCIDR    string
	localIP            string
	hostName           string
	useConnTrack       bool
	iptablesSyncPeriod time.Duration
	mtu                uint32

	// Synchronizes operations on egressPolicies
	egressPoliciesLock sync.Mutex
	egressPolicies     map[uint32][]osapi.EgressNetworkPolicy
	egressDNS          *EgressDNS

	host             knetwork.Host
	kubeletCniPlugin knetwork.NetworkPlugin

	clearLbr0IptablesRule bool

	kubeInformers kinternalinformers.SharedInformerFactory

	// Holds runtime endpoint shim to make SDN <-> runtime communication
	runtimeEndpoint       string
	runtimeRequestTimeout time.Duration
	runtimeService        kubeletapi.RuntimeService
}

// Called by higher layers to create the plugin SDN node instance
func NewNodePlugin(pluginName string, osClient *osclient.Client, kClient kclientset.Interface, kubeInformers kinternalinformers.SharedInformerFactory,
	hostname string, selfIP string, mtu uint32, proxyConfig componentconfig.KubeProxyConfiguration, runtimeEndpoint string) (*OsdnNode, error) {
	var policy osdnPolicy
	var pluginId int
	var minOvsVersion string
	var useConnTrack bool
	switch strings.ToLower(pluginName) {
	case osapi.SingleTenantPluginName:
		policy = NewSingleTenantPlugin()
		pluginId = 0
	case osapi.MultiTenantPluginName:
		policy = NewMultiTenantPlugin()
		pluginId = 1
	case osapi.NetworkPolicyPluginName:
		policy = NewNetworkPolicyPlugin()
		pluginId = 2
		minOvsVersion = "2.6.0"
		useConnTrack = true
	default:
		// Not an OpenShift plugin
		return nil, nil
	}

	// If our CNI config file exists, remove it so that kubelet doesn't think
	// we're ready yet
	os.Remove(filepath.Join(cniDirPath, openshiftCNIFile))

	log.Infof("Initializing SDN node of type %q with configured hostname %q (IP %q), iptables sync period %q", pluginName, hostname, selfIP, proxyConfig.IPTables.SyncPeriod.Duration.String())
	if hostname == "" {
		output, err := kexec.New().Command("uname", "-n").CombinedOutput()
		if err != nil {
			return nil, err
		}
		hostname = strings.TrimSpace(string(output))
		log.Infof("Resolved hostname to %q", hostname)
	}
	if selfIP == "" {
		var err error
		selfIP, err = netutils.GetNodeIP(hostname)
		if err != nil {
			log.V(5).Infof("Failed to determine node address from hostname %s; using default interface (%v)", hostname, err)
			var defaultIP net.IP
			defaultIP, err = kubeutilnet.ChooseHostInterface()
			if err != nil {
				return nil, err
			}
			selfIP = defaultIP.String()
			log.Infof("Resolved IP address to %q", selfIP)
		}
	}

	if useConnTrack && proxyConfig.Mode != componentconfig.ProxyModeIPTables {
		return nil, fmt.Errorf("%q plugin is not compatible with proxy-mode %q", pluginName, proxyConfig.Mode)
	}

	ovsif, err := ovs.New(kexec.New(), BR, minOvsVersion)
	if err != nil {
		return nil, err
	}
	oc := NewOVSController(ovsif, pluginId, useConnTrack)

	plugin := &OsdnNode{
		policy:             policy,
		kClient:            kClient,
		osClient:           osClient,
		oc:                 oc,
		podManager:         newPodManager(kClient, policy, mtu, oc),
		localIP:            selfIP,
		hostName:           hostname,
		useConnTrack:       useConnTrack,
		iptablesSyncPeriod: proxyConfig.IPTables.SyncPeriod.Duration,
		mtu:                mtu,
		egressPolicies:     make(map[uint32][]osapi.EgressNetworkPolicy),
		egressDNS:          NewEgressDNS(),
		kubeInformers:      kubeInformers,

		runtimeEndpoint: runtimeEndpoint,
		// 2 minutes is the current default value used in kubelet
		runtimeRequestTimeout: 2 * time.Minute,
		// populated on demand
		runtimeService: nil,
	}

	if err := plugin.dockerPreCNICleanup(); err != nil {
		return nil, err
	}

	return plugin, nil
}

// Detect whether we are upgrading from a pre-CNI openshift and clean up
// interfaces and iptables rules that are no longer required
func (node *OsdnNode) dockerPreCNICleanup() error {
	exec := kexec.New()
	itx := ipcmd.NewTransaction(exec, "lbr0")
	itx.SetLink("down")
	if err := itx.EndTransaction(); err != nil {
		// no cleanup required
		return nil
	}

	node.clearLbr0IptablesRule = true

	// Restart docker to kill old pods and make it use docker0 again.
	// "systemctl restart" will bail out (unnecessarily) in the
	// OpenShift-in-a-container case, so we work around that by sending
	// the messages by hand.
	if _, err := osexec.Command("dbus-send", "--system", "--print-reply", "--reply-timeout=2000", "--type=method_call", "--dest=org.freedesktop.systemd1", "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager.Reload").CombinedOutput(); err != nil {
		log.Error(err)
	}
	if _, err := osexec.Command("dbus-send", "--system", "--print-reply", "--reply-timeout=2000", "--type=method_call", "--dest=org.freedesktop.systemd1", "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager.RestartUnit", "string:'docker.service' string:'replace'").CombinedOutput(); err != nil {
		log.Error(err)
	}

	// Delete pre-CNI interfaces
	for _, intf := range []string{"lbr0", "vovsbr", "vlinuxbr"} {
		itx := ipcmd.NewTransaction(exec, intf)
		itx.DeleteLink()
		itx.IgnoreError()
		itx.EndTransaction()
	}

	// Wait until docker has restarted since kubelet will exit if docker isn't running
	if _, err := ensureDockerClient(); err != nil {
		return err
	}

	log.Infof("Cleaned up left-over openshift-sdn docker bridge and interfaces")

	return nil
}

func ensureDockerClient() (dockertools.Interface, error) {
	endpoint := os.Getenv("DOCKER_HOST")
	if endpoint == "" {
		endpoint = "unix:///var/run/docker.sock"
	}
	dockerClient := dockertools.ConnectToDockerOrDie(endpoint, time.Minute, time.Minute)

	// Wait until docker has restarted since kubelet will exit it docker isn't running
	err := kwait.ExponentialBackoff(
		kwait.Backoff{
			Duration: 100 * time.Millisecond,
			Factor:   1.2,
			Steps:    6,
		},
		func() (bool, error) {
			if _, err := dockerClient.Version(); err != nil {
				// wait longer
				return false, nil
			}
			return true, nil
		})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to docker: %v", err)
	}
	return dockerClient, nil
}

func (node *OsdnNode) Start() error {
	var err error
	node.networkInfo, err = getNetworkInfo(node.osClient)
	if err != nil {
		return fmt.Errorf("failed to get network information: %v", err)
	}

	hostIPNets, _, err := netutils.GetHostIPNetworks([]string{TUN})
	if err != nil {
		return fmt.Errorf("failed to get host network information: %v", err)
	}
	if err := node.networkInfo.checkHostNetworks(hostIPNets); err != nil {
		// checkHostNetworks() errors *should* be fatal, but we didn't used to check this, and we can't break (mostly-)working nodes on upgrade.
		log.Errorf("Local networks conflict with SDN; this will eventually cause problems: %v", err)
	}

	node.localSubnetCIDR, err = node.getLocalSubnet()
	if err != nil {
		return err
	}

	nodeIPTables := newNodeIPTables(node.networkInfo.ClusterNetwork.String(), node.iptablesSyncPeriod, !node.useConnTrack)
	if err = nodeIPTables.Setup(); err != nil {
		return fmt.Errorf("failed to set up iptables: %v", err)
	}

	networkChanged, err := node.SetupSDN()
	if err != nil {
		return err
	}

	err = node.SubnetStartNode()
	if err != nil {
		return err
	}

	if err = node.policy.Start(node); err != nil {
		return err
	}
	if !node.useConnTrack {
		node.watchServices()
	}

	log.V(5).Infof("Starting openshift-sdn pod manager")
	if err := node.podManager.Start(cniserver.CNIServerSocketPath, node.localSubnetCIDR, node.networkInfo.ClusterNetwork); err != nil {
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
				log.Warningf("will restart pod '%s/%s' due to update failure on restart: %s", p.Namespace, p.Name, err)
				podsToKill = append(podsToKill, p)
			} else if vnid, err := node.policy.GetVNID(p.Namespace); err == nil {
				node.policy.EnsureVNIDRules(vnid)
			}
		}

		// Kill pods we couldn't recover; they will get restarted and then
		// we'll be able to set them up correctly
		if len(podsToKill) > 0 {
			docker, err := ensureDockerClient()
			if err != nil {
				log.Warningf("failed to get docker client: %v", err)
			} else if err := killUpdateFailedPods(docker, podsToKill); err != nil {
				log.Warningf("failed to restart pods that failed to update at startup: %v", err)
			}
		}
	}

	if err := os.MkdirAll(cniDirPath, 0755); err != nil {
		return err
	}

	go kwait.Forever(node.policy.SyncVNIDRules, time.Hour)

	log.V(5).Infof("openshift-sdn network plugin ready")

	// Write our CNI config file out to disk to signal to kubelet that
	// our network plugin is ready
	return ioutil.WriteFile(filepath.Join(cniDirPath, openshiftCNIFile), []byte(`
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
	RegisterSharedInformerEventHandlers(node.kubeInformers,
		node.handleAddOrUpdateService, node.handleDeleteService, Services)
}

func (node *OsdnNode) handleAddOrUpdateService(obj, oldObj interface{}, eventType watch.EventType) {
	serv := obj.(*kapi.Service)
	// Ignore headless services
	if !kapihelper.IsServiceIPSet(serv) {
		return
	}

	log.V(5).Infof("Watch %s event for Service %q", eventType, serv.Name)
	oldServ, exists := oldObj.(*kapi.Service)
	if exists {
		if !isServiceChanged(oldServ, serv) {
			return
		}
		node.DeleteServiceRules(oldServ)
	}

	netid, err := node.policy.GetVNID(serv.Namespace)
	if err != nil {
		log.Errorf("Skipped adding service rules for serviceEvent: %v, Error: %v", eventType, err)
		return
	}

	node.AddServiceRules(serv, netid)
	node.policy.EnsureVNIDRules(netid)
}

func (node *OsdnNode) handleDeleteService(obj interface{}) {
	serv := obj.(*kapi.Service)
	log.V(5).Infof("Watch %s event for Service %q", watch.Deleted, serv.Name)
	node.DeleteServiceRules(serv)
}
