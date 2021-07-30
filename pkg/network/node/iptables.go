// +build linux

package node

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/pkg/util/iptables"
	utilexec "k8s.io/utils/exec"
)

type NodeIPTables struct {
	ipt                iptables.Interface
	clusterNetworkCIDR []string
	masqueradeServices bool
	vxlanPort          uint32
	masqueradeBitHex   string // the masquerade bit as hex value

	mu sync.Mutex // Protects concurrent access to syncIPTableRules()

	egressIPs map[string]string
}

// this will retry 10 times over a period of 13 seconds
var iptablesBackoff wait.Backoff = wait.Backoff{
	Duration: 500 * time.Millisecond,
	Factor:   1.25,
	Steps:    10,
}

// execIPTablesWithRetry allows a simple way to retry iptables commands if they fail the first time
func execIPTablesWithRetry(f func() error) error {
	return wait.ExponentialBackoff(iptablesBackoff, func() (bool, error) {
		if err := f(); err != nil {
			if isResourceError(err) {
				glog.V(5).Infof("Call to iptables failed with transient failure: %v", err)
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
}

const iptablesStatusResourceProblem = 4

// isResourceError returns true if the error indicates that iptables ran into a "resource
// problem" and was unable to attempt the request. In particular, this will be true if it
// times out trying to get the iptables lock.
func isResourceError(err error) bool {
	if ee, isExitError := err.(utilexec.ExitError); isExitError {
		return ee.ExitStatus() == iptablesStatusResourceProblem
	}
	return false
}

func newNodeIPTables(ipt iptables.Interface, clusterNetworkCIDR []string, masqueradeServices bool, vxlanPort uint32, masqueradeBit uint32) *NodeIPTables {
	return &NodeIPTables{
		ipt:                ipt,
		clusterNetworkCIDR: clusterNetworkCIDR,
		masqueradeServices: masqueradeServices,
		vxlanPort:          vxlanPort,
		masqueradeBitHex:   fmt.Sprintf("%#x", 1<<masqueradeBit),
		egressIPs:          make(map[string]string),
	}
}

func (n *NodeIPTables) Setup() error {
	if err := n.syncIPTableRules(); err != nil {
		return err
	}
	return nil
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
		var existed bool
		err := execIPTablesWithRetry(func() error {
			var err error
			existed, err = n.ipt.EnsureRule(iptables.Append, iptables.Table(chain.table), iptables.Chain(chain.name), rule...)
			return err
		})
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
		var chainExisted bool
		err := execIPTablesWithRetry(func() error {
			var err error
			chainExisted, err = n.ipt.EnsureChain(iptables.Table(chain.table), iptables.Chain(chain.name))
			return err
		})
		if err != nil {
			return fmt.Errorf("failed to ensure chain %s exists: %v", chain.name, err)
		}
		if chain.srcChain != "" {
			// Create the rule pointing to it from its parent chain. Note that since we
			// use iptables.Prepend each time, but process the chains in reverse order,
			// chains with the same table and srcChain (ie, OPENSHIFT-FIREWALL-FORWARD
			// and OPENSHIFT-ADMIN-OUTPUT-RULES) will run in the same order as they
			// appear in getNodeIPTablesChains().
			err = execIPTablesWithRetry(func() error {
				_, err = n.ipt.EnsureRule(iptables.Prepend, iptables.Table(chain.table), iptables.Chain(chain.srcChain), append(chain.srcRule, "-j", chain.name)...)
				return err
			})
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
			err = execIPTablesWithRetry(func() error {
				return n.ipt.FlushChain(iptables.Table(chain.table), iptables.Chain(chain.name))
			})
			if err != nil {
				return fmt.Errorf("failed to flush chain %s: %v", chain.name, err)
			}
			if _, err = n.addChainRules(chain); err != nil {
				return err
			}
		}
	}

	for egressIP, mark := range n.egressIPs {
		if err := n.ensureEgressIPRules(egressIP, mark); err != nil {
			return err
		}
	}

	return nil
}

func (n *NodeIPTables) getNodeIPTablesChains() []Chain {

	var chainArray []Chain

	chainArray = append(chainArray,
		Chain{
			table:    "filter",
			name:     "OPENSHIFT-FIREWALL-ALLOW",
			srcChain: "INPUT",
			srcRule:  []string{"-m", "comment", "--comment", "firewall overrides"},
			rules: [][]string{
				{"-p", "udp", "--dport", fmt.Sprintf("%d", n.vxlanPort), "-m", "comment", "--comment", "VXLAN incoming", "-j", "ACCEPT"},
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

	masqRules := [][]string{
		// Skip traffic already marked by kube-proxy for masquerading.
		// This fixes a bug where traffic destined to a service's ExternalIP
		// but also intended to go be SNAT'd to an EgressIP was dropped.
		{"-m", "mark", "--mark", n.masqueradeBitHex + "/" + n.masqueradeBitHex, "-j", "RETURN"},
	}
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

func (n *NodeIPTables) ensureEgressIPRules(egressIP, mark string) error {
	for _, cidr := range n.clusterNetworkCIDR {
		err := execIPTablesWithRetry(func() error {
			_, err := n.ipt.EnsureRule(iptables.Prepend, iptables.TableNAT, iptables.Chain("OPENSHIFT-MASQUERADE"), "-s", cidr, "-m", "mark", "--mark", mark, "-j", "SNAT", "--to-source", egressIP)
			return err
		})
		if err != nil {
			return err
		}
	}
	err := execIPTablesWithRetry(func() error {
		_, err := n.ipt.EnsureRule(iptables.Append, iptables.TableFilter, iptables.Chain("OPENSHIFT-FIREWALL-ALLOW"), "-d", egressIP, "-m", "conntrack", "--ctstate", "NEW", "-j", "REJECT")
		return err
	})
	return err
}

func (n *NodeIPTables) AddEgressIPRules(egressIP, mark string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if err := n.ensureEgressIPRules(egressIP, mark); err != nil {
		return err
	}
	n.egressIPs[egressIP] = mark
	return nil
}

func (n *NodeIPTables) DeleteEgressIPRules(egressIP, mark string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	delete(n.egressIPs, egressIP)

	for _, cidr := range n.clusterNetworkCIDR {
		err := execIPTablesWithRetry(func() error {
			return n.ipt.DeleteRule(iptables.TableNAT, iptables.Chain("OPENSHIFT-MASQUERADE"), "-s", cidr, "-m", "mark", "--mark", mark, "-j", "SNAT", "--to-source", egressIP)
		})
		if err != nil {
			return err
		}
	}
	err := execIPTablesWithRetry(func() error {
		return n.ipt.DeleteRule(iptables.TableFilter, iptables.Chain("OPENSHIFT-FIREWALL-ALLOW"), "-d", egressIP, "-m", "conntrack", "--ctstate", "NEW", "-j", "REJECT")
	})
	return err
}

var masqRuleRE = regexp.MustCompile(`-A OPENSHIFT-MASQUERADE .* --to-source ([^ ]*)`)
var filterRuleRE = regexp.MustCompile(`-A OPENSHIFT-FIREWALL-ALLOW -d ([^ ]*)/32 .* -j REJECT`)

func (n *NodeIPTables) findStaleEgressIPRules(table iptables.Table, ruleMatch *regexp.Regexp) (map[string]string, error) {
	buf := bytes.NewBuffer(nil)
	err := n.ipt.SaveInto(table, buf)
	if err != nil {
		return nil, err
	}
	rules := make(map[string]string)
	for _, line := range strings.Split(string(buf.Bytes()), "\n") {
		match := ruleMatch.FindStringSubmatch(line)
		if len(match) != 2 {
			continue
		}
		rules[match[1]] = match[0]
	}

	// Delete rules matching current egress IPs
	for ip := range n.egressIPs {
		delete(rules, ip)
	}
	return rules, nil
}

func (n *NodeIPTables) SyncEgressIPRules() {
	masqRules, err := n.findStaleEgressIPRules(iptables.TableNAT, masqRuleRE)
	if err != nil {
		glog.Warningf("Error looking for stale egress IP iptables rules: %v", err)
	}
	filterRules, err := n.findStaleEgressIPRules(iptables.TableFilter, filterRuleRE)
	if err != nil {
		glog.Warningf("Error looking for stale egress IP iptables rules: %v", err)
	}

	for ip, rule := range masqRules {
		glog.V(2).Infof("Deleting iptables masquerade rule for stale egress IP %s", ip)
		args := strings.Split(rule, " ")
		if len(args) != 12 {
			glog.Warningf("Error deleting iptables masquerade rule for stale egress IP %s: unexpected rule format %q", ip, rule)
			continue
		}
		args = args[2:]
		err := execIPTablesWithRetry(func() error {
			return n.ipt.DeleteRule(iptables.TableNAT, iptables.Chain("OPENSHIFT-MASQUERADE"), args...)
		})
		if err != nil {
			glog.Warningf("Error deleting iptables masquerade rule for stale egress IP %s: %v", ip, err)
		}
	}

	for ip, rule := range filterRules {
		glog.V(2).Infof("Deleting iptables filter rule for stale egress IP %s", ip)
		args := strings.Split(rule, " ")
		if len(args) != 10 {
			glog.Warningf("Error deleting iptables filter rule for stale egress IP %s: unexpected rule format %q", ip, rule)
			continue
		}
		args = args[2:]
		err := execIPTablesWithRetry(func() error {
			return n.ipt.DeleteRule(iptables.TableFilter, iptables.Chain("OPENSHIFT-FIREWALL-ALLOW"), args...)
		})
		if err != nil {
			glog.Warningf("Error deleting iptables filter rule for stale egress IP %s: %v", ip, err)
		}
	}
}
