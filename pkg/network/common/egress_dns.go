package common

import (
	"net"
	"sync"
	"time"

	networkapi "github.com/openshift/origin/pkg/network/apis/network"

	ktypes "k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kexec "k8s.io/utils/exec"
)

type EgressDNSUpdate struct {
	UID       ktypes.UID
	Namespace string
}

type EgressDNS struct {
	// Protects pdMap/namespaces operations
	lock sync.Mutex
	// Holds Egress DNS entries for each policy
	pdMap map[ktypes.UID]*DNS
	// Maintain namespaces for each policy to avoid querying etcd in syncEgressDNSPolicyRules()
	namespaces map[ktypes.UID]string

	// Report change when Add operation is done
	added chan bool

	// Report changes when there are dns updates
	Updates chan EgressDNSUpdate
}

func NewEgressDNS() *EgressDNS {
	return &EgressDNS{
		pdMap:      map[ktypes.UID]*DNS{},
		namespaces: map[ktypes.UID]string{},
		added:      make(chan bool),
		Updates:    make(chan EgressDNSUpdate),
	}
}

func (e *EgressDNS) Add(policy networkapi.EgressNetworkPolicy) {
	dnsInfo := NewDNS(kexec.New())
	for _, rule := range policy.Spec.Egress {
		if len(rule.To.DNSName) > 0 {
			if err := dnsInfo.Add(rule.To.DNSName); err != nil {
				utilruntime.HandleError(err)
			}
		}
	}

	if dnsInfo.Size() > 0 {
		e.lock.Lock()
		defer e.lock.Unlock()

		e.pdMap[policy.UID] = dnsInfo
		e.namespaces[policy.UID] = policy.Namespace
		e.signalAdded()
	}
}

func (e *EgressDNS) Delete(policy networkapi.EgressNetworkPolicy) {
	e.lock.Lock()
	defer e.lock.Unlock()

	if _, ok := e.pdMap[policy.UID]; ok {
		delete(e.pdMap, policy.UID)
		delete(e.namespaces, policy.UID)
	}
}

func (e *EgressDNS) Update(policyUID ktypes.UID) (error, bool) {
	e.lock.Lock()
	defer e.lock.Unlock()

	if dnsInfo, ok := e.pdMap[policyUID]; ok {
		return dnsInfo.Update()
	}
	return nil, false
}

func (e *EgressDNS) Sync() {
	var duration time.Duration
	for {
		tm, policyUID, policyNamespace, ok := e.GetMinQueryTime()
		if !ok {
			duration = 30 * time.Minute
		} else {
			now := time.Now()
			if tm.After(now) {
				// Item needs to wait for this duration before it can be processed
				duration = tm.Sub(now)
			} else {
				err, changed := e.Update(policyUID)
				if err != nil {
					utilruntime.HandleError(err)
				}

				if changed {
					e.Updates <- EgressDNSUpdate{policyUID, policyNamespace}
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

func (e *EgressDNS) GetMinQueryTime() (time.Time, ktypes.UID, string, bool) {
	e.lock.Lock()
	defer e.lock.Unlock()

	timeSet := false
	var minTime time.Time
	var uid ktypes.UID

	for policyUID, dnsInfo := range e.pdMap {
		tm, ok := dnsInfo.GetMinQueryTime()
		if !ok {
			continue
		}

		if (timeSet == false) || tm.Before(minTime) {
			timeSet = true
			minTime = tm
			uid = policyUID
		}
	}

	return minTime, uid, e.namespaces[uid], timeSet
}

func (e *EgressDNS) GetIPs(policy networkapi.EgressNetworkPolicy, dnsName string) []net.IP {
	e.lock.Lock()
	defer e.lock.Unlock()

	dnsInfo, ok := e.pdMap[policy.UID]
	if !ok {
		return []net.IP{}
	}
	return dnsInfo.Get(dnsName).ips
}

func (e *EgressDNS) GetNetCIDRs(policy networkapi.EgressNetworkPolicy, dnsName string) []net.IPNet {
	cidrs := []net.IPNet{}
	for _, ip := range e.GetIPs(policy, dnsName) {
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
