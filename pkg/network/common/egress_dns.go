package common

import (
	"net"
	"sync"
	"time"

	networkapi "github.com/openshift/api/network/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog"
)

type EgressDNSUpdate struct {
	UID       ktypes.UID
	Namespace string
}

type EgressDNSUpdates []EgressDNSUpdate

type EgressDNS struct {
	// Protects pdMap/namespaces operations
	lock sync.Mutex
	// holds DNS entries globally
	dns *DNS
	// this map holds which DNS names are in what policy objects
	dnsNamesToPolicies map[string]sets.String
	// Maintain namespaces for each policy to avoid querying etcd in syncEgressDNSPolicyRules()
	namespaces map[ktypes.UID]string

	// Report change when Add operation is done
	added chan bool

	// Report changes when there are dns updates
	Updates chan EgressDNSUpdates
}

func NewEgressDNS() (*EgressDNS, error) {
	dnsInfo, err := NewDNS("/etc/resolv.conf")
	if err != nil {
		utilruntime.HandleError(err)
		return nil, err
	}
	return &EgressDNS{
		dns:                dnsInfo,
		dnsNamesToPolicies: map[string]sets.String{},
		namespaces:         map[ktypes.UID]string{},
		added:              make(chan bool),
		Updates:            make(chan EgressDNSUpdates),
	}, nil
}

func (e *EgressDNS) Add(policy networkapi.EgressNetworkPolicy) {
	e.lock.Lock()
	defer e.lock.Unlock()

	for _, rule := range policy.Spec.Egress {
		if len(rule.To.DNSName) > 0 {
			if _, exists := e.dnsNamesToPolicies[rule.To.DNSName]; !exists {
				e.dnsNamesToPolicies[rule.To.DNSName] = sets.NewString(string(policy.UID))
				//only call Add if the dnsName doesn't exist in the dnsNamesToPolicies
				if err := e.dns.Add(rule.To.DNSName); err != nil {
					utilruntime.HandleError(err)
				}
				e.signalAdded()
			} else {
				e.dnsNamesToPolicies[rule.To.DNSName].Insert(string(policy.UID))
			}
		}
	}
	e.namespaces[policy.UID] = policy.Namespace
}

func (e *EgressDNS) Delete(policy networkapi.EgressNetworkPolicy) {
	e.lock.Lock()
	defer e.lock.Unlock()
	//delete the entry from the dnsNames to UIDs map for each rule in the policy
	//if the slice is empty at this point, delete the entry from the dns object too
	//also remove the policy entry from the namespaces map.
	for _, rule := range policy.Spec.Egress {
		if len(rule.To.DNSName) > 0 {
			if uids, ok := e.dnsNamesToPolicies[rule.To.DNSName]; ok {
				uids.Delete(string(policy.UID))
				if uids.Len() == 0 {
					e.dns.Delete(rule.To.DNSName)
					delete(e.dnsNamesToPolicies, rule.To.DNSName)
				} else {
					e.dnsNamesToPolicies[rule.To.DNSName] = uids
				}
			}
		}
	}

	if _, ok := e.namespaces[policy.UID]; ok {
		delete(e.namespaces, policy.UID)
	}
}

func (e *EgressDNS) Update(dns string) (bool, error) {
	e.lock.Lock()
	defer e.lock.Unlock()

	return e.dns.Update(dns)
}

func (e *EgressDNS) Sync() {
	var duration time.Duration
	for {
		tm, dnsName, updates, ok := e.GetNextQueryTime()
		if !ok {
			duration = 30 * time.Minute
		} else {
			now := time.Now()
			if tm.After(now) {
				// Item needs to wait for this duration before it can be processed
				duration = tm.Sub(now)
			} else {
				changed, err := e.Update(dnsName)
				if err != nil {
					utilruntime.HandleError(err)
				}

				if changed {
					e.Updates <- updates
				}
				continue
			}
		}

		// Wait for the given duration or till something got added
		select {
		case <-e.added:
		case <-time.After(duration):
		}
	}
}

func (e *EgressDNS) GetNextQueryTime() (time.Time, string, []EgressDNSUpdate, bool) {
	e.lock.Lock()
	defer e.lock.Unlock()
	policyUpdates := make([]EgressDNSUpdate, 0)
	tm, dnsName, timeSet := e.dns.GetNextQueryTime()
	if !timeSet {
		return tm, dnsName, nil, timeSet
	}

	if uids, exists := e.dnsNamesToPolicies[dnsName]; exists {
		for uid := range uids {
			policyUpdates = append(policyUpdates, EgressDNSUpdate{ktypes.UID(uid), e.namespaces[ktypes.UID(uid)]})
		}
	} else {
		klog.V(5).Infof("Didn't find any entry for dns name: %s in the dns map.", dnsName)
	}
	return tm, dnsName, policyUpdates, timeSet
}

func (e *EgressDNS) GetIPs(dnsName string) []net.IP {
	e.lock.Lock()
	defer e.lock.Unlock()
	return e.dns.Get(dnsName).ips

}

func (e *EgressDNS) GetNetCIDRs(dnsName string) []net.IPNet {
	cidrs := []net.IPNet{}
	for _, ip := range e.GetIPs(dnsName) {
		// IPv4 CIDR
		cidrs = append(cidrs, net.IPNet{IP: ip, Mask: net.CIDRMask(32, 32)})
	}
	return cidrs
}

func (e *EgressDNS) signalAdded() {
	// Non-blocking op
	select {
	case e.added <- true:
	default:
	}
}
