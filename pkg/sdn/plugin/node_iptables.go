package plugin

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"

	utildbus "k8s.io/kubernetes/pkg/util/dbus"
	kexec "k8s.io/kubernetes/pkg/util/exec"
	"k8s.io/kubernetes/pkg/util/iptables"
	utilwait "k8s.io/kubernetes/pkg/util/wait"
)

type FirewallRule struct {
	table string
	chain string
	args  []string
}

type NodeIPTables struct {
	ipt                iptables.Interface
	clusterNetworkCIDR string
	syncPeriod         time.Duration

	mu sync.Mutex // Protects concurrent access to syncIPTableRules()
}

func newNodeIPTables(clusterNetworkCIDR string, syncPeriod time.Duration) *NodeIPTables {
	return &NodeIPTables{
		ipt:                iptables.New(kexec.New(), utildbus.New(), iptables.ProtocolIpv4),
		clusterNetworkCIDR: clusterNetworkCIDR,
		syncPeriod:         syncPeriod,
	}
}

func (n *NodeIPTables) Setup() error {
	if err := n.syncIPTableRules(); err != nil {
		return err
	}

	// If firewalld is running, reload will call this method
	n.ipt.AddReloadFunc(func() {
		if err := n.syncIPTableRules(); err != nil {
			glog.Errorf("Reloading openshift iptables failed: %v", err)
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
			glog.Errorf("Syncing openshift iptables failed: %v", err)
		}
	}
}

// syncIPTableRules syncs the cluster network cidr iptables rules.
// Called from SyncLoop() or firwalld reload()
func (n *NodeIPTables) syncIPTableRules() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	start := time.Now()
	defer func() {
		glog.V(4).Infof("syncIPTableRules took %v", time.Since(start))
	}()
	glog.V(3).Infof("Syncing openshift iptables rules")

	rules := n.getStaticNodeIPTablesRules()
	for _, rule := range rules {
		_, err := n.ipt.EnsureRule(iptables.Prepend, iptables.Table(rule.table), iptables.Chain(rule.chain), rule.args...)
		if err != nil {
			return fmt.Errorf("Failed to ensure rule %v exists: %v", rule, err)
		}
	}
	return nil
}

// Get openshift iptables rules
func (n *NodeIPTables) getStaticNodeIPTablesRules() []FirewallRule {
	return []FirewallRule{
		{"nat", "POSTROUTING", []string{"-s", n.clusterNetworkCIDR, "-j", "MASQUERADE"}},
		{"filter", "INPUT", []string{"-p", "udp", "-m", "multiport", "--dports", VXLAN_PORT, "-m", "comment", "--comment", "001 vxlan incoming", "-j", "ACCEPT"}},
		{"filter", "INPUT", []string{"-i", TUN, "-m", "comment", "--comment", "traffic from SDN", "-j", "ACCEPT"}},
		{"filter", "INPUT", []string{"-i", "docker0", "-m", "comment", "--comment", "traffic from docker", "-j", "ACCEPT"}},
		{"filter", "FORWARD", []string{"-d", n.clusterNetworkCIDR, "-j", "ACCEPT"}},
		{"filter", "FORWARD", []string{"-s", n.clusterNetworkCIDR, "-j", "ACCEPT"}},
	}
}
