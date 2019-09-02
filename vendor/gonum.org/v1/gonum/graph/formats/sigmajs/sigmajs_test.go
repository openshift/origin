// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sigmajs

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"testing"
)

var sigmajsExampleTests = []struct {
	path           string
	wantNodes      int
	wantEdges      int
	wantGraph      *Graph
	wantAttributes map[string]bool
}{
	{
		path:      "geolocalized.json",
		wantNodes: 17,
		wantEdges: 35,
		wantGraph: &Graph{
			Nodes: []Node{
				{
					ID: "n1",
					Attributes: map[string]interface{}{
						"label":     "n1",
						"longitude": 2.48,
						"latitude":  50.93,
						"size":      "5.5",
						"color":     "rgb(1,179,255)",
					},
				},
				{
					ID: "n2",
					Attributes: map[string]interface{}{
						"label":     "n2",
						"latitude":  50.88,
						"longitude": 2.0,
						"size":      "5.0",
						"color":     "rgb(1,179,255)",
					},
				},
				{
					ID: "n4",
					Attributes: map[string]interface{}{
						"label":     "n4",
						"latitude":  49.4,
						"longitude": 0.19,
						"size":      "6.0",
						"color":     "rgb(1,179,255)",
					},
				},
				{
					ID: "n5",
					Attributes: map[string]interface{}{
						"label":     "n5",
						"latitude":  48.49,
						"longitude": -1.92,
						"size":      "6.0",
						"color":     "rgb(1,179,255)",
					},
				},
				{
					ID: "n6",
					Attributes: map[string]interface{}{
						"label":     "n6",
						"latitude":  48.26,
						"longitude": -4.38,
						"size":      "4.5",
						"color":     "rgb(1,179,255)",
					},
				},
				{
					ID: "n7",
					Attributes: map[string]interface{}{
						"label":     "n7",
						"latitude":  47.15,
						"longitude": -2.09,
						"size":      "6.5",
						"color":     "rgb(1,179,255)",
					},
				},
				{
					ID: "n8",
					Attributes: map[string]interface{}{
						"label":     "n8",
						"latitude":  46.02,
						"longitude": -1.04,
						"size":      "6.5",
						"color":     "rgb(1,179,255)",
					},
				},
				{
					ID: "n9",
					Attributes: map[string]interface{}{
						"label":     "n9",
						"latitude":  43.22,
						"longitude": -1.85,
						"size":      "5.0",
						"color":     "rgb(1,179,255)",
					},
				},
				{
					ID: "n10",
					Attributes: map[string]interface{}{
						"label":     "n10",
						"latitude":  42.38,
						"longitude": 3.18,
						"color":     "rgb(1,179,255)",
						"size":      "4.0",
					},
				},
				{
					ID: "n11",
					Attributes: map[string]interface{}{
						"label":     "n11",
						"latitude":  43.47,
						"longitude": 4.04,
						"size":      "5.5",
						"color":     "rgb(1,179,255)",
					},
				},
				{
					ID: "n12",
					Attributes: map[string]interface{}{
						"label":     "n12",
						"latitude":  42.9,
						"longitude": 6.59,
						"size":      "5.0",
						"color":     "rgb(1,179,255)",
					},
				},
				{
					ID: "n13",
					Attributes: map[string]interface{}{
						"label":     "n13",
						"latitude":  43.62,
						"longitude": 7.66,
						"size":      "6.0",
						"color":     "rgb(1,179,255)",
					},
				},
				{
					ID: "n14",
					Attributes: map[string]interface{}{
						"label":     "n14",
						"latitude":  46.05,
						"longitude": 6.19,
						"size":      "6.5",
						"color":     "rgb(1,179,255)",
					},
				},
				{
					ID: "n15",
					Attributes: map[string]interface{}{
						"label":     "n15",
						"latitude":  47.43,
						"longitude": 7.65,
						"size":      "6.0",
						"color":     "rgb(1,179,255)",
					},
				},
				{
					ID: "n16",
					Attributes: map[string]interface{}{
						"label":     "n16",
						"latitude":  48.9,
						"longitude": 8.32,
						"size":      "5.5",
						"color":     "rgb(1,179,255)",
					},
				},
				{
					ID: "n17",
					Attributes: map[string]interface{}{
						"label":     "n17",
						"latitude":  49.83,
						"longitude": 4.94,
						"size":      "6.5",
						"color":     "rgb(1,179,255)",
					},
				},
				{
					ID: "Paris",
					Attributes: map[string]interface{}{
						"label":     "Paris",
						"latitude":  48.72,
						"longitude": 2.46,
						"size":      "9.0",
						"color":     "rgb(1,179,255)",
					},
				},
			},
			Edges: []Edge{
				{ID: "8", Source: "n1", Target: "Paris"},
				{ID: "7", Source: "n2", Target: "n4"},
				{ID: "28", Source: "n4", Target: "n1"},
				{ID: "30", Source: "n4", Target: "n7"},
				{ID: "26", Source: "n5", Target: "n1"},
				{ID: "27", Source: "n5", Target: "n2"},
				{ID: "0", Source: "n6", Target: "n5"},
				{ID: "29", Source: "n7", Target: "n5"},
				{ID: "1", Source: "n7", Target: "n8"},
				{ID: "17", Source: "n7", Target: "Paris"},
				{ID: "10", Source: "n8", Target: "n13"},
				{ID: "18", Source: "n8", Target: "Paris"},
				{ID: "15", Source: "n9", Target: "n8"},
				{ID: "34", Source: "n10", Target: "n9"},
				{ID: "31", Source: "n10", Target: "n11"},
				{ID: "11", Source: "n11", Target: "n13"},
				{ID: "13", Source: "n11", Target: "n14"},
				{ID: "32", Source: "n12", Target: "n10"},
				{ID: "12", Source: "n12", Target: "n11"},
				{ID: "23", Source: "n12", Target: "n13"},
				{ID: "33", Source: "n13", Target: "n10"},
				{ID: "25", Source: "n13", Target: "n14"},
				{ID: "14", Source: "n14", Target: "n9"},
				{ID: "5", Source: "n14", Target: "n17"},
				{ID: "19", Source: "n14", Target: "Paris"},
				{ID: "6", Source: "n15", Target: "n8"},
				{ID: "22", Source: "n15", Target: "n16"},
				{ID: "20", Source: "n15", Target: "Paris"},
				{ID: "4", Source: "n16", Target: "n15"},
				{ID: "24", Source: "n16", Target: "Paris"},
				{ID: "9", Source: "n17", Target: "n7"},
				{ID: "21", Source: "n17", Target: "n17"},
				{ID: "2", Source: "Paris", Target: "n4"},
				{ID: "3", Source: "Paris", Target: "n17"},
				{ID: "16", Source: "Paris", Target: "Paris"},
			},
		},
	},
	{
		path:      "arctic.json",
		wantNodes: 1715,
		wantEdges: 6676,
		wantAttributes: map[string]bool{
			"label":              true,
			"x":                  true,
			"y":                  true,
			"color":              true,
			"size":               true,
			"attributes":         true,
			"attributes.nodedef": true,
		},
	},
}

