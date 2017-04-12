package ovs

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ovsFake implements a fake ovs.Interface for testing purposes
//
// Note that the code here is *not* expected to be 100% equivalent to ovsExec, as
// that would require porting over huge amounts of ovs-ofctl source code. It needs
// to support enough features to make the SDN unit tests pass, and should do enough
// error checking to catch bugs that have tripped us up in the past (eg,
// specifying "nw_dst" without "ip").
type ovsFake struct {
	bridge string

	ports map[string]int
	flows []ovsFlow
}

// ovsFlow represents an OVS flow
type ovsFlow struct {
	table    int
	priority int
	created  time.Time
	cookie   string
	fields   []ovsField
	actions  string
}

type ovsField struct {
	name  string
	value string
}

const (
	minPriority     = 0
	defaultPriority = 32768
	maxPriority     = 65535
)

// NewFake returns a new ovs.Interface
func NewFake(bridge string) Interface {
	return &ovsFake{bridge: bridge}
}

func (fake *ovsFake) AddBridge(properties ...string) error {
	fake.ports = make(map[string]int)
	fake.flows = make([]ovsFlow, 0)
	return nil
}

func (fake *ovsFake) DeleteBridge() error {
	fake.ports = nil
	fake.flows = nil
	return nil
}

func (fake *ovsFake) ensureExists() error {
	if fake.ports == nil {
		return fmt.Errorf("no bridge named %s", fake.bridge)
	}
	return nil
}

func (fake *ovsFake) GetOFPort(port string) (int, error) {
	if err := fake.ensureExists(); err != nil {
		return -1, err
	}

	if ofport, exists := fake.ports[port]; exists {
		return ofport, nil
	} else {
		return -1, fmt.Errorf("no row %q in table Interface", port)
	}
}

func (fake *ovsFake) AddPort(port string, ofportRequest int, properties ...string) (int, error) {
	if err := fake.ensureExists(); err != nil {
		return -1, err
	}

	ofport, exists := fake.ports[port]
	if exists {
		if ofport != ofportRequest && ofportRequest != -1 {
			return -1, fmt.Errorf("allocated ofport (%d) did not match request (%d)", ofport, ofportRequest)
		}
	} else {
		if ofportRequest == -1 {
			ofport := 1
			for _, existingPort := range fake.ports {
				if existingPort >= ofport {
					ofport = existingPort + 1
				}
			}
		} else {
			if ofportRequest < 1 || ofportRequest > 65535 {
				return -1, fmt.Errorf("requested ofport (%d) out of range", ofportRequest)
			}
			ofport = ofportRequest
		}
		fake.ports[port] = ofport
	}
	return ofport, nil
}

func (fake *ovsFake) DeletePort(port string) error {
	if err := fake.ensureExists(); err != nil {
		return err
	}

	delete(fake.ports, port)
	return nil
}

func (fake *ovsFake) SetFrags(mode string) error {
	return nil
}

func (ovsif *ovsFake) Create(table string, values ...string) (string, error) {
	return "fake-UUID", nil
}

func (fake *ovsFake) Destroy(table, record string) error {
	return nil
}

func (fake *ovsFake) Get(table, record, column string) (string, error) {
	return "", nil
}

func (fake *ovsFake) Set(table, record string, values ...string) error {
	return nil
}

func (fake *ovsFake) Clear(table, record string, columns ...string) error {
	return nil
}

type ovsFakeTx struct {
	fake *ovsFake
	err  error
}

func (fake *ovsFake) NewTransaction() Transaction {
	return &ovsFakeTx{fake: fake, err: fake.ensureExists()}
}

type parseCmd string

const (
	parseForAdd    parseCmd = "add-flow"
	parseForDelete parseCmd = "del-flows"
)

func fieldSet(parsed *ovsFlow, field string) bool {
	for _, f := range parsed.fields {
		if f.name == field {
			return true
		}
	}
	return false
}

func checkNotAllowedField(flow string, parsed *ovsFlow, field string, cmd parseCmd) error {
	if fieldSet(parsed, field) {
		return fmt.Errorf("bad flow %q (field %q not allowed in %s)", flow, field, cmd)
	}
	return nil
}

func checkUnsupportedField(flow string, parsed *ovsFlow, field string) error {
	if fieldSet(parsed, field) {
		return fmt.Errorf("bad flow %q (field %q not supported)", flow, field)
	}
	return nil
}

