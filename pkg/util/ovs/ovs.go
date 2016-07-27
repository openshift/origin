// Package ovs provides a wrapper around ovs-vsctl and ovs-ofctl
package ovs

import (
	"fmt"
	"strings"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/util/exec"
)

type Transaction struct {
	execer exec.Interface
	bridge string
	err    error
}

// NewTransaction begins a new OVS transaction for a given bridge. If an error
// occurs at any step in the transaction, it will be recorded until
// EndTransaction(), and any further calls on the transaction will be ignored.
func NewTransaction(execer exec.Interface, bridge string) *Transaction {
	return &Transaction{execer: execer, bridge: bridge}
}

func (tx *Transaction) exec(cmd string, args ...string) (string, error) {
	if tx.err != nil {
		return "", tx.err
	}

	cmdpath, err := tx.execer.LookPath(cmd)
	if err != nil {
		tx.err = fmt.Errorf("OVS is not installed")
		return "", tx.err
	}

	glog.V(5).Infof("Executing: %s %s", cmdpath, strings.Join(args, " "))
	var output []byte
	output, tx.err = tx.execer.Command(cmdpath, args...).CombinedOutput()
	if tx.err != nil {
		glog.V(5).Infof("Error executing %s: %s", cmdpath, string(output))
	}
	return string(output), tx.err
}

func (tx *Transaction) vsctlExec(args ...string) (string, error) {
	return tx.exec("ovs-vsctl", args...)
}

func (tx *Transaction) ofctlExec(args ...string) (string, error) {
	args = append([]string{"-O", "OpenFlow13"}, args...)
	return tx.exec("ovs-ofctl", args...)
}

// AddBridge creates the bridge associated with the transaction, optionally setting
// properties on it (as with "ovs-vsctl set Bridge ..."). If the bridge already
// existed, it will be destroyed and recreated.
func (tx *Transaction) AddBridge(properties ...string) {
	args := []string{"--if-exists", "del-br", tx.bridge, "--", "add-br", tx.bridge}
	if len(properties) > 0 {
		args = append(args, "--", "set", "Bridge", tx.bridge)
		args = append(args, properties...)
	}
	tx.vsctlExec(args...)
}

// DeleteBridge deletes the bridge associated with the transaction. (It is an
// error if the bridge does not exist.)
func (tx *Transaction) DeleteBridge() {
	tx.vsctlExec("del-br", tx.bridge)
}

// AddPort adds an interface to the bridge, requesting the indicated port
// number, and optionally setting properties on it (as with "ovs-vsctl set
// Interface ...").
func (tx *Transaction) AddPort(port string, ofport uint, properties ...string) {
	args := []string{"--if-exists", "del-port", port, "--", "add-port", tx.bridge, port, "--", "set", "Interface", port, fmt.Sprintf("ofport_request=%d", ofport)}
	if len(properties) > 0 {
		args = append(args, properties...)
	}
	tx.vsctlExec(args...)
}

// DeletePort removes an interface from the bridge. (It is an error if the
// interface is not currently a bridge port.)
func (tx *Transaction) DeletePort(port string) {
	tx.vsctlExec("del-port", port)
}

// AddFlow adds a flow to the bridge. The arguments are passed to fmt.Sprintf().
func (tx *Transaction) AddFlow(flow string, args ...interface{}) {
	if len(args) > 0 {
		flow = fmt.Sprintf(flow, args...)
	}
	tx.ofctlExec("add-flow", tx.bridge, flow)
}

// DeleteFlows deletes all matching flows from the bridge. The arguments are
// passed to fmt.Sprintf().
func (tx *Transaction) DeleteFlows(flow string, args ...interface{}) {
	if len(args) > 0 {
		flow = fmt.Sprintf(flow, args...)
	}
	tx.ofctlExec("del-flows", tx.bridge, flow)
}

// DumpFlows dumps the flow table for the bridge and returns it as an array of
// strings, one per flow. Since this function has a return value, it also
// returns an error immediately if an error occurs.
func (tx *Transaction) DumpFlows() ([]string, error) {
	out, err := tx.ofctlExec("dump-flows", tx.bridge)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(out, "\n")
	flows := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.Contains(line, "cookie=") {
			flows = append(flows, line)
		}
	}
	return flows, nil
}

// EndTransaction ends an OVS transaction and returns any error that occurred
// during the transaction. You should not use the transaction again after
// calling this function.
func (tx *Transaction) EndTransaction() error {
	err := tx.err
	tx.err = nil
	return err
}