func TestUnmarshal(t *testing.T) {
	for _, test := range sigmajsExampleTests {
		data, err := ioutil.ReadFile(filepath.Join("testdata", test.path))
		if err != nil {
			t.Errorf("failed to read %q: %v", test.path, err)
			continue
		}
		var got Graph
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
		if test.wantGraph != nil && !reflect.DeepEqual(&got, test.wantGraph) {
			t.Errorf("unexpected result for %q:\ngot:\n%#v\nwant:\n%#v", test.path, got, test.wantGraph)
		}
		if test.wantAttributes != nil {
			var paths []string
			for _, n := range got.Nodes {
				paths = attrPaths(paths, "", n.Attributes)
			}
			gotAttrs := make(map[string]bool)
			for _, p := range paths {
				gotAttrs[p] = true
			}
			if !reflect.DeepEqual(gotAttrs, test.wantAttributes) {
				t.Errorf("unexpected result for %q:\ngot:\n%#v\nwant:\n%#v", test.path, gotAttrs, test.wantAttributes)
			}
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

func TestMarshal(t *testing.T) {
	for _, test := range sigmajsExampleTests {
		data, err := ioutil.ReadFile(filepath.Join("testdata", test.path))
		if err != nil {
			t.Errorf("failed to read %q: %v", test.path, err)
			continue
		}
		var want Graph
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
		var got Graph
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
