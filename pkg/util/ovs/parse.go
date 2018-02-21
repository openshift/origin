package ovs

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ovsFlow represents an OVS flow
type OvsFlow struct {
	Table    int
	Priority int
	Created  time.Time
	Cookie   string
	Fields   []OvsField
	Actions  []OvsField

	ptype ParseType
}

type OvsField struct {
	Name  string
	Value string
}

const (
	minPriority     = 0
	defaultPriority = 32768
	maxPriority     = 65535
)

type ParseType string

const (
	ParseForAdd    ParseType = "add"
	ParseForFilter ParseType = "filter"
	ParseForDump   ParseType = "dump"
)

func fieldSet(parsed *OvsFlow, field string) bool {
	for _, f := range parsed.Fields {
		if f.Name == field {
			return true
		}
	}
	return false
}

func checkNotAllowedField(flow string, parsed *OvsFlow, field string, ptype ParseType) error {
	if fieldSet(parsed, field) {
		return fmt.Errorf("bad flow %q (field %q not allowed in %s)", flow, field, ptype)
	}
	return nil
}

func checkUnimplementedField(flow string, parsed *OvsFlow, field string) error {
	if fieldSet(parsed, field) {
		return fmt.Errorf("bad flow %q (field %q not implemented)", flow, field)
	}
	return nil
}

func actionToOvsField(action string) (*OvsField, error) {
	if action == "" {
		return nil, fmt.Errorf("cannot make field from empty action")
	}
	sep := strings.IndexAny(action, ":(")
	if sep == -1 {
		return &OvsField{Name: strings.TrimSpace(action)}, nil
	} else if sep == len(action)-1 {
		return nil, fmt.Errorf("action %q has no value", action)
	}
	return &OvsField{
		Name:  strings.TrimSpace(action[:sep]),
		Value: strings.Trim(action[sep:], ": "),
	}, nil
}

func parseActions(actions string) ([]OvsField, error) {
	fields := make([]OvsField, 0)
	var parenLevel, braceLevel, idx int
	origActions := actions
	for actions != "" {
		token := strings.IndexAny(actions[idx:], ",([])")
		if token == -1 {
			if parenLevel > 0 {
				return nil, fmt.Errorf("mismatched parentheses in actions %q", origActions)
			} else if braceLevel > 0 {
				return nil, fmt.Errorf("mismatched braces in actions %q", origActions)
			}
			field, err := actionToOvsField(actions)
			if err != nil {
				return nil, err
			}
			fields = append(fields, *field)
			break
		}
		idx += token

		switch actions[idx] {
		case ',':
			if parenLevel == 0 && braceLevel == 0 {
				field, err := actionToOvsField(actions[:idx])
				if err != nil {
					return nil, err
				}
				fields = append(fields, *field)
				actions = actions[idx+1:]
				idx = 0
			}
		case '(':
			parenLevel += 1
		case '[':
			braceLevel += 1
		case ')':
			parenLevel -= 1
			if parenLevel < 0 {
				return nil, fmt.Errorf("mismatched parentheses in actions %q", origActions)
			}
		case ']':
			braceLevel -= 1
			if braceLevel < 0 {
				return nil, fmt.Errorf("mismatched braces in actions %q", origActions)
			}
		}
		// Advance past token
		idx += 1
	}
	return fields, nil
}

func findField(name string, fields *[]OvsField) (*OvsField, bool) {
	for _, f := range *fields {
		if f.Name == name {
			return &f, true
		}
	}
	return nil, false
}

func (of *OvsFlow) FindField(name string) (*OvsField, bool) {
	return findField(name, &of.Fields)
}

func (of *OvsFlow) FindAction(name string) (*OvsField, bool) {
	return findField(name, &of.Actions)
}

func (of *OvsFlow) NoteHasPrefix(prefix string) bool {
	note, ok := of.FindAction("note")
	return ok && strings.HasPrefix(strings.ToLower(note.Value), strings.ToLower(prefix))
}

