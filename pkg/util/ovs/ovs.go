// Package ovs provides a wrapper around ovs-vsctl and ovs-ofctl
package ovs

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/util/exec"
)

const (
	OVS_OFCTL = "ovs-ofctl"
	OVS_VSCTL = "ovs-vsctl"
)

type Interface struct {
	execer exec.Interface
	bridge string
}

// New returns a new ovs.Interface
func New(execer exec.Interface, bridge string) (*Interface, error) {
	if _, err := execer.LookPath(OVS_OFCTL); err != nil {
		return nil, fmt.Errorf("OVS is not installed")
	}
	if _, err := execer.LookPath(OVS_VSCTL); err != nil {
		return nil, fmt.Errorf("OVS is not installed")
	}

	return &Interface{execer: execer, bridge: bridge}, nil
}

func (ovsif *Interface) exec(cmd string, args ...string) (string, error) {
	if cmd == OVS_OFCTL {
		args = append([]string{"-O", "OpenFlow13"}, args...)
	}
	glog.V(5).Infof("Executing: %s %s", cmd, strings.Join(args, " "))

	output, err := ovsif.execer.Command(cmd, args...).CombinedOutput()
	if err != nil {
		glog.V(5).Infof("Error executing %s: %s", cmd, string(output))
		return "", err
	}

	outStr := string(output)
	if outStr != "" {
		// If output is a single line, strip the trailing newline
		nl := strings.Index(outStr, "\n")
		if nl == len(outStr)-1 {
			outStr = outStr[:nl]
		}
	}
	return outStr, nil
}

// AddBridge creates the bridge associated with the interface, optionally setting
// properties on it (as with "ovs-vsctl set Bridge ..."). If the bridge already
// existed, it will be destroyed and recreated.
func (ovsif *Interface) AddBridge(properties ...string) error {
	args := []string{"--if-exists", "del-br", ovsif.bridge, "--", "add-br", ovsif.bridge}
	if len(properties) > 0 {
		args = append(args, "--", "set", "Bridge", ovsif.bridge)
		args = append(args, properties...)
	}
	_, err := ovsif.exec(OVS_VSCTL, args...)
	return err
}

// DeleteBridge deletes the bridge associated with the interface. (It is an
// error if the bridge does not exist.)
func (ovsif *Interface) DeleteBridge() error {
	_, err := ovsif.exec(OVS_VSCTL, "del-br", ovsif.bridge)
	return err
}

// AddPort adds an interface to the bridge, requesting the indicated port
// number, and optionally setting properties on it (as with "ovs-vsctl set
// Interface ..."). Returns the allocated port number (or an error).
func (ovsif *Interface) AddPort(port string, ofportRequest int, properties ...string) (int, error) {
	args := []string{"--may-exist", "add-port", ovsif.bridge, port}
	if ofportRequest > 0 || len(properties) > 0 {
		args = append(args, "--", "set", "Interface", port)
		if ofportRequest > 0 {
			args = append(args, fmt.Sprintf("ofport_request=%d", ofportRequest))
		}
		if len(properties) > 0 {
			args = append(args, properties...)
		}
	}
	_, err := ovsif.exec(OVS_VSCTL, args...)
	if err != nil {
		return -1, err
	}
	ofportStr, err := ovsif.exec(OVS_VSCTL, "get", "Interface", port, "ofport")
	if err != nil {
		return -1, err
	}
	ofport, err := strconv.Atoi(ofportStr)
	if err != nil {
		return -1, fmt.Errorf("Could not parse allocated ofport %q: %v", ofportStr, err)
	}
	if ofportRequest > 0 && ofportRequest != ofport {
		return -1, fmt.Errorf("Allocated ofport (%d) did not match request (%d)", ofport, ofportRequest)
	}
	return ofport, nil
}

// DeletePort removes an interface from the bridge. (It is not an
// error if the interface is not currently a bridge port.)
func (ovsif *Interface) DeletePort(port string) error {
	_, err := ovsif.exec(OVS_VSCTL, "--if-exists", "del-port", ovsif.bridge, port)
	return err
}

type Transaction struct {
	ovsif *Interface
	err   error
}

func (tx *Transaction) exec(cmd string, args ...string) (string, error) {
	out := ""
	if tx.err == nil {
		out, tx.err = tx.ovsif.exec(cmd, args...)
	}
	return out, tx.err
}

// NewTransaction begins a new OVS transaction. If an error occurs at
// any step in the transaction, it will be recorded until
// EndTransaction(), and any further calls on the transaction will be
// ignored.
func (ovsif *Interface) NewTransaction() *Transaction {
	return &Transaction{ovsif: ovsif}
}

// AddFlow adds a flow to the bridge. The arguments are passed to fmt.Sprintf().
func (tx *Transaction) AddFlow(flow string, args ...interface{}) {
	if len(args) > 0 {
		flow = fmt.Sprintf(flow, args...)
	}
	tx.exec(OVS_OFCTL, "add-flow", tx.ovsif.bridge, flow)
}

// DeleteFlows deletes all matching flows from the bridge. The arguments are
// passed to fmt.Sprintf().
func (tx *Transaction) DeleteFlows(flow string, args ...interface{}) {
	if len(args) > 0 {
		flow = fmt.Sprintf(flow, args...)
	}
	tx.exec(OVS_OFCTL, "del-flows", tx.ovsif.bridge, flow)
}

// EndTransaction ends an OVS transaction and returns any error that occurred
// during the transaction. You should not use the transaction again after
// calling this function.
func (tx *Transaction) EndTransaction() error {
	err := tx.err
	tx.err = nil
	return err
}

// DumpFlows dumps the flow table for the bridge and returns it as an array of
// strings, one per flow.
func (ovsif *Interface) DumpFlows() ([]string, error) {
	out, err := ovsif.exec(OVS_OFCTL, "dump-flows", ovsif.bridge)
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
