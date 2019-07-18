package ovs

import (
	"reflect"
	"strings"
	"testing"
)

type flowTest struct {
	input string
	match OvsFlow
}

func TestParseFlows(t *testing.T) {
	parseTests := []flowTest{
		{
			input: "table=0, priority=200, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
			match: OvsFlow{
				Table:    0,
				Priority: 200,
				Fields: []OvsField{
					{Name: "in_port", Value: "1"},
					{Name: "arp", Value: ""},
					{Name: "nw_src", Value: "10.128.0.0/14"},
					{Name: "nw_dst", Value: "10.128.0.0/23"},
				},
				Actions: []OvsField{
					{Name: "move", Value: "NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[]"},
					{Name: "goto_table", Value: "10"},
				},
			},
		},
		{
			input: "table=10, priority=0, actions=drop",
			match: OvsFlow{
				Table:    10,
				Priority: 0,
				Fields:   []OvsField{},
				Actions: []OvsField{
					{Name: "drop"},
				},
			},
		},
		{
			input: "table=50, priority=100, arp, nw_dst=10.128.0.0/23, actions=load:0x1234->NXM_NX_TUN_ID[0..31],set_field:172.17.0.2->tun_dst,match:1",
			match: OvsFlow{
				Table:    50,
				Priority: 100,
				Fields: []OvsField{
					{Name: "arp", Value: ""},
					{Name: "nw_dst", Value: "10.128.0.0/23"},
				},
				Actions: []OvsField{
					{Name: "load", Value: "0x1234->NXM_NX_TUN_ID[0..31]"},
					{Name: "set_field", Value: "172.17.0.2->tun_dst"},
					{Name: "match", Value: "1"},
				},
			},
		},
		{
			input: "table=21, priority=100, ip, actions=ct(commit,table=30)",
			match: OvsFlow{
				Table:    21,
				Priority: 100,
				Fields: []OvsField{
					{Name: "ip", Value: ""},
				},
				Actions: []OvsField{
					{Name: "ct(commit,table=30)"},
				},
			},
		},
		{
			// everything after actions is considered part of actions; this would be a syntax error if we actually parsed actions
			input: "table=10, priority=0, actions=drop, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23",
			match: OvsFlow{
				Table:    10,
				Priority: 0,
				Fields:   []OvsField{},
				Actions: []OvsField{
					{Name: "drop"},
					{Name: "in_port=1"},
					{Name: "arp"},
					{Name: "nw_src=10.128.0.0/14"},
					{Name: "nw_dst=10.128.0.0/23"},
				},
			},
		},
	}

	for i, test := range parseTests {
		parsed, err := ParseFlow(ParseForAdd, test.input)
		if err != nil {
			t.Fatalf("unexpected error from ParseFlow: %v", err)
		}
		if !FlowMatches(parsed, &test.match) {
			t.Fatalf("parsed flow %d (%#v) does not match expected output (%#v)", i, parsed, &test.match)
		}
	}
}

func TestParseFlowsDefaults(t *testing.T) {
	parseTests := []flowTest{
		{
			// Default table is 0
			input: "actions=drop",
			match: OvsFlow{
				Table:    0,
				Priority: 32768,
				Fields:   []OvsField{},
				Actions: []OvsField{
					{Name: "drop"},
				},
			},
		},
	}

	for i, test := range parseTests {
		parsed, err := ParseFlow(ParseForAdd, test.input)
		if err != nil {
			t.Fatalf("unexpected error from ParseFlow: %v", err)
		}
		if !FlowMatches(parsed, &test.match) {
			t.Fatalf("parsed flow %d (%#v) does not match expected output (%#v)", i, parsed, &test.match)
		}
	}
}

func TestParseFlowsBad(t *testing.T) {
	parseTests := []flowTest{
		{
			// table is empty
			input: "table=, priority=200, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
		},
		{
			// table is non-numeric
			input: "table=foo, priority=200, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
		},
		{
			// table out of range
			input: "table=-1, priority=200, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
		},
		{
			// table out of range
			input: "table=1000, priority=200, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
		},
		{
			// priority is empty
			input: "table=0, priority=, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
		},
		{
			// priority is non-numeric
			input: "table=0, priority=high, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
		},
		{
			// priority is out of range
			input: "table=0, priority=-1, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
		},
		{
			// priority is out of range
			input: "table=0, priority=200000, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
		},
		{
			// field value is empty
			input: "table=0, priority=200, in_port=, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
		},
		{
			// actions is empty
			input: "table=0, priority=200, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=",
		},
		{
			// actions is empty
			input: "table=0, priority=200, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions",
		},
		{
			// nw_src/nw_dst without arp/ip
			input: "table=0, priority=200, in_port=1, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
		},
	}

	for i, test := range parseTests {
		_, err := ParseFlow(ParseForAdd, test.input)
		if err == nil {
			t.Fatalf("unexpected lack of error from ParseFlow on %d %q", i, test.input)
		}
	}
}

