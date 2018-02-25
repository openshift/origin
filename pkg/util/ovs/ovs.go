package ovs

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/golang/glog"

	utilversion "k8s.io/kubernetes/pkg/util/version"
	"k8s.io/utils/exec"
)

// Interface represents an interface to OVS
type Interface interface {
	// AddBridge creates the bridge associated with the interface, optionally setting
	// properties on it (as with "ovs-vsctl set Bridge ..."). If the bridge already
	// exists this errors.
	AddBridge(properties ...string) error

	// DeleteBridge deletes the bridge associated with the interface. The boolean
	// that can be passed determines if a bridge not existing is an error. Passing
	// true will delete bridge --if-exists, passing false will error if the bridge
	// does not exist.
	DeleteBridge(ifExists bool) error

	// AddPort adds an interface to the bridge, requesting the indicated port
	// number, and optionally setting properties on it (as with "ovs-vsctl set
	// Interface ..."). Returns the allocated port number (or an error).
	AddPort(port string, ofportRequest int, properties ...string) (int, error)

	// DeletePort removes an interface from the bridge. (It is not an
	// error if the interface is not currently a bridge port.)
	DeletePort(port string) error

	// GetOFPort returns the OpenFlow port number of a given network interface
	// attached to a bridge.
	GetOFPort(port string) (int, error)

	// SetFrags sets the fragmented-packet-handling mode (as with
	// "ovs-ofctl set-frags")
	SetFrags(mode string) error

	// Create creates a record in the OVS database, as with "ovs-vsctl create" and
	// returns the UUID of the newly-created item.
	// NOTE: This only works for QoS; for all other tables the created object will
	// immediately be garbage-collected; we'd need an API that calls "create" and "set"
	// in the same "ovs-vsctl" call.
	Create(table string, values ...string) (string, error)

	// Destroy deletes the indicated record in the OVS database. It is not an error if
	// the record does not exist
	Destroy(table, record string) error

	// Get gets the indicated value from the OVS database. For multi-valued or
	// map-valued columns, the data is returned in the same format as "ovs-vsctl get".
	Get(table, record, column string) (string, error)

	// Set sets one or more columns on a record in the OVS database, as with
	// "ovs-vsctl set"
	Set(table, record string, values ...string) error

	// Clear unsets the indicated columns in the OVS database. It is not an error if
	// the value is already unset
	Clear(table, record string, columns ...string) error

	// Find finds records in the OVS database that match the given condition.
	// It returns the value of the given column of matching records.
	Find(table, column, condition string) ([]string, error)

	// DumpFlows dumps the flow table for the bridge and returns it as an array of
	// strings, one per flow. If flow is not "" then it describes the flows to dump.
	DumpFlows(flow string, args ...interface{}) ([]string, error)

	// NewTransaction begins a new OVS transaction. If an error occurs at
	// any step in the transaction, it will be recorded until
	// EndTransaction(), and any further calls on the transaction will be
	// ignored.
	NewTransaction() Transaction
}

// Transaction manages a single set of OVS flow modifications
type Transaction interface {
	// AddFlow adds a flow to the bridge. The arguments are passed to fmt.Sprintf().
	AddFlow(flow string, args ...interface{})

	// DeleteFlows deletes all matching flows from the bridge. The arguments are
	// passed to fmt.Sprintf().
	DeleteFlows(flow string, args ...interface{})

	// EndTransaction ends an OVS transaction and returns any error that occurred
	// during the transaction. You should not use the transaction again after
	// calling this function.
	EndTransaction() error
}

const (
	OVS_OFCTL = "ovs-ofctl"
	OVS_VSCTL = "ovs-vsctl"
)

// ovsExec implements ovs.Interface via calls to ovs-ofctl and ovs-vsctl
type ovsExec struct {
	execer exec.Interface
	bridge string
}