func parseFlow(cmd parseCmd, flow string, args ...interface{}) (*ovsFlow, error) {
	if len(args) > 0 {
		flow = fmt.Sprintf(flow, args...)
	}

	// According to the man page, "flow descriptions comprise a series of field=value
	// assignments, separated by commas or white space." However, you can also have
	// fields with no value (eg, "ip"), and the "actions" field can also have internal
	// commas, whitespace, and equals signs (but if it is present, it must be the
	// last field specified).

	parsed := &ovsFlow{
		table:    0,
		priority: defaultPriority,
		fields:   make([]ovsField, 0),
		created:  time.Now(),
		cookie:   "0",
	}
	flow = strings.Trim(flow, " ")
	origFlow := flow
	for flow != "" {
		field := ""
		value := ""

		end := strings.IndexAny(flow, ", ")
		if end == -1 {
			end = len(flow)
		}
		eq := strings.Index(flow, "=")
		if eq == -1 || eq > end {
			field = flow[:end]
		} else {
			field = flow[:eq]
			if field == "actions" {
				end = len(flow)
				value = flow[eq+1:]
			} else {
				valueEnd := end - 1
				for flow[valueEnd] == ' ' || flow[valueEnd] == ',' {
					valueEnd--
				}
				value = strings.Trim(flow[eq+1:end], ", ")
			}
			if value == "" {
				return nil, fmt.Errorf("bad flow definition %q (empty field %q)", origFlow, field)
			}
		}

		switch field {
		case "table":
			table, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("bad flow %q (bad table number %q)", origFlow, value)
			} else if table < 0 || table > 255 {
				return nil, fmt.Errorf("bad flow %q (table number %q out of range)", origFlow, value)
			}
			parsed.table = table
		case "priority":
			priority, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("bad flow %q (bad priority %q)", origFlow, value)
			} else if priority < minPriority || priority > maxPriority {
				return nil, fmt.Errorf("bad flow %q (priority %q out of range)", origFlow, value)
			}
			parsed.priority = priority
		case "actions":
			parsed.actions = value
		case "cookie":
			parsed.cookie = value
		default:
			parsed.fields = append(parsed.fields, ovsField{field, value})
		}

		for end < len(flow) && (flow[end] == ',' || flow[end] == ' ') {
			end++
		}
		flow = flow[end:]
	}

	// Sanity-checking
	switch cmd {
	case parseForAdd:
		if err := checkNotAllowedField(flow, parsed, "out_port", cmd); err != nil {
			return nil, err
		}
		if err := checkNotAllowedField(flow, parsed, "out_group", cmd); err != nil {
			return nil, err
		}

		if parsed.actions == "" {
			return nil, fmt.Errorf("bad flow %q (empty actions)", flow)
		}

	case parseForDelete:
		if err := checkNotAllowedField(flow, parsed, "priority", cmd); err != nil {
			return nil, err
		}
		if err := checkNotAllowedField(flow, parsed, "actions", cmd); err != nil {
			return nil, err
		}
		if err := checkUnsupportedField(flow, parsed, "out_port"); err != nil {
			return nil, err
		}
		if err := checkUnsupportedField(flow, parsed, "out_group"); err != nil {
			return nil, err
		}
	}

	if (fieldSet(parsed, "nw_src") || fieldSet(parsed, "nw_dst")) &&
		!(fieldSet(parsed, "ip") || fieldSet(parsed, "arp") || fieldSet(parsed, "tcp") || fieldSet(parsed, "udp")) {
		return nil, fmt.Errorf("bad flow %q (specified nw_src/nw_dst without ip/arp/tcp/udp)", flow)
	}
	if (fieldSet(parsed, "arp_spa") || fieldSet(parsed, "arp_tpa") || fieldSet(parsed, "arp_sha") || fieldSet(parsed, "arp_tha")) && !fieldSet(parsed, "arp") {
		return nil, fmt.Errorf("bad flow %q (specified arp_spa/arp_tpa/arp_sha/arp_tpa without arp)", flow)
	}
	if (fieldSet(parsed, "tcp_src") || fieldSet(parsed, "tcp_dst")) && !fieldSet(parsed, "tcp") {
		return nil, fmt.Errorf("bad flow %q (specified tcp_src/tcp_dst without tcp)", flow)
	}
	if (fieldSet(parsed, "udp_src") || fieldSet(parsed, "udp_dst")) && !fieldSet(parsed, "udp") {
		return nil, fmt.Errorf("bad flow %q (specified udp_src/udp_dst without udp)", flow)
	}
	if (fieldSet(parsed, "tp_src") || fieldSet(parsed, "tp_dst")) && !(fieldSet(parsed, "tcp") || fieldSet(parsed, "udp")) {
		return nil, fmt.Errorf("bad flow %q (specified tp_src/tp_dst without tcp/udp)", flow)
	}
	if fieldSet(parsed, "ip_frag") && (fieldSet(parsed, "tcp") || fieldSet(parsed, "udp")) {
		return nil, fmt.Errorf("bad flow %q (specified ip_frag with tcp/udp)", flow)
	}

	return parsed, nil
}