func TestFlowMatchesBad(t *testing.T) {
	parseTests := []flowTest{
		{
			// Table is wrong
			input: "table=0, priority=200, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
			match: OvsFlow{
				Table:    10,
				Priority: 200,
				Fields: []OvsField{
					{Name: "in_port", Value: "1"},
					{Name: "arp", Value: ""},
					{Name: "nw_src", Value: "10.128.0.0/14"},
					{Name: "nw_dst", Value: "10.128.0.0/23"},
				},
				Actions: []OvsField{
					{Name: "move", Value: "NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[]"},
					{Name: "goto_table", Value: "10"},
				},
			},
		},
		{
			// field value is incorrect
			input: "table=0, priority=200, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
			match: OvsFlow{
				Table:    0,
				Priority: 200,
				Fields: []OvsField{
					{Name: "in_port", Value: "2"},
					{Name: "arp", Value: ""},
					{Name: "nw_src", Value: "10.128.0.0/14"},
					{Name: "nw_dst", Value: "10.128.0.0/23"},
				},
				Actions: []OvsField{
					{Name: "move", Value: "NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[]"},
					{Name: "goto_table", Value: "10"},
				},
			},
		},
		{
			// present field is matched against empty field
			input: "table=0, priority=200, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
			match: OvsFlow{
				Table:    0,
				Priority: 200,
				Fields: []OvsField{
					{Name: "in_port", Value: ""},
					{Name: "arp", Value: ""},
					{Name: "nw_src", Value: "10.128.0.0/14"},
					{Name: "nw_dst", Value: "10.128.0.0/23"},
				},
				Actions: []OvsField{
					{Name: "move", Value: "NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[]"},
					{Name: "goto_table", Value: "10"},
				},
			},
		},
		{
			// empty field is matched against present field
			input: "table=0, priority=200, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
			match: OvsFlow{
				Table:    0,
				Priority: 200,
				Fields: []OvsField{
					{Name: "in_port", Value: "1"},
					{Name: "arp", Value: "jean"},
					{Name: "nw_src", Value: "10.128.0.0/14"},
					{Name: "nw_dst", Value: "10.128.0.0/23"},
				},
				Actions: []OvsField{
					{Name: "move", Value: "NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[]"},
					{Name: "goto_table", Value: "10"},
				},
			},
		},
		{
			// match field is not present in input
			input: "table=0, priority=200, in_port=1, arp, nw_src=10.128.0.0/14, nw_dst=10.128.0.0/23, actions=move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
			match: OvsFlow{
				Table:    0,
				Priority: 200,
				Fields: []OvsField{
					{Name: "in_port", Value: "1"},
					{Name: "arp", Value: ""},
					{Name: "nw_src", Value: "10.128.0.0/14"},
					{Name: "nw_dst", Value: "10.128.0.0/23"},
					{Name: "today", Value: "wednesday"},
				},
				Actions: []OvsField{
					{Name: "move", Value: "NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[]"},
					{Name: "goto_table", Value: "10"},
				},
			},
		},
	}

	for i, test := range parseTests {
		parsed, err := ParseFlow(ParseForAdd, test.input)
		if err != nil {
			t.Fatalf("unexpected error from ParseFlow: %v", err)
		}
		if FlowMatches(parsed, &test.match) {
			t.Fatalf("parsed flow %d (%#v) unexpectedly matches output (%#v)", i, parsed, &test.match)
		}
	}
}

func TestParseActions(t *testing.T) {
	parseTests := []struct {
		input  string
		match  []OvsField
		errStr string
	}{
		{
			input: "move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],goto_table:10",
			match: []OvsField{
				{Name: "move", Value: "NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[]"},
				{Name: "goto_table", Value: "10"},
			},
		},
		{
			input: "move:NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[],ct(commit,table=30),goto_table:10",
			match: []OvsField{
				{Name: "move", Value: "NXM_NX_TUN_ID[0..31]->NXM_NX_REG0[]"},
				{Name: "ct", Value: "(commit,table=30)"},
				{Name: "goto_table", Value: "10"},
			},
		},
		{
			// Test that spaces are stripped from name/value
			input: "load:4->NXM_NX_REG0[], note:adsfasdfasdf, goto_table:21",
			match: []OvsField{
				{Name: "load", Value: "4->NXM_NX_REG0[]"},
				{Name: "note", Value: "adsfasdfasdf"},
				{Name: "goto_table", Value: "21"},
			},
		},
		{
			input: "ct(commit,exec(set_field:1->ct_mark),table=70)",
			match: []OvsField{
				{Name: "ct", Value: "(commit,exec(set_field:1->ct_mark),table=70)"},
			},
		},
		{
			input: "drop",
			match: []OvsField{
				{Name: "drop"},
			},
		},
		{
			input: "goto_table:30",
			match: []OvsField{
				{Name: "goto_table", Value: "30"},
			},
		},
		{
			input:  "move:NXM_NX_TUN_ID[0..31]]]],goto_table:10",
			errStr: "mismatched braces in actions",
		},
		{
			input:  "move:NXM_NX_TUN_ID[[[[[0..31],goto_table:10",
			errStr: "mismatched braces in actions",
		},
		{
			input:  "ct(commit,table=30))))),goto_table:10",
			errStr: "mismatched parentheses in action",
		},
		{
			input:  "ct(((((commit,table=30),goto_table:10",
			errStr: "mismatched parentheses in action",
		},
		{
			input:  "goto_table:",
			errStr: "has no value",
		},
		{
			input:  ",,",
			errStr: "cannot make field from empty action",
		},
	}

	for i, test := range parseTests {
		parsed, err := parseActions(test.input)
		if err != nil {
			if test.errStr != "" && !strings.Contains(err.Error(), test.errStr) {
				t.Fatalf("unexpected error from parseActions: %v", err)
			}
		} else if test.errStr != "" {
			t.Fatalf("expected error %q from parseActions", test.errStr)
		}
		if !reflect.DeepEqual(parsed, test.match) {
			t.Fatalf("parsed action %d (%#v) does not match expected output (%#v)", i, parsed, test.match)
		}
	}
}

