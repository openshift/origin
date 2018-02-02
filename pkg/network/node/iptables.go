// +build linux

package node

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	utildbus "k8s.io/kubernetes/pkg/util/dbus"
	"k8s.io/kubernetes/pkg/util/iptables"
	kexec "k8s.io/utils/exec"
)

type NodeIPTables struct {
	ipt                iptables.Interface
	clusterNetworkCIDR []string
	syncPeriod         time.Duration
	masqueradeServices bool

	mu sync.Mutex // Protects concurrent access to syncIPTableRules()
}

func newNodeIPTables(clusterNetworkCIDR []string, syncPeriod time.Duration, masqueradeServices bool) *NodeIPTables {
	return &NodeIPTables{
		ipt:                iptables.New(kexec.New(), utildbus.New(), iptables.ProtocolIpv4),
		clusterNetworkCIDR: clusterNetworkCIDR,
		syncPeriod:         syncPeriod,
		masqueradeServices: masqueradeServices,
	}
}

func (n *NodeIPTables) Setup() error {
	if err := n.syncIPTableRules(); err != nil {
		return err
	}

	// If firewalld is running, reload will call this method
	n.ipt.AddReloadFunc(func() {
		if err := n.syncIPTableRules(); err != nil {
			utilruntime.HandleError(fmt.Errorf("Reloading openshift iptables failed: %v", err))
		}
	})

	go utilwait.Forever(n.syncLoop, 0)
	return nil
}

// syncLoop periodically calls syncIPTableRules().
// This is expected to run as a go routine or as the main loop. It does not return.
func (n *NodeIPTables) syncLoop() {
	t := time.NewTicker(n.syncPeriod)
	defer t.Stop()
	for {
		<-t.C
		glog.V(6).Infof("Periodic openshift iptables sync")
		err := n.syncIPTableRules()
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Syncing openshift iptables failed: %v", err))
		}
	}
}

type Chain struct {
	table    string
	name     string
	srcChain string
	srcRule  []string
	rules    [][]string
}

// Adds all the rules in chain, returning true if they were all already present
func (n *NodeIPTables) addChainRules(chain Chain) (bool, error) {
	allExisted := true
	for _, rule := range chain.rules {
		existed, err := n.ipt.EnsureRule(iptables.Append, iptables.Table(chain.table), iptables.Chain(chain.name), rule...)
		if err != nil {
			return false, fmt.Errorf("failed to ensure rule %v exists: %v", rule, err)
		}
		if !existed {
			allExisted = false
		}
	}
	return allExisted, nil
}

// syncIPTableRules syncs the cluster network cidr iptables rules.
// Called from SyncLoop() or firewalld reload()
func (n *NodeIPTables) syncIPTableRules() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	start := time.Now()
	defer func() {
		glog.V(4).Infof("syncIPTableRules took %v", time.Since(start))
	}()
	glog.V(3).Infof("Syncing openshift iptables rules")

	chains := n.getNodeIPTablesChains()
	for i := len(chains) - 1; i >= 0; i-- {
		chain := chains[i]
		// Create chain if it does not already exist
		chainExisted, err := n.ipt.EnsureChain(iptables.Table(chain.table), iptables.Chain(chain.name))
		if err != nil {
			return fmt.Errorf("failed to ensure chain %s exists: %v", chain.name, err)
		}
		if chain.srcChain != "" {
			// Create the rule pointing to it from its parent chain. Note that since we
			// use iptables.Prepend each time, but process the chains in reverse order,
			// chains with the same table and srcChain (ie, OPENSHIFT-FIREWALL-FORWARD
			// and OPENSHIFT-ADMIN-OUTPUT-RULES) will run in the same order as they
			// appear in getNodeIPTablesChains().
			_, err = n.ipt.EnsureRule(iptables.Prepend, iptables.Table(chain.table), iptables.Chain(chain.srcChain), append(chain.srcRule, "-j", chain.name)...)
			if err != nil {
				return fmt.Errorf("failed to ensure rule from %s to %s exists: %v", chain.srcChain, chain.name, err)
			}
		}

		// Add/sync the rules
		rulesExisted, err := n.addChainRules(chain)
		if err != nil {
			return err
		}
		if chainExisted && !rulesExisted {
			// Chain existed but not with the expected rules; this probably means
			// it contained rules referring to a *different* subnet; flush them
			// and try again.
			if err = n.ipt.FlushChain(iptables.Table(chain.table), iptables.Chain(chain.name)); err != nil {
				return fmt.Errorf("failed to flush chain %s: %v", chain.name, err)
			}
			if _, err = n.addChainRules(chain); err != nil {
				return err
			}
		}
	}

	return nil
}

const vxlanPort = "4789"