// New returns a new ovs.Interface
func New(execer exec.Interface, bridge string, minVersion string) (Interface, error) {
	if _, err := execer.LookPath(OVS_OFCTL); err != nil {
		return nil, fmt.Errorf("OVS is not installed")
	}
	if _, err := execer.LookPath(OVS_VSCTL); err != nil {
		return nil, fmt.Errorf("OVS is not installed")
	}

	ovsif := &ovsExec{execer: execer, bridge: bridge}

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

func (ovsif *ovsExec) exec(cmd string, args ...string) (string, error) {
	if cmd == OVS_OFCTL {
		args = append([]string{"-O", "OpenFlow13"}, args...)
	}
	glog.V(5).Infof("Executing: %s %s", cmd, strings.Join(args, " "))

	output, err := ovsif.execer.Command(cmd, args...).CombinedOutput()
	if err != nil {
		glog.V(2).Infof("Error executing %s: %s", cmd, string(output))
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

func (ovsif *ovsExec) AddBridge(properties ...string) error {
	args := []string{"add-br", ovsif.bridge}
	if len(properties) > 0 {
		args = append(args, "--", "set", "Bridge", ovsif.bridge)
		args = append(args, properties...)
	}
	_, err := ovsif.exec(OVS_VSCTL, args...)
	return err
}

func (ovsif *ovsExec) DeleteBridge(ifExists bool) error {
	args := []string{"del-br", ovsif.bridge}

	if ifExists {
		args = append([]string{"--if-exists"}, args...)
	}
	_, err := ovsif.exec(OVS_VSCTL, args...)
	return err
}

func (ovsif *ovsExec) GetOFPort(port string) (int, error) {
	ofportStr, err := ovsif.exec(OVS_VSCTL, "get", "Interface", port, "ofport")
	if err != nil {
		return -1, fmt.Errorf("failed to get OVS port for %s: %v", port, err)
	}
	ofport, err := strconv.Atoi(ofportStr)
	if err != nil {
		return -1, fmt.Errorf("could not parse allocated ofport %q: %v", ofportStr, err)
	}
	if ofport == -1 {
		errStr, err := ovsif.exec(OVS_VSCTL, "get", "Interface", port, "error")
		if err != nil || errStr == "" {
			errStr = "unknown error"
		}
		return -1, fmt.Errorf("error on port %s: %s", port, errStr)
	}
	return ofport, nil
}

func (ovsif *ovsExec) AddPort(port string, ofportRequest int, properties ...string) (int, error) {
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
	if err != nil {
		return -1, err
	}
	if ofportRequest > 0 && ofportRequest != ofport {
		return -1, fmt.Errorf("allocated ofport (%d) did not match request (%d)", ofport, ofportRequest)
	}
	return ofport, nil
}

func (ovsif *ovsExec) DeletePort(port string) error {
	_, err := ovsif.exec(OVS_VSCTL, "--if-exists", "del-port", ovsif.bridge, port)
	return err
}

func (ovsif *ovsExec) SetFrags(mode string) error {
	_, err := ovsif.exec(OVS_OFCTL, "set-frags", ovsif.bridge, mode)
	return err
}

func (ovsif *ovsExec) Create(table string, values ...string) (string, error) {
	args := append([]string{"create", table}, values...)
	return ovsif.exec(OVS_VSCTL, args...)
}

func (ovsif *ovsExec) Destroy(table, record string) error {
	_, err := ovsif.exec(OVS_VSCTL, "--if-exists", "destroy", table, record)
	return err
}

func (ovsif *ovsExec) Get(table, record, column string) (string, error) {
	return ovsif.exec(OVS_VSCTL, "get", table, record, column)
}

func (ovsif *ovsExec) Set(table, record string, values ...string) error {
	args := append([]string{"set", table, record}, values...)
	_, err := ovsif.exec(OVS_VSCTL, args...)
	return err
}

// Returns the given column of records that match the condition
func (ovsif *ovsExec) Find(table, column, condition string) ([]string, error) {
	output, err := ovsif.exec(OVS_VSCTL, "--no-heading", "--columns="+column, "find", table, condition)
	if err != nil {
		return nil, err
	}
	values := strings.Split(output, "\n\n")
	// We want "bare" values for strings, but we can't pass --bare to ovs-vsctl because
	// it breaks more complicated types. So try passing each value through Unquote();
	// if it fails, that means the value wasn't a quoted string, so use it as-is.
	for i, val := range values {
		if unquoted, err := strconv.Unquote(val); err == nil {
			values[i] = unquoted
		}
	}
	return values, nil
}

func (ovsif *ovsExec) Clear(table, record string, columns ...string) error {
	args := append([]string{"--if-exists", "clear", table, record}, columns...)
	_, err := ovsif.exec(OVS_VSCTL, args...)
	return err
}

type ovsExecTx struct {
	ovsif *ovsExec
	err   error
}

func (tx *ovsExecTx) exec(cmd string, args ...string) (string, error) {
	out := ""
	if tx.err == nil {
		out, tx.err = tx.ovsif.exec(cmd, args...)
	}
	return out, tx.err
}

func (ovsif *ovsExec) NewTransaction() Transaction {
	return &ovsExecTx{ovsif: ovsif}
}

func (tx *ovsExecTx) AddFlow(flow string, args ...interface{}) {
	if len(args) > 0 {
		flow = fmt.Sprintf(flow, args...)
	}
	tx.exec(OVS_OFCTL, "add-flow", tx.ovsif.bridge, flow)
}

func (tx *ovsExecTx) DeleteFlows(flow string, args ...interface{}) {
	if len(args) > 0 {
		flow = fmt.Sprintf(flow, args...)
	}
	tx.exec(OVS_OFCTL, "del-flows", tx.ovsif.bridge, flow)
}

func (tx *ovsExecTx) EndTransaction() error {
	err := tx.err
	tx.err = nil
	return err
}

func (ovsif *ovsExec) DumpFlows(flow string, args ...interface{}) ([]string, error) {
	if len(args) > 0 {
		flow = fmt.Sprintf(flow, args...)
	}
	out, err := ovsif.exec(OVS_OFCTL, "dump-flows", ovsif.bridge, flow)
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
