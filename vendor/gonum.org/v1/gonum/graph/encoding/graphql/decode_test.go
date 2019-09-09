// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package graphql

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"testing"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/encoding"
	"gonum.org/v1/gonum/graph/encoding/dot"
	"gonum.org/v1/gonum/graph/simple"
)

var decodeTests = []struct {
	name    string
	json    string
	roots   map[uint64]bool
	wantDOT string
	wantErr error
}{
	{
		name: "starwars",
		json: starwars,
		roots: map[uint64]bool{
			0xa3cff1a4c3ef3bb6: true,
			0xb39aa14d66aedad5: true,
		},
		wantDOT: `strict digraph {
  // Node definitions.
  "0x8a10d5a2611fd03f" [name="Richard Marquand"];
  "0xa3cff1a4c3ef3bb6" [
    name="Star Wars: Episode V - The Empire Strikes Back"
    release_date="1980-05-21T00:00:00Z"
    revenue=534000000
    running_time=124
  ];
  "0xb39aa14d66aedad5" [
    name="Star Wars: Episode VI - Return of the Jedi"
    release_date="1983-05-25T00:00:00Z"
    revenue=572000000
    running_time=131
  ];
  "0x0312de17a7ee89f9" [name="Luke Skywalker"];
  "0x3da8d1dcab1bb381" [name="Han Solo"];
  "0x4a7d0b5fe91e78a4" [name="Irvin Kernshner"];
  "0x718337b9dcbaa7d9" [name="Princess Leia"];

  // Edge definitions.
  "0xa3cff1a4c3ef3bb6" -> "0x0312de17a7ee89f9" [label=starring];
  "0xa3cff1a4c3ef3bb6" -> "0x3da8d1dcab1bb381" [label=starring];
  "0xa3cff1a4c3ef3bb6" -> "0x4a7d0b5fe91e78a4" [label=director];
  "0xa3cff1a4c3ef3bb6" -> "0x718337b9dcbaa7d9" [label=starring];
  "0xb39aa14d66aedad5" -> "0x8a10d5a2611fd03f" [label=director];
  "0xb39aa14d66aedad5" -> "0x0312de17a7ee89f9" [label=starring];
  "0xb39aa14d66aedad5" -> "0x3da8d1dcab1bb381" [label=starring];
  "0xb39aa14d66aedad5" -> "0x718337b9dcbaa7d9" [label=starring];
}`,
	},
	{
		name: "tutorial",
		json: dgraphTutorial,
		roots: map[uint64]bool{
			0xfd90205a458151f:  true,
			0x52a80955d40ec819: true,
		},
		wantDOT: `strict digraph {
  // Node definitions.
  "0x892a6da7ee1fbdec" [
    age=55
    name=Sarah
  ];
  "0x99b74c1b5ab100ec" [
    age=35
    name=Artyom
  ];
  "0xb9e12a67e34d6acc" [
    age=19
    name=Catalina
  ];
  "0xbf104824c777525d" [name=Perro];
  "0xf590a923ea1fccaa" [name=Goldie];
  "0xf92d7dbe272d680b" [name="Hyung Sin"];
  "0x0fd90205a458151f" [
    age=39
    name=Michael
  ];
  "0x37734fcf0a6fcc69" [name="Rammy the sheep"];
  "0x52a80955d40ec819" [
    age=35
    name=Amit
  ];
  "0x5e9ad1cd9466228c" [
    age=24
    name="Sang Hyun"
  ];

  // Edge definitions.
  "0xb9e12a67e34d6acc" -> "0xbf104824c777525d" [label=owns_pet];
  "0xb9e12a67e34d6acc" -> "0x5e9ad1cd9466228c" [label=friend];
  "0xf92d7dbe272d680b" -> "0x5e9ad1cd9466228c" [label=friend];
  "0x0fd90205a458151f" -> "0x892a6da7ee1fbdec" [label=friend];
  "0x0fd90205a458151f" -> "0x99b74c1b5ab100ec" [label=friend];
  "0x0fd90205a458151f" -> "0xb9e12a67e34d6acc" [label=friend];
  "0x0fd90205a458151f" -> "0x37734fcf0a6fcc69" [label=owns_pet];
  "0x0fd90205a458151f" -> "0x52a80955d40ec819" [label=friend];
  "0x0fd90205a458151f" -> "0x5e9ad1cd9466228c" [label=friend];
  "0x52a80955d40ec819" -> "0x99b74c1b5ab100ec" [label=friend];
  "0x52a80955d40ec819" -> "0x0fd90205a458151f" [label=friend];
  "0x52a80955d40ec819" -> "0x5e9ad1cd9466228c" [label=friend];
  "0x5e9ad1cd9466228c" -> "0xb9e12a67e34d6acc" [label=friend];
  "0x5e9ad1cd9466228c" -> "0xf590a923ea1fccaa" [label=owns_pet];
  "0x5e9ad1cd9466228c" -> "0xf92d7dbe272d680b" [label=friend];
  "0x5e9ad1cd9466228c" -> "0x52a80955d40ec819" [label=friend];
}`,
	},
	{
		name:    "tutorial missing IDs",
		json:    dgraphTutorialMissingIDs,
		wantErr: errors.New("graphql: no UID for node"), // Incomplete error string.
	},
}