func ParseFlow(ptype ParseType, flow string, args ...interface{}) (*OvsFlow, error) {
	if len(args) > 0 {
		flow = fmt.Sprintf(flow, args...)
	}

	// According to the man page, "flow descriptions comprise a series of field=value
	// assignments, separated by commas or white space." However, you can also have
	// fields with no value (eg, "ip"), and the "actions" field can also have internal
	// commas, whitespace, and equals signs (but if it is present, it must be the
	// last field specified).

	parsed := &OvsFlow{
		Table:    -1,
		Priority: -1,
		Fields:   make([]OvsField, 0),
		Actions:  make([]OvsField, 0),
		Created:  time.Now(),
		ptype:    ptype,
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
			parsed.Table = table
		case "priority":
			priority, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("bad flow %q (bad priority %q)", origFlow, value)
			} else if priority < minPriority || priority > maxPriority {
				return nil, fmt.Errorf("bad flow %q (priority %q out of range)", origFlow, value)
			}
			parsed.Priority = priority
		case "actions":
			var err error
			parsed.Actions, err = parseActions(value)
			if err != nil {
				return nil, err
			}
		case "cookie":
			parsed.Cookie = value
		default:
			parsed.Fields = append(parsed.Fields, OvsField{field, value})
		}

		for end < len(flow) && (flow[end] == ',' || flow[end] == ' ') {
			end++
		}
		flow = flow[end:]
	}

	// Sanity-checking and defaults
	switch ptype {
	case ParseForAdd:
		if err := checkNotAllowedField(flow, parsed, "out_port", ptype); err != nil {
			return nil, err
		}
		if err := checkNotAllowedField(flow, parsed, "out_group", ptype); err != nil {
			return nil, err
		}

		if len(parsed.Actions) == 0 {
			return nil, fmt.Errorf("bad flow %q (empty actions)", flow)
		}

		if parsed.Table == -1 {
			parsed.Table = 0
		}
		if parsed.Priority == -1 {
			parsed.Priority = defaultPriority
		}
		if parsed.Cookie == "" {
			parsed.Cookie = "0"
		}

	case ParseForFilter:
		if err := checkNotAllowedField(flow, parsed, "priority", ptype); err != nil {
			return nil, err
		}
		if err := checkUnimplementedField(flow, parsed, "out_port"); err != nil {
			return nil, err
		}
		if err := checkUnimplementedField(flow, parsed, "out_group"); err != nil {
			return nil, err
		}

		if len(parsed.Actions) != 0 {
			return nil, fmt.Errorf("bad flow %q (field %q not allowed in %s)", flow, "actions", ptype)
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

// flowMatches tests if flow matches match. If match.ptype is ParseForAdd, then the table,
// priority, and all fields much match. If it is ParseForFilter, then any fields specified in
// match must match, but there can be additional fields in flow that are not in match.
func FlowMatches(flow, match *OvsFlow) bool {
	if match.ptype == ParseForAdd || match.Table != -1 {
		if flow.Table != match.Table {
			return false
		}
	}
	if match.ptype == ParseForAdd || match.Priority != -1 {
		if flow.Priority != match.Priority {
			return false
		}
	}
	if match.ptype == ParseForAdd || match.Cookie != "" {
		if !fieldMatches(flow.Cookie, match.Cookie, match.ptype) {
			return false
		}
	}
	if match.ptype == ParseForAdd && len(flow.Fields) != len(match.Fields) {
		return false
	}
	for _, matchField := range match.Fields {
		var flowValue *string
		for _, flowField := range flow.Fields {
			if flowField.Name == matchField.Name {
				flowValue = &flowField.Value
				break
			}
		}
		if flowValue == nil || !fieldMatches(*flowValue, matchField.Value, match.ptype) {
			return false
		}
	}
	return true
}

func fieldMatches(val, match string, ptype ParseType) bool {
	if val == match {
		return true
	}
	if ptype == ParseForAdd {
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

func ParseExternalIDs(externalIDs string) (map[string]string, error) {
	ids := make(map[string]string, 1)
	// Output from "find" and "list" will have braces, but input to "set" won't
	if externalIDs[0] == '{' && externalIDs[len(externalIDs)-1] == '}' {
		externalIDs = externalIDs[1 : len(externalIDs)-1]
	}
	for _, id := range strings.Split(externalIDs, ",") {
		parsed := strings.Split(strings.TrimSpace(id), "=")
		if len(parsed) != 2 {
			return nil, fmt.Errorf("could not parse external-id %q", id)
		}
		key := parsed[0]
		value := parsed[1]
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		}
		ids[key] = value
	}
	return ids, nil
}

func UnparseExternalIDs(externalIDs map[string]string) string {
	ids := []string{}
	for key, value := range externalIDs {
		ids = append(ids, key+"="+strconv.Quote(value))
	}
	return "{" + strings.Join(ids, ",") + "}"
}
