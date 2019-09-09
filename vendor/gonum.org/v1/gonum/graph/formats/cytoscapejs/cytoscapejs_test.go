// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cytoscapejs

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"testing"
)

var cytoscapejsElementsTests = []struct {
	path           string
	wantNodes      int
	wantEdges      int
	wantGraph      []Element
	wantAttributes []string
}{
	{
		path:      "edge-type.json",
		wantNodes: 10,
		wantEdges: 10,
		wantGraph: []Element{
			{Data: ElemData{ID: "n01", Attributes: map[string]interface{}{"type": "bezier"}}},
			{Data: ElemData{ID: "n02"}},
			{Data: ElemData{ID: "e01", Source: "n01", Target: "n02"}, Classes: "bezier"},
			{Data: ElemData{ID: "e02", Source: "n01", Target: "n02"}, Classes: "bezier"},
			{Data: ElemData{ID: "e03", Source: "n02", Target: "n01"}, Classes: "bezier"},
			{Data: ElemData{ID: "n03", Attributes: map[string]interface{}{"type": "unbundled-bezier"}}},
			{Data: ElemData{ID: "n04"}},
			{Data: ElemData{ID: "e04", Source: "n03", Target: "n04"}, Classes: "unbundled-bezier"},
			{Data: ElemData{ID: "n05", Attributes: map[string]interface{}{"type": "unbundled-bezier(multiple)"}}},
			{Data: ElemData{ID: "n06"}},
			{Data: ElemData{ID: "e05", Source: "n05", Target: "n06", Parent: ""}, Classes: "multi-unbundled-bezier"},
			{Data: ElemData{ID: "n07", Attributes: map[string]interface{}{"type": "haystack"}}},
			{Data: ElemData{ID: "n08"}},
			{Data: ElemData{ID: "e06", Source: "n08", Target: "n07"}, Classes: "haystack"},
			{Data: ElemData{ID: "e07", Source: "n08", Target: "n07"}, Classes: "haystack"},
			{Data: ElemData{ID: "e08", Source: "n08", Target: "n07"}, Classes: "haystack"},
			{Data: ElemData{ID: "e09", Source: "n08", Target: "n07"}, Classes: "haystack"},
			{Data: ElemData{ID: "n09", Attributes: map[string]interface{}{"type": "segments"}}},
			{Data: ElemData{ID: "n10"}},
			{Data: ElemData{ID: "e10", Source: "n09", Target: "n10"}, Classes: "segments"},
		},
	},
}

func TestUnmarshalElements(t *testing.T) {
	for _, test := range cytoscapejsElementsTests {
		data, err := ioutil.ReadFile(filepath.Join("testdata", test.path))
		if err != nil {
			t.Errorf("failed to read %q: %v", test.path, err)
			continue
		}
		var got []Element
		err = json.Unmarshal(data, &got)
		if err != nil {
			t.Errorf("failed to unmarshal %q: %v", test.path, err)
			continue
		}
		var gotNodes, gotEdges int
		for _, e := range got {
			typ, err := e.Type()
			if err != nil {
				t.Errorf("unexpected error finding element type for %+v: %v", e, err)
			}
			switch typ {
			case NodeElement:
				gotNodes++
			case EdgeElement:
				gotEdges++
			}
		}

		if gotNodes != test.wantNodes {
			t.Errorf("unexpected result for order of %q: got:%d want:%d", test.path, gotNodes, test.wantNodes)
		}
		if gotEdges != test.wantEdges {
			t.Errorf("unexpected result for size of %q: got:%d want:%d", test.path, gotEdges, test.wantEdges)
		}
		if test.wantGraph != nil && !reflect.DeepEqual(got, test.wantGraph) {
			t.Errorf("unexpected result for %q:\ngot:\n%#v\nwant:\n%#v", test.path, got, test.wantGraph)
		}
	}
}

func TestMarshalElements(t *testing.T) {
	for _, test := range cytoscapejsElementsTests {
		data, err := ioutil.ReadFile(filepath.Join("testdata", test.path))
		if err != nil {
			t.Errorf("failed to read %q: %v", test.path, err)
			continue
		}
		var want []Element
		err = json.Unmarshal(data, &want)
		if err != nil {
			t.Errorf("failed to unmarshal %q: %v", test.path, err)
			continue
		}
		marshaled, err := json.Marshal(want)
		if err != nil {
			t.Errorf("failed to unmarshal %q: %v", test.path, err)
			continue
		}
		var got []Element
		err = json.Unmarshal(marshaled, &got)
		if err != nil {
			t.Errorf("failed to unmarshal %q: %v", test.path, err)
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("unexpected result for %q:\ngot:\n%#v\nwant:\n%#v", test.path, got, want)
		}
	}
}

