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
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	knetwork "k8s.io/kubernetes/pkg/kubelet/network"
	"k8s.io/kubernetes/pkg/labels"
	kexec "k8s.io/kubernetes/pkg/util/exec"
	kubeutilnet "k8s.io/kubernetes/pkg/util/net"
	kwait "k8s.io/kubernetes/pkg/util/wait"
)

type OsdnNode struct {
	multitenant        bool
	kClient            *kclient.Client
	osClient           *osclient.Client
	ovs                *ovs.Interface
	networkInfo        *NetworkInfo
	podManager         *podManager
	localSubnetCIDR    string
	localIP            string
	hostName           string
	podNetworkReady    chan struct{}
	kubeletInitReady   chan struct{}
	vnids              *nodeVNIDMap
	iptablesSyncPeriod time.Duration
	mtu                uint32
	egressPolicies     map[uint32][]*osapi.EgressNetworkPolicy

	host             knetwork.Host
	kubeletCniPlugin knetwork.NetworkPlugin

	clearLbr0IptablesRule bool
}

// Called by higher layers to create the plugin SDN node instance
func NewNodePlugin(pluginName string, osClient *osclient.Client, kClient *kclient.Client, hostname string, selfIP string, iptablesSyncPeriod time.Duration, mtu uint32) (*OsdnNode, error) {
	if !osapi.IsOpenShiftNetworkPlugin(pluginName) {
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

	ovsif, err := ovs.New(kexec.New(), BR)
	if err != nil {
		return nil, err
	}

	plugin := &OsdnNode{
		multitenant:        osapi.IsOpenShiftMultitenantNetworkPlugin(pluginName),
		kClient:            kClient,
		osClient:           osClient,
		ovs:                ovsif,
		localIP:            selfIP,
		hostName:           hostname,
		vnids:              newNodeVNIDMap(),
		podNetworkReady:    make(chan struct{}),
		kubeletInitReady:   make(chan struct{}),
		iptablesSyncPeriod: iptablesSyncPeriod,
		mtu:                mtu,
		egressPolicies:     make(map[uint32][]*osapi.EgressNetworkPolicy),
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
		return fmt.Errorf("Failed to get network information: %v", err)
	}

	nodeIPTables := newNodeIPTables(node.networkInfo.ClusterNetwork.String(), node.iptablesSyncPeriod)
	if err = nodeIPTables.Setup(); err != nil {
		return fmt.Errorf("Failed to set up iptables: %v", err)
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

	if node.multitenant {
		if err = node.VnidStartNode(); err != nil {
			return err
		}
		if err = node.SetupEgressNetworkPolicy(); err != nil {
			return err
		}
	}

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
	node.podManager, err = newPodManager(node.host, node.multitenant, node.localSubnetCIDR, node.networkInfo, node.kClient, node.vnids, node.mtu)
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
	podList, err := node.kClient.Pods(namespace).List(opts)
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
