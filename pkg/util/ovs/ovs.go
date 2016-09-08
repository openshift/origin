// Package ovs provides a wrapper around ovs-vsctl and ovs-ofctl
package ovs

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/util/exec"
	utilversion "k8s.io/kubernetes/pkg/util/version"
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
func New(execer exec.Interface, bridge string, minVersion string) (*Interface, error) {
	if _, err := execer.LookPath(OVS_OFCTL); err != nil {
		return nil, fmt.Errorf("OVS is not installed")
	}
	if _, err := execer.LookPath(OVS_VSCTL); err != nil {
		return nil, fmt.Errorf("OVS is not installed")
	}

	ovsif := &Interface{execer: execer, bridge: bridge}

	if minVersion != "" {
		minVer := utilversion.MustParseGeneric(minVersion)

		out, err := ovsif.exec(OVS_VSCTL, "--version")
		if err != nil {
			return nil, fmt.Errorf("could not check OVS version is %s or higher", minVersion)
		}
		// First output line should end with version
		lines := strings.Split(out, "\n")
		spc := strings.LastIndex(lines[0], " ")
		instVer, err := utilversion.ParseGeneric(lines[0][spc+1:])
		if err != nil {
			return nil, fmt.Errorf("could not find OVS version in %q", lines[0])
		}
		if !instVer.AtLeast(minVer) {
			return nil, fmt.Errorf("found OVS %v, need %s or later", instVer, minVersion)
		}
	}

	return ovsif, nil
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

// GetOFPort returns the OpenFlow port number of a given network interface
// attached to a bridge.
func (ovsif *Interface) GetOFPort(port string) (int, error) {
	ofportStr, err := ovsif.exec(OVS_VSCTL, "get", "Interface", port, "ofport")
	if err != nil {
		return -1, err
	}
	ofport, err := strconv.Atoi(ofportStr)
	if err != nil {
		return -1, fmt.Errorf("Could not parse allocated ofport %q: %v", ofportStr, err)
	}
	return ofport, nil
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
	ofport, err := ovsif.GetOFPort(port)
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

func (ovsif *Interface) SetFrags(mode string) error {
	_, err := ovsif.exec(OVS_OFCTL, "set-frags", ovsif.bridge, mode)
	return err
}

// Create creates a record in the OVS database, as with "ovs-vsctl create" and
// returns the UUID of the newly-created item.
// NOTE: This only works for QoS; for all other tables the created object will
// immediately be garbage-collected; we'd need an API that calls "create" and "set"
// in the same "ovs-vsctl" call.
func (ovsif *Interface) Create(table string, values ...string) (string, error) {
	args := append([]string{"create", table}, values...)
	return ovsif.exec(OVS_VSCTL, args...)
}

// Destroy deletes the indicated record in the OVS database. It is not an error if
// the record does not exist
func (ovsif *Interface) Destroy(table, record string) error {
	_, err := ovsif.exec(OVS_VSCTL, "--if-exists", "destroy", table, record)
	return err
}

// Get gets the indicated value from the OVS database. For multi-valued or
// map-valued columns, the data is returned in the same format as "ovs-vsctl get".
func (ovsif *Interface) Get(table, record, column string) (string, error) {
	return ovsif.exec(OVS_VSCTL, "get", table, record, column)
}

// Set sets one or more columns on a record in the OVS database, as with
// "ovs-vsctl set"
func (ovsif *Interface) Set(table, record string, values ...string) error {
	args := append([]string{"set", table, record}, values...)
	_, err := ovsif.exec(OVS_VSCTL, args...)
	return err
}

// Clear unsets the indicated columns in the OVS database. It is not an error if
// the value is already unset
func (ovsif *Interface) Clear(table, record string, columns ...string) error {
	args := append([]string{"--if-exists", "clear", table, record}, columns...)
	_, err := ovsif.exec(OVS_VSCTL, args...)
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
