package plugin

import (
	"fmt"
	"net"
	osexec "os/exec"
	"strings"
	"time"

	log "github.com/golang/glog"

	"github.com/openshift/origin/pkg/sdn/plugin/cniserver"

	osclient "github.com/openshift/origin/pkg/client"
	osapi "github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/util/ipcmd"
	"github.com/openshift/origin/pkg/util/netutils"
	"github.com/openshift/origin/pkg/util/ovs"

	docker "github.com/fsouza/go-dockerclient"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/fields"
	knetwork "k8s.io/kubernetes/pkg/kubelet/network"
	"k8s.io/kubernetes/pkg/labels"
	kexec "k8s.io/kubernetes/pkg/util/exec"
	kubeutilnet "k8s.io/kubernetes/pkg/util/net"
	kwait "k8s.io/kubernetes/pkg/util/wait"
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

	RefVNID(vnid uint32)
	UnrefVNID(vnid uint32)
}

type OsdnNode struct {
	policy             osdnPolicy
	kClient            *kclientset.Clientset
	osClient           *osclient.Client
	ovs                *ovs.Interface
	networkInfo        *NetworkInfo
	podManager         *podManager
	localSubnetCIDR    string
	localIP            string
	hostName           string
	podNetworkReady    chan struct{}
	kubeletInitReady   chan struct{}
	iptablesSyncPeriod time.Duration
	mtu                uint32
	egressPolicies     map[uint32][]osapi.EgressNetworkPolicy

	host             knetwork.Host
	kubeletCniPlugin knetwork.NetworkPlugin

	clearLbr0IptablesRule bool
}