var cytoscapejsNodeEdgeTests = []struct {
	path               string
	wantNodes          int
	wantEdges          int
	wantGraph          *Elements
	firstNode          Node
	firstEdge          Edge
	wantNodeAttributes map[string]bool
	wantEdgeAttributes map[string]bool
}{
	{
		path:      "cola-compound.json",
		wantNodes: 9,
		wantEdges: 7,
		wantGraph: &Elements{
			Nodes: []Node{
				{Data: NodeData{ID: "compound-1", Parent: ""}},
				{Data: NodeData{ID: "compound-2", Parent: ""}},
				{Data: NodeData{ID: "compound-3", Parent: ""}},
				{Data: NodeData{ID: "b", Parent: "compound-1"}},
				{Data: NodeData{ID: "c", Parent: "compound-1"}},
				{Data: NodeData{ID: "a", Parent: "compound-2"}},
				{Data: NodeData{ID: "d", Parent: "compound-3"}},
				{Data: NodeData{ID: "e", Parent: "compound-3"}},
				{Data: NodeData{ID: "f", Parent: ""}},
			},
			Edges: []Edge{
				{Data: EdgeData{ID: "ab", Source: "a", Target: "b"}},
				{Data: EdgeData{ID: "bc", Source: "b", Target: "c"}},
				{Data: EdgeData{ID: "ac", Source: "a", Target: "c"}},
				{Data: EdgeData{ID: "cd", Source: "c", Target: "d"}},
				{Data: EdgeData{ID: "de", Source: "d", Target: "e"}},
				{Data: EdgeData{ID: "df", Source: "d", Target: "f"}},
				{Data: EdgeData{ID: "af", Source: "a", Target: "f"}},
			},
		},
	},
	{
		path:      "tokyo-railways.json",
		wantNodes: 943,
		wantEdges: 860,
		firstNode: Node{
			Data: NodeData{
				ID: "8220",
				Attributes: map[string]interface{}{
					"station_name": "京成高砂",
					"close_ymd":    "",
					"lon":          139.866875,
					"post":         "",
					"e_status":     0.0,
					"SUID":         8220.0,
					"station_g_cd": 2300110.0,
					"add":          "東京都葛飾区高砂五丁目28-1",
					"line_cd":      99340.0,
					"selected":     false,
					"open_ymd":     "",
					"name":         "9934001",
					"pref_name":    "東京都",
					"shared_name":  "9934001",
					"lat":          35.750932,
					"x":            1398668.75,
					"y":            -357509.32,
				},
			},
			Position: &Position{
				X: 1398668.75,
				Y: -357509.32,
			},
		},
		firstEdge: Edge{
			Data: EdgeData{
				ID:     "18417",
				Source: "8220",
				Target: "8221",
				Attributes: map[string]interface{}{
					"line_name_k":        "ホクソウテツドウホクソウセン",
					"is_bullet":          false,
					"lon":                140.03784499075186,
					"company_name_k":     "ホクソウテツドウ",
					"zoom":               11.0,
					"SUID":               18417.0,
					"company_type":       0.0,
					"company_name_h":     "北総鉄道株式会社",
					"interaction":        "99340",
					"shared_interaction": "99340",
					"company_url":        "http://www.hokuso-railway.co.jp/",
					"line_name":          "北総鉄道北総線",
					"selected":           false,
					"company_name":       "北総鉄道",
					"company_cd":         152.0,
					"name":               "9934001 (99340) 9934002",
					"rr_cd":              99.0,
					"company_name_r":     "北総鉄道",
					"e_status_x":         0.0,
					"shared_name":        "9934001 (99340) 9934002",
					"lat":                35.78346285846615,
					"e_status_y":         0.0,
					"line_name_h":        "北総鉄道北総線",
				},
			},
		},
		wantNodeAttributes: map[string]bool{
			"station_name": true,
			"close_ymd":    true,
			"lon":          true,
			"post":         true,
			"e_status":     true,
			"SUID":         true,
			"station_g_cd": true,
			"add":          true,
			"line_cd":      true,
			"selected":     true,
			"open_ymd":     true,
			"name":         true,
			"pref_name":    true,
			"shared_name":  true,
			"lat":          true,
			"x":            true,
			"y":            true,
		},
		wantEdgeAttributes: map[string]bool{
			"line_name_k":        true,
			"is_bullet":          true,
			"lon":                true,
			"company_name_k":     true,
			"zoom":               true,
			"SUID":               true,
			"company_type":       true,
			"company_name_h":     true,
			"interaction":        true,
			"shared_interaction": true,
			"company_url":        true,
			"line_name":          true,
			"selected":           true,
			"company_name":       true,
			"company_cd":         true,
			"name":               true,
			"rr_cd":              true,
			"company_name_r":     true,
			"e_status_x":         true,
			"shared_name":        true,
			"lat":                true,
			"e_status_y":         true,
			"line_name_h":        true,
		},
	},
}