func TestDecode(t *testing.T) {
	for _, test := range decodeTests {
		dst := newDirectedGraph()
		err := Unmarshal([]byte(test.json), "_uid_", dst)
		if test.wantErr == nil && err != nil {
			t.Errorf("failed to unmarshal GraphQL JSON graph for %q: %v", test.name, err)
		} else if test.wantErr != nil {
			if err == nil {
				t.Errorf("expected error for %q: got:%v want:%v", test.name, err, test.wantErr)
			}
			continue
		}
		b, err := dot.Marshal(dst, "", "", "  ")
		if err != nil {
			t.Fatalf("failed to DOT marshal graph %q: %v", test.name, err)
		}
		gotDOT := string(b)
		if gotDOT != test.wantDOT {
			t.Errorf("unexpected DOT encoding for %q:\ngot:\n%s\nwant:\n%s", test.name, gotDOT, test.wantDOT)
		}
		checkDOT(t, b)
	}
}

type directedGraph struct {
	*simple.DirectedGraph
}

func newDirectedGraph() *directedGraph {
	return &directedGraph{DirectedGraph: simple.NewDirectedGraph()}
}

func (g *directedGraph) NewNode() graph.Node {
	return &node{attributes: make(attributes)}
}

func (g *directedGraph) NewEdge(from, to graph.Node) graph.Edge {
	return &edge{Edge: g.DirectedGraph.NewEdge(from, to)}
}

type node struct {
	id uint64
	attributes
}

func (n *node) ID() int64     { return int64(n.id) }
func (n *node) DOTID() string { return fmt.Sprintf("0x%016x", uint64(n.id)) }

func (n *node) SetIDFromString(uid string) error {
	if !strings.HasPrefix(uid, "0x") {
		return fmt.Errorf("uid is not hex value: %q", uid)
	}
	var err error
	n.id, err = strconv.ParseUint(uid[2:], 16, 64)
	return err
}

type edge struct {
	graph.Edge
	label string
}

func (e *edge) SetLabel(l string) {
	e.label = l
}

func (e *edge) Attributes() []encoding.Attribute {
	return []encoding.Attribute{{Key: "label", Value: e.label}}
}

type attributes map[string]encoding.Attribute

func (a attributes) SetAttribute(attr encoding.Attribute) error {
	a[attr.Key] = attr
	return nil
}

func (a attributes) Attributes() []encoding.Attribute {
	keys := make([]string, 0, len(a))
	for k := range a {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	attr := make([]encoding.Attribute, 0, len(keys))
	for _, k := range keys {
		v := a[k]
		if strings.Contains(v.Value, " ") {
			v.Value = `"` + v.Value + `"`
		}
		attr = append(attr, v)
	}
	return attr
}

// checkDOT hands b to the dot executable if it exists and fails t if dot
// returns an error.
func checkDOT(t *testing.T, b []byte) {
	dot, err := exec.LookPath("dot")
	if err != nil {
		t.Logf("skipping DOT syntax check: %v", err)
		return
	}
	cmd := exec.Command(dot)
	cmd.Stdin = bytes.NewReader(b)
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	err = cmd.Run()
	if err != nil {
		t.Errorf("invalid DOT syntax: %v\n%s\ninput:\n%s", err, stderr.String(), b)
	}
}