// Called by higher layers to create the plugin SDN node instance
func NewNodePlugin(pluginName string, osClient *osclient.Client, kClient *kclientset.Clientset, hostname string, selfIP string, iptablesSyncPeriod time.Duration, mtu uint32) (*OsdnNode, error) {
	var policy osdnPolicy
	var minOvsVersion string
	switch strings.ToLower(pluginName) {
	case osapi.SingleTenantPluginName:
		policy = NewSingleTenantPlugin()
	case osapi.MultiTenantPluginName:
		policy = NewMultiTenantPlugin()
	case osapi.NetworkPolicyPluginName:
		policy = NewNetworkPolicyPlugin()
		minOvsVersion = "2.5.0"
	default:
		// Not an OpenShift plugin
		return nil, nil
	}

	log.Infof("Initializing SDN node of type %q with configured hostname %q (IP %q), iptables sync period %q", pluginName, hostname, selfIP, iptablesSyncPeriod.String())
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

	ovsif, err := ovs.New(kexec.New(), BR, minOvsVersion)
	if err != nil {
		return nil, err
	}

	plugin := &OsdnNode{
		policy:             policy,
		kClient:            kClient,
		osClient:           osClient,
		ovs:                ovsif,
		localIP:            selfIP,
		hostName:           hostname,
		podNetworkReady:    make(chan struct{}),
		kubeletInitReady:   make(chan struct{}),
		iptablesSyncPeriod: iptablesSyncPeriod,
		mtu:                mtu,
		egressPolicies:     make(map[uint32][]osapi.EgressNetworkPolicy),
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

	// Wait until docker has restarted since kubelet will exit it docker isn't running
	dockerClient, err := docker.NewClientFromEnv()
	if err != nil {
		return fmt.Errorf("failed to get docker client: %v", err)
	}
	err = kwait.ExponentialBackoff(
		kwait.Backoff{
			Duration: 100 * time.Millisecond,
			Factor:   1.2,
			Steps:    6,
		},
		func() (bool, error) {
			if err := dockerClient.Ping(); err != nil {
				// wait longer
				return false, nil
			}
			return true, nil
		})
	if err != nil {
		return fmt.Errorf("failed to connect to docker after SDN cleanup restart: %v", err)
	}

	log.Infof("Cleaned up left-over openshift-sdn docker bridge and interfaces")

	return nil
}

func (node *OsdnNode) Start() error {
	var err error
	node.networkInfo, err = getNetworkInfo(node.osClient)
	if err != nil {
		return fmt.Errorf("failed to get network information: %v", err)
	}

	nodeIPTables := newNodeIPTables(node.networkInfo.ClusterNetwork.String(), node.iptablesSyncPeriod)
	if err = nodeIPTables.Setup(); err != nil {
		return fmt.Errorf("failed to set up iptables: %v", err)
	}

	node.localSubnetCIDR, err = node.getLocalSubnet()
	if err != nil {
		return err
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
	go kwait.Forever(node.watchServices, 0)

	// Wait for kubelet to init the plugin so we get a knetwork.Host
	log.V(5).Infof("Waiting for kubelet network plugin initialization")
	<-node.kubeletInitReady
	// Wait for kubelet itself to finish initializing
	kwait.PollInfinite(100*time.Millisecond,
		func() (bool, error) {
			if node.host.GetRuntime() == nil {
				return false, nil
			}
			return true, nil
		})

	log.V(5).Infof("Creating and initializing openshift-sdn pod manager")
	node.podManager, err = newPodManager(node.host, node.localSubnetCIDR, node.networkInfo, node.kClient, node.policy, node.mtu, node.ovs)
	if err != nil {
		return err
	}
	if err := node.podManager.Start(cniserver.CNIServerSocketPath); err != nil {
		return err
	}

	if networkChanged {
		var pods []kapi.Pod
		pods, err = node.GetLocalPods(kapi.NamespaceAll)
		if err != nil {
			return err
		}
		for _, p := range pods {
			err = node.UpdatePod(p)
			if err != nil {
				log.Warningf("Could not update pod %q: %s", p.Name, err)
				continue
			}
			if vnid, err := node.policy.GetVNID(p.Namespace); err == nil {
				node.policy.RefVNID(vnid)
			}
		}
	}

	log.V(5).Infof("openshift-sdn network plugin ready")
	node.markPodNetworkReady()

	return nil
}

// FIXME: this should eventually go into kubelet via a CNI UPDATE/CHANGE action
// See https://github.com/containernetworking/cni/issues/89
func (node *OsdnNode) UpdatePod(pod kapi.Pod) error {
	req := &cniserver.PodRequest{
		Command:      cniserver.CNI_UPDATE,
		PodNamespace: pod.Namespace,
		PodName:      pod.Name,
		ContainerId:  getPodContainerID(&pod),
		// netns is read from docker if needed, since we don't get it from kubelet
		Result: make(chan *cniserver.PodResult),
	}

	// Send request and wait for the result
	_, err := node.podManager.handleCNIRequest(req)
	return err
}

func (node *OsdnNode) GetLocalPods(namespace string) ([]kapi.Pod, error) {
	fieldSelector := fields.Set{"spec.nodeName": node.hostName}.AsSelector()
	opts := kapi.ListOptions{
		LabelSelector: labels.Everything(),
		FieldSelector: fieldSelector,
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

func (node *OsdnNode) markPodNetworkReady() {
	close(node.podNetworkReady)
}

func (node *OsdnNode) IsPodNetworkReady() error {
	select {
	case <-node.podNetworkReady:
		return nil
	default:
		return fmt.Errorf("SDN pod network is not ready")
	}
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
	services := make(map[string]*kapi.Service)
	RunEventQueue(node.kClient.CoreClient.RESTClient(), Services, func(delta cache.Delta) error {
		serv := delta.Object.(*kapi.Service)

		// Ignore headless services
		if !kapi.IsServiceIPSet(serv) {
			return nil
		}

		log.V(5).Infof("Watch %s event for Service %q", delta.Type, serv.ObjectMeta.Name)
		switch delta.Type {
		case cache.Sync, cache.Added, cache.Updated:
			oldsvc, exists := services[string(serv.UID)]
			if exists {
				if !isServiceChanged(oldsvc, serv) {
					break
				}
				node.DeleteServiceRules(oldsvc)
			}

			netid, err := node.policy.GetVNID(serv.Namespace)
			if err != nil {
				return fmt.Errorf("skipped adding service rules for serviceEvent: %v, Error: %v", delta.Type, err)
			}

			node.AddServiceRules(serv, netid)
			services[string(serv.UID)] = serv
			if !exists {
				node.policy.RefVNID(netid)
			}
		case cache.Deleted:
			delete(services, string(serv.UID))
			node.DeleteServiceRules(serv)

			netid, err := node.policy.GetVNID(serv.Namespace)
			if err == nil {
				node.policy.UnrefVNID(netid)
			}
		}
		return nil
	})
}
