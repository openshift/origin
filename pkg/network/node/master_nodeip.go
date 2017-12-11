package node

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	"github.com/openshift/origin/pkg/util/netutils"
)

// AnnotateMasterTrafficNodeIP assigns master traffic node IP as annotation on the local node object.
// We use exponential back off polling on the local node object instead of node shared informers
// because kube informers will not be started until all the openshift controllers are started.
func (plugin *OsdnNode) AnnotateMasterTrafficNodeIP() error {
	backoff := utilwait.Backoff{
		// A bit under 2 minutes total
		Duration: time.Second,
		Factor:   1.5,
		Steps:    11,
	}

	var node *kapi.Node
	err := utilwait.ExponentialBackoff(backoff, func() (bool, error) {
		var err error
		node, err = plugin.kClient.Core().Nodes().Get(plugin.hostName, metav1.GetOptions{})
		if err == nil {
			return true, nil
		} else if kapierrors.IsNotFound(err) {
			glog.Warningf("Could not find local node object: %s, Waiting...", plugin.hostName)
			return false, nil
		} else {
			return false, err
		}
	})
	if err != nil {
		return fmt.Errorf("failed to get node object for this host: %s, error: %v", plugin.hostName, err)
	}

	if err := plugin.handleLocalNode(node); err != nil {
		return err
	}

	return nil
}

func (plugin *OsdnNode) handleLocalNode(node *kapi.Node) error {
	if len(plugin.masterTrafficIP) == 0 {
		// Unset master traffic node IP if needed
		return plugin.unsetMasterTrafficNodeIPAnnotation(node)
	}

	nodeIP, err := getMasterTrafficNodeIPAnnotation(node)
	if (err == nil) && (nodeIP == plugin.masterTrafficIP) {
		return nil
	}

	isLocal, err := plugin.isNodeIPLocal(plugin.masterTrafficIP)
	if err != nil {
		return fmt.Errorf("failed to check if master traffic node IP %q is local or not, %v", plugin.masterTrafficIP, err)
	}
	if !isLocal {
		return fmt.Errorf("master traffic node IP %q is not local", plugin.masterTrafficIP)
	}

	if err := plugin.setMasterTrafficNodeIPAnnotation(node); err != nil {
		return fmt.Errorf("unable to set master traffic node IP annotation for node %q, %v", node.Name, err)
	}

	return nil
}

func (plugin *OsdnNode) isNodeIPLocal(nodeIP string) (bool, error) {
	_, hostIPs, err := netutils.GetHostIPNetworks([]string{Tun0})
	if err != nil {
		return false, err
	}

	for _, ip := range hostIPs {
		if nodeIP == ip.String() {
			return true, nil
		}
	}
	return false, nil
}

func getMasterTrafficNodeIPAnnotation(node *kapi.Node) (string, error) {
	if len(node.Annotations) > 0 {
		if nodeIP, ok := node.Annotations[networkapi.MasterTrafficNodeIPAnnotation]; ok {
			return nodeIP, nil
		}
	}

	return "", fmt.Errorf("master traffic node IP not found for node %q", node.Name)
}

func (plugin *OsdnNode) setMasterTrafficNodeIPAnnotation(node *kapi.Node) error {
	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}
	node.Annotations[networkapi.MasterTrafficNodeIPAnnotation] = plugin.masterTrafficIP

	return plugin.updateNode(node)
}

func (plugin *OsdnNode) unsetMasterTrafficNodeIPAnnotation(node *kapi.Node) error {
	if node.Annotations != nil {
		if _, ok := node.Annotations[networkapi.MasterTrafficNodeIPAnnotation]; ok {
			delete(node.Annotations, networkapi.MasterTrafficNodeIPAnnotation)
			return plugin.updateNode(node)
		}
	}

	return nil
}

func (plugin *OsdnNode) updateNode(node *kapi.Node) error {
	// A bit over 1 minute total
	backoff := utilwait.Backoff{
		Duration: time.Second,
		Factor:   1.5,
		Steps:    10,
	}

	return utilwait.ExponentialBackoff(backoff, func() (bool, error) {
		_, err := plugin.kClient.Core().Nodes().Update(node)
		if err == nil {
			return true, nil
		} else if kapierrors.IsNotFound(err) {
			return false, fmt.Errorf("could not find local node for host: %s", plugin.hostName)
		} else {
			return false, nil
		}
	})
}
