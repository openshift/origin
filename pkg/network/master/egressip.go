package master

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	networkclient "github.com/openshift/client-go/network/clientset/versioned"
	networkinformers "github.com/openshift/client-go/network/informers/externalversions/network/v1"
	"github.com/openshift/origin/pkg/network/common"
)

type egressIPManager struct {
	sync.Mutex

	tracker            *common.EgressIPTracker
	networkClient      networkclient.Interface
	hostSubnetInformer networkinformers.HostSubnetInformer

	updatePending bool
	updatedAgain  bool

	monitorNodes map[string]*egressNode
	stop         chan struct{}
}

type egressNode struct {
	ip      string
	offline bool
	retries int
}

func newEgressIPManager() *egressIPManager {
	eim := &egressIPManager{}
	eim.tracker = common.NewEgressIPTracker(eim)
	return eim
}

func (eim *egressIPManager) Start(networkClient networkclient.Interface, hostSubnetInformer networkinformers.HostSubnetInformer, netNamespaceInformer networkinformers.NetNamespaceInformer) {
	eim.networkClient = networkClient
	eim.hostSubnetInformer = hostSubnetInformer
	eim.tracker.Start(hostSubnetInformer, netNamespaceInformer)
}

func (eim *egressIPManager) UpdateEgressCIDRs() {
	eim.Lock()
	defer eim.Unlock()

	// Coalesce multiple "UpdateEgressCIDRs" notifications into one by queueing
	// the update to happen a little bit later in a goroutine, and postponing that
	// update any time we get another "UpdateEgressCIDRs".

	if eim.updatePending {
		eim.updatedAgain = true
	} else {
		eim.updatePending = true
		go utilwait.PollInfinite(time.Second, eim.maybeDoUpdateEgressCIDRs)
	}
}

func (eim *egressIPManager) maybeDoUpdateEgressCIDRs() (bool, error) {
	eim.Lock()
	defer eim.Unlock()

	if eim.updatedAgain {
		eim.updatedAgain = false
		return false, nil
	}
	eim.updatePending = false

	// At this point it has been at least 1 second since the last "UpdateEgressCIDRs"
	// notification, so things are stable.
	//
	// ReallocateEgressIPs() will figure out what HostSubnets either can have new
	// egress IPs added to them, or need to have egress IPs removed from them, and
	// returns a map from node name to the new EgressIPs value, for each changed
	// HostSubnet.
	//
	// If a HostSubnet's EgressCIDRs changes while we are processing the reallocation,
	// we won't process that until this reallocation is complete.

	allocation := eim.tracker.ReallocateEgressIPs()
	monitorNodes := make(map[string]*egressNode, len(allocation))
	for nodeName, egressIPs := range allocation {
		resultErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			hs, err := eim.hostSubnetInformer.Lister().Get(nodeName)
			if err != nil {
				return err
			}

			if node := eim.monitorNodes[hs.HostIP]; node != nil {
				monitorNodes[hs.HostIP] = node
			} else {
				monitorNodes[hs.HostIP] = &egressNode{ip: hs.HostIP}
			}

			oldIPs := sets.NewString(hs.EgressIPs...)
			newIPs := sets.NewString(egressIPs...)
			if !oldIPs.Equal(newIPs) {
				hs.EgressIPs = egressIPs
				_, err = eim.networkClient.Network().HostSubnets().Update(hs)
			}
			return err
		})
		if resultErr != nil {
			utilruntime.HandleError(fmt.Errorf("Could not update HostSubnet EgressIPs: %v", resultErr))
		}
	}

	eim.monitorNodes = monitorNodes
	if len(monitorNodes) > 0 {
		if eim.stop == nil {
			eim.stop = make(chan struct{})
			go eim.poll(eim.stop)
		}
	} else {
		if eim.stop != nil {
			close(eim.stop)
			eim.stop = nil
		}
	}

	return true, nil
}

const (
	pollInterval   = 5 * time.Second
	repollInterval = time.Second
	maxRetries     = 2
)

func (eim *egressIPManager) poll(stop chan struct{}) {
	retry := false
	for {
		select {
		case <-stop:
			return
		default:
		}

		start := time.Now()
		retry := eim.check(retry)
		if !retry {
			// If less than pollInterval has passed since start, then sleep until it has
			time.Sleep(start.Add(pollInterval).Sub(time.Now()))
		}
	}
}

func (eim *egressIPManager) check(retrying bool) bool {
	var timeout time.Duration
	if retrying {
		timeout = repollInterval
	} else {
		timeout = pollInterval
	}

	needRetry := false
	for _, node := range eim.monitorNodes {
		if retrying && node.retries == 0 {
			continue
		}

		online := eim.tracker.Ping(node.ip, timeout)
		if node.offline && online {
			glog.Infof("Node %s is back online", node.ip)
			node.offline = false
			eim.tracker.SetNodeOffline(node.ip, false)
		} else if !node.offline && !online {
			node.retries++
			if node.retries > maxRetries {
				glog.Warningf("Node %s is offline", node.ip)
				node.retries = 0
				node.offline = true
				eim.tracker.SetNodeOffline(node.ip, true)
			} else {
				glog.V(2).Infof("Node %s may be offline... retrying", node.ip)
				needRetry = true
			}
		}
	}

	return needRetry
}

func (eim *egressIPManager) ClaimEgressIP(vnid uint32, egressIP, nodeIP string) {
}

func (eim *egressIPManager) ReleaseEgressIP(egressIP, nodeIP string) {
}

func (eim *egressIPManager) SetNamespaceEgressNormal(vnid uint32) {
}

func (eim *egressIPManager) SetNamespaceEgressDropped(vnid uint32) {
}

func (eim *egressIPManager) SetNamespaceEgressViaEgressIP(vnid uint32, egressIP, nodeIP string) {
}