// flowMatches tests if flow matches match. If exact is true, then the table, priority,
// and all fields much match. If exact is false, then the table and any fields specified
// in match must match, but priority is not checked, and there can be additional fields
// in flow that are not in match.
func flowMatches(flow, match *ovsFlow, exact bool) bool {
	if flow.table != match.table && (exact || match.table != 0) {
		return false
	}
	if exact && flow.priority != match.priority {
		return false
	}
	if exact && len(flow.fields) != len(match.fields) {
		return false
	}
	if match.cookie != "" && !fieldMatches(flow.cookie, match.cookie, exact) {
		return false
	}
	for _, matchField := range match.fields {
		var flowValue *string
		for _, flowField := range flow.fields {
			if flowField.name == matchField.name {
				flowValue = &flowField.value
				break
			}
		}
		if flowValue == nil || !fieldMatches(*flowValue, matchField.value, exact) {
			return false
		}
	}
	return true
}

func fieldMatches(val, match string, exact bool) bool {
	if val == match {
		return true
	}
	if exact {
		return false
	}

	// Handle bitfield/mask matches. (Some other syntax like "10.128.0.0/14" might
	// get examined here, but it will fail the first ParseUint call and so not
	// reach the final check.)
	split := strings.Split(match, "/")
	if len(split) == 2 {
		matchNum, err1 := strconv.ParseUint(split[0], 0, 32)
		mask, err2 := strconv.ParseUint(split[1], 0, 32)
		valNum, err3 := strconv.ParseUint(val, 0, 32)
		if err1 == nil && err2 == nil && err3 == nil {
			if (matchNum & mask) == (valNum & mask) {
				return true
			}
		}
	}

	return false
}

// sort.Interface support
type ovsFlows []ovsFlow

func (f ovsFlows) Len() int      { return len(f) }
func (f ovsFlows) Swap(i, j int) { f[i], f[j] = f[j], f[i] }
func (f ovsFlows) Less(i, j int) bool {
	if f[i].table != f[j].table {
		return f[i].table < f[j].table
	}
	if f[i].priority != f[j].priority {
		return f[i].priority > f[j].priority
	}
	return f[i].created.Before(f[j].created)
}

func (tx *ovsFakeTx) AddFlow(flow string, args ...interface{}) {
	if tx.err != nil {
		return
	}
	parsed, err := parseFlow(parseForAdd, flow, args...)
	if err != nil {
		tx.err = err
		return
	}

	// If there is already an exact match for this flow, then the new flow replaces it.
	for i := range tx.fake.flows {
		if flowMatches(&tx.fake.flows[i], parsed, true) {
			tx.fake.flows[i] = *parsed
			return
		}
	}

	tx.fake.flows = append(tx.fake.flows, *parsed)
	sort.Sort(ovsFlows(tx.fake.flows))
}

func (tx *ovsFakeTx) DeleteFlows(flow string, args ...interface{}) {
	if tx.err != nil {
		return
	}
	parsed, err := parseFlow(parseForDelete, flow, args...)
	if err != nil {
		tx.err = err
		return
	}

	newFlows := make([]ovsFlow, 0, len(tx.fake.flows))
	for _, flow := range tx.fake.flows {
		if !flowMatches(&flow, parsed, false) {
			newFlows = append(newFlows, flow)
		}
	}
	tx.fake.flows = newFlows
}

func (tx *ovsFakeTx) EndTransaction() error {
	err := tx.err
	tx.err = nil
	return err
}

func (fake *ovsFake) DumpFlows() ([]string, error) {
	if err := fake.ensureExists(); err != nil {
		return nil, err
	}

	flows := make([]string, 0, len(fake.flows))
	for _, flow := range fake.flows {
		str := fmt.Sprintf(" cookie=%s, table=%d", flow.cookie, flow.table)
		if flow.priority != defaultPriority {
			str += fmt.Sprintf(", priority=%d", flow.priority)
		}
		for _, field := range flow.fields {
			if field.value == "" {
				str += fmt.Sprintf(", %s", field.name)
			} else {
				str += fmt.Sprintf(", %s=%s", field.name, field.value)
			}
		}
		if flow.actions != "" {
			str += fmt.Sprintf(", actions=%s", flow.actions)
		}
		flows = append(flows, str)
	}

	return flows, nil
}