func TestMatchNote(t *testing.T) {
	noteTests := []struct {
		input   string
		prefix  string
		success bool
		errStr  string
	}{
		{
			// Prefix match
			input:   "table=10,actions=note:00.33.22.33.00.00.00",
			prefix:  "00.33.22.33.00",
			success: true,
		},
		{
			input:   "table=10,actions=note:00.33.22.33.00.00.00",
			prefix:  "00.bb.dd.ee",
			success: false,
		},
		{
			// Case insensitive match
			input:   "table=10,actions=note:00.aA.Aa.bB.Bb.00.00",
			prefix:  "00.aa.aa.bb.bb.00",
			success: true,
		},
		{
			// missing note
			input:   "table=10,actions=goto_table:50",
			prefix:  "00.aa.aa.bb.bb.00",
			success: false,
		},
	}

	for i, test := range noteTests {
		flow, err := ParseFlow(ParseForDump, test.input)
		if err != nil {
			if test.errStr != "" && !strings.Contains(err.Error(), test.errStr) {
				t.Fatalf("unexpected error from ParseFlow: %v", err)
			}
		} else if test.errStr != "" {
			t.Fatalf("expected error %q from ParseFlow", test.errStr)
		}
		if success := flow.NoteHasPrefix(test.prefix); success != test.success {
			t.Fatalf("note prefix %d success (%v) does not match expected success (%v)", i, success, test.success)
		}
	}
}

func TestExternalIDs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		output   map[string]string
		unparsed []string
	}{
		{
			name:     "one element, no punctuation",
			input:    `foo=bar`,
			output:   map[string]string{"foo": "bar"},
			unparsed: []string{`{foo="bar"}`},
		},
		{
			name:     "one element, with punctuation",
			input:    `{foo="bar"}`,
			output:   map[string]string{"foo": "bar"},
			unparsed: []string{`{foo="bar"}`},
		},
		{
			name:     "two elements, no punctuation",
			input:    `foo=bar,two=blah`,
			output:   map[string]string{"foo": "bar", "two": "blah"},
			unparsed: []string{`{foo="bar",two="blah"}`, `{two="blah",foo="bar"}`},
		},
		{
			name:     "two elements, with punctuation",
			input:    `{foo="bar", two="blah"}`,
			output:   map[string]string{"foo": "bar", "two": "blah"},
			unparsed: []string{`{foo="bar",two="blah"}`, `{two="blah",foo="bar"}`},
		},
		{
			name:   "three elements, one quoted",
			input:  `foo=bar,two="blah",three=baz`,
			output: map[string]string{"foo": "bar", "two": "blah", "three": "baz"},
			unparsed: []string{
				`{foo="bar",two="blah",three="baz"}`,
				`{foo="bar",three="baz",two="blah"}`,
				`{two="blah",foo="bar",three="baz"}`,
				`{two="blah",three="baz",foo="bar"}`,
				`{three="baz",foo="bar",two="blah"}`,
				`{three="baz",two="blah",foo="bar"}`,
			},
		},
		{
			name:   "missing element",
			input:  `foo=bar,,three=baz`,
			output: nil,
		},
		{
			name:   "bad element",
			input:  `foo=bar,two`,
			output: nil,
		},
	}

	for _, test := range tests {
		parsed, err := ParseExternalIDs(test.input)
		if test.output == nil {
			if err == nil {
				t.Fatalf("on test %q expected failure but got %#v", test.name, parsed)
			}
			continue
		}
		if !reflect.DeepEqual(parsed, test.output) {
			t.Fatalf("on test %q expected %#v but got %#v", test.name, test.output, parsed)
		}
		unparsed := UnparseExternalIDs(parsed)
		found := false
		for _, check := range test.unparsed {
			if unparsed == check {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("on test %q unparsed %q was not any of %#v", test.name, unparsed, test.unparsed)
		}
	}
}