func (n *NodeIPTables) getNodeIPTablesChains() []Chain {

	var chainArray []Chain

	chainArray = append(chainArray,
		Chain{
			table:    "filter",
			name:     "OPENSHIFT-FIREWALL-ALLOW",
			srcChain: "INPUT",
			srcRule:  []string{"-m", "comment", "--comment", "firewall overrides"},
			rules: [][]string{
				{"-p", "udp", "--dport", vxlanPort, "-m", "comment", "--comment", "VXLAN incoming", "-j", "ACCEPT"},
				{"-i", Tun0, "-m", "comment", "--comment", "from SDN to localhost", "-j", "ACCEPT"},
				{"-i", "docker0", "-m", "comment", "--comment", "from docker to localhost", "-j", "ACCEPT"},
			},
		},
		Chain{
			table:    "filter",
			name:     "OPENSHIFT-ADMIN-OUTPUT-RULES",
			srcChain: "FORWARD",
			srcRule:  []string{"-i", Tun0, "!", "-o", Tun0, "-m", "comment", "--comment", "administrator overrides"},
			rules:    nil,
		},
	)

	var masqRules [][]string
	var masq2Rules [][]string
	var filterRules [][]string
	for _, cidr := range n.clusterNetworkCIDR {
		if n.masqueradeServices {
			masqRules = append(masqRules, []string{"-s", cidr, "-m", "comment", "--comment", "masquerade pod-to-service and pod-to-external traffic", "-j", "MASQUERADE"})
		} else {
			masqRules = append(masqRules, []string{"-s", cidr, "-m", "comment", "--comment", "masquerade pod-to-external traffic", "-j", "OPENSHIFT-MASQUERADE-2"})
			masq2Rules = append(masq2Rules, []string{"-d", cidr, "-m", "comment", "--comment", "masquerade pod-to-external traffic", "-j", "RETURN"})
		}

		filterRules = append(filterRules, []string{"-s", cidr, "-m", "comment", "--comment", "attempted resend after connection close", "-m", "conntrack", "--ctstate", "INVALID", "-j", "DROP"})
		filterRules = append(filterRules, []string{"-d", cidr, "-m", "comment", "--comment", "forward traffic from SDN", "-j", "ACCEPT"})
		filterRules = append(filterRules, []string{"-s", cidr, "-m", "comment", "--comment", "forward traffic to SDN", "-j", "ACCEPT"})
	}

	chainArray = append(chainArray,
		Chain{
			table:    "nat",
			name:     "OPENSHIFT-MASQUERADE",
			srcChain: "POSTROUTING",
			srcRule:  []string{"-m", "comment", "--comment", "rules for masquerading OpenShift traffic"},
			rules:    masqRules,
		},
		Chain{
			table:    "filter",
			name:     "OPENSHIFT-FIREWALL-FORWARD",
			srcChain: "FORWARD",
			srcRule:  []string{"-m", "comment", "--comment", "firewall overrides"},
			rules:    filterRules,
		},
	)
	if !n.masqueradeServices {
		masq2Rules = append(masq2Rules, []string{"-j", "MASQUERADE"})
		chainArray = append(chainArray,
			Chain{
				table: "nat",
				name:  "OPENSHIFT-MASQUERADE-2",
				rules: masq2Rules,
			},
		)
	}
	return chainArray
}

func (n *NodeIPTables) AddEgressIPRules(egressIP, mark string) error {
	for _, cidr := range n.clusterNetworkCIDR {
		_, err := n.ipt.EnsureRule(iptables.Prepend, iptables.TableNAT, iptables.Chain("OPENSHIFT-MASQUERADE"), "-s", cidr, "-m", "mark", "--mark", mark, "-j", "SNAT", "--to-source", egressIP)
		if err != nil {
			return err
		}
	}
	_, err := n.ipt.EnsureRule(iptables.Append, iptables.TableFilter, iptables.Chain("OPENSHIFT-FIREWALL-ALLOW"), "-d", egressIP, "-m", "conntrack", "--ctstate", "NEW", "-j", "REJECT")
	return err
}

func (n *NodeIPTables) DeleteEgressIPRules(egressIP, mark string) error {
	for _, cidr := range n.clusterNetworkCIDR {
		err := n.ipt.DeleteRule(iptables.TableNAT, iptables.Chain("OPENSHIFT-MASQUERADE"), "-s", cidr, "-m", "mark", "--mark", mark, "-j", "SNAT", "--to-source", egressIP)
		if err != nil {
			return err
		}
	}
	return n.ipt.DeleteRule(iptables.TableFilter, iptables.Chain("OPENSHIFT-FIREWALL-ALLOW"), "-d", egressIP, "-m", "conntrack", "--ctstate", "NEW", "-j", "REJECT")
}