func TestUnmarshalNodeEdge(t *testing.T) {
	for _, test := range cytoscapejsNodeEdgeTests {
		data, err := ioutil.ReadFile(filepath.Join("testdata", test.path))
		if err != nil {
			t.Errorf("failed to read %q: %v", test.path, err)
			continue
		}
		var got Elements
		err = json.Unmarshal(data, &got)
		if err != nil {
			t.Errorf("failed to unmarshal %q: %v", test.path, err)
			continue
		}

		if len(got.Nodes) != test.wantNodes {
			t.Errorf("unexpected result for order of %q: got:%d want:%d", test.path, len(got.Nodes), test.wantNodes)
		}
		if len(got.Edges) != test.wantEdges {
			t.Errorf("unexpected result for size of %q: got:%d want:%d", test.path, len(got.Edges), test.wantEdges)
		}
		if test.wantGraph != nil {
			if !reflect.DeepEqual(&got, test.wantGraph) {
				t.Errorf("unexpected result for %q:\ngot:\n%#v\nwant:\n%#v", test.path, got.Nodes, test.wantGraph.Nodes)
			}
		} else {
			if !reflect.DeepEqual(got.Nodes[0], test.firstNode) {
				t.Errorf("unexpected result for %q:\ngot:\n%#v\nwant:\n%#v", test.path, got.Nodes[0], test.firstNode)
			}
			if !reflect.DeepEqual(got.Edges[0], test.firstEdge) {
				t.Errorf("unexpected result for %q:\ngot:\n%v\nwant:\n%#v", test.path, got.Edges[0].Data.Source, test.firstEdge.Data.Source)
			}
		}
		if test.wantNodeAttributes != nil {
			var paths []string
			for _, n := range got.Nodes {
				paths = attrPaths(paths, "", n.Data.Attributes)
			}
			gotAttrs := make(map[string]bool)
			for _, p := range paths {
				gotAttrs[p] = true
			}
			if !reflect.DeepEqual(gotAttrs, test.wantNodeAttributes) {
				t.Errorf("unexpected result for %q:\ngot:\n%#v\nwant:\n%#v", test.path, gotAttrs, test.wantNodeAttributes)
			}
		}
		if test.wantEdgeAttributes != nil {
			var paths []string
			for _, e := range got.Edges {
				paths = attrPaths(paths, "", e.Data.Attributes)
			}
			gotAttrs := make(map[string]bool)
			for _, p := range paths {
				gotAttrs[p] = true
			}
			if !reflect.DeepEqual(gotAttrs, test.wantEdgeAttributes) {
				t.Errorf("unexpected result for %q:\ngot:\n%#v\nwant:\n%#v", test.path, gotAttrs, test.wantEdgeAttributes)
			}
		}
	}
}

func TestMarshalNodeEdge(t *testing.T) {
	for _, test := range cytoscapejsNodeEdgeTests {
		data, err := ioutil.ReadFile(filepath.Join("testdata", test.path))
		if err != nil {
			t.Errorf("failed to read %q: %v", test.path, err)
			continue
		}
		var want Elements
		err = json.Unmarshal(data, &want)
		if err != nil {
			t.Errorf("failed to unmarshal %q: %v", test.path, err)
			continue
		}
		marshaled, err := json.Marshal(want)
		if err != nil {
			t.Errorf("failed to unmarshal %q: %v", test.path, err)
			continue
		}
		var got Elements
		err = json.Unmarshal(marshaled, &got)
		if err != nil {
			t.Errorf("failed to unmarshal %q: %v", test.path, err)
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("unexpected result for %q:\ngot:\n%#v\nwant:\n%#v", test.path, got, want)
		}
	}
}

func attrPaths(dst []string, prefix string, m map[string]interface{}) []string {
	for k, v := range m {
		path := prefix
		if path != "" {
			path += "."
		}
		if v, ok := v.(map[string]interface{}); ok {
			dst = attrPaths(dst, path+k, v)
		}
		dst = append(dst, path+k)
	}
	return dst
}
