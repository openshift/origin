package master

import (
	"fmt"
	"sync"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
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
	for nodeName, egressIPs := range allocation {
		resultErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			hs, err := eim.hostSubnetInformer.Lister().Get(nodeName)
			if err != nil {
				return err
			}

			hs.EgressIPs = egressIPs
			_, err = eim.networkClient.Network().HostSubnets().Update(hs)
			return err
		})
		if resultErr != nil {
			utilruntime.HandleError(fmt.Errorf("Could not update HostSubnet EgressIPs: %v", resultErr))
		}
	}

	return true, nil
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
