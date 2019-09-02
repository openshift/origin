// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gexf12

import (
	"encoding/xml"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

var gexfExampleTests = []struct {
	path        string
	unmarshaled Content
	marshaled   string
}{
	{
		path: "basic.gexf",
		unmarshaled: Content{
			XMLName: xml.Name{Space: "http://www.gexf.net/1.2draft", Local: "gexf"},
			Meta:    nil,
			Graph: Graph{
				Attributes: nil,
				Nodes: Nodes{
					Nodes: []Node{
						{ID: "0", Label: "Hello"},
						{ID: "1", Label: "Word"},
					},
				},
				Edges: Edges{
					Edges: []Edge{
						{ID: "0", Source: "0", Target: "1"},
					},
				},
				DefaultEdgeType: "directed",
				Mode:            "static",
			},
			Version: "1.2",
		},
		marshaled: `<gexf xmlns="http://www.gexf.net/1.2draft" version="1.2">
	<graph defaultedgetype="directed" mode="static">
		<nodes>
			<node id="0" label="Hello"></node>
			<node id="1" label="Word"></node>
		</nodes>
		<edges>
			<edge id="0" source="0" target="1"></edge>
		</edges>
	</graph>
</gexf>`,
	},
	{
		path: "data.gexf",
		unmarshaled: Content{
			XMLName: xml.Name{Space: "http://www.gexf.net/1.2draft", Local: "gexf"},
			Meta: &Meta{
				Creator:      "Gephi.org",
				Description:  "A Web network",
				LastModified: time.Date(2009, 03, 20, 0, 0, 0, 0, time.UTC),
			},
			Graph: Graph{
				Attributes: []Attributes{{
					Class: "node",
					Attributes: []Attribute{
						{
							ID:    "0",
							Title: "url",
							Type:  "string",
						},
						{
							ID:    "1",
							Title: "indegree",
							Type:  "float",
						},
						{
							ID:      "2",
							Title:   "frog",
							Type:    "boolean",
							Default: "true",
						},
					},
				}},
				Nodes: Nodes{
					Nodes: []Node{
						{
							ID: "0", Label: "Gephi",
							AttValues: &AttValues{AttValues: []AttValue{
								{For: "0", Value: "http://gephi.org"},
								{For: "1", Value: "1"},
							}},
						},
						{
							ID: "1", Label: "Webatlas",
							AttValues: &AttValues{AttValues: []AttValue{
								{For: "0", Value: "http://webatlas.fr"},
								{For: "1", Value: "2"},
							}},
						},
						{
							ID: "2", Label: "RTGI",
							AttValues: &AttValues{AttValues: []AttValue{
								{For: "0", Value: "http://rtgi.fr"},
								{For: "1", Value: "1"},
							}},
						},
						{
							ID: "3", Label: "BarabasiLab",
							AttValues: &AttValues{AttValues: []AttValue{
								{For: "0", Value: "http://barabasilab.com"},
								{For: "1", Value: "1"},
								{For: "2", Value: "false"},
							}},
						},
					},
				},
				Edges: Edges{
					Edges: []Edge{
						{ID: "0", Source: "0", Target: "1"},
						{ID: "1", Source: "0", Target: "2"},
						{ID: "2", Source: "1", Target: "0"},
						{ID: "3", Source: "2", Target: "1"},
						{ID: "4", Source: "0", Target: "3"},
					},
				},
				DefaultEdgeType: "directed",
			},
			Version: "1.2",
		},
		marshaled: `<gexf xmlns="http://www.gexf.net/1.2draft" version="1.2">
	<meta lastmodifieddate="2009-03-20">
		<creator>Gephi.org</creator>
		<description>A Web network</description>
	</meta>
	<graph defaultedgetype="directed">
		<attributes class="node">
			<attribute id="0" title="url" type="string"></attribute>
			<attribute id="1" title="indegree" type="float"></attribute>
			<attribute id="2" title="frog" type="boolean">
				<default>true</default>
			</attribute>
		</attributes>
		<nodes>
			<node id="0" label="Gephi">
				<attvalues>
					<attvalue for="0" value="http://gephi.org"></attvalue>
					<attvalue for="1" value="1"></attvalue>
				</attvalues>
			</node>
			<node id="1" label="Webatlas">
				<attvalues>
					<attvalue for="0" value="http://webatlas.fr"></attvalue>
					<attvalue for="1" value="2"></attvalue>
				</attvalues>
			</node>
			<node id="2" label="RTGI">
				<attvalues>
					<attvalue for="0" value="http://rtgi.fr"></attvalue>
					<attvalue for="1" value="1"></attvalue>
				</attvalues>
			</node>
			<node id="3" label="BarabasiLab">
				<attvalues>
					<attvalue for="0" value="http://barabasilab.com"></attvalue>
					<attvalue for="1" value="1"></attvalue>
					<attvalue for="2" value="false"></attvalue>
				</attvalues>
			</node>
		</nodes>
		<edges>
			<edge id="0" source="0" target="1"></edge>
			<edge id="1" source="0" target="2"></edge>
			<edge id="2" source="1" target="0"></edge>
			<edge id="3" source="2" target="1"></edge>
			<edge id="4" source="0" target="3"></edge>
		</edges>
	</graph>
</gexf>`,
	},
	{
		path: "hierarchy1.gexf",
		unmarshaled: Content{
			XMLName: xml.Name{Space: "http://www.gexf.net/1.2draft", Local: "gexf"},
			Meta: &Meta{
				Creator:      "Gephi.org",
				Description:  "A hierarchy file",
				LastModified: time.Date(2009, 10, 1, 0, 0, 0, 0, time.UTC),
			},
			Graph: Graph{
				Nodes: Nodes{
					Nodes: []Node{{
						ID:    "a",
						Label: "Kevin Bacon",
						Nodes: &Nodes{
							Nodes: []Node{
								{
									ID: "b", Label: "God",
									Nodes: &Nodes{
										Nodes: []Node{
											{ID: "c", Label: "human1"},
											{ID: "d", Label: "human2"},
											{ID: "i", Label: "human3"},
										},
									},
								},
								{
									ID: "e", Label: "Me",
									Nodes: &Nodes{
										Nodes: []Node{
											{ID: "f", Label: "frog1"},
											{ID: "g", Label: "frog2"},
											{ID: "h", Label: "frog3"},
										},
									},
								},
								{
									ID: "j", Label: "You",
								},
							},
						},
					}},
				},
				Edges: Edges{
					Edges: []Edge{
						{ID: "0", Source: "b", Target: "e"},
						{ID: "1", Source: "c", Target: "d"},
						{ID: "2", Source: "c", Target: "i"},
						{ID: "3", Source: "g", Target: "b"},
						{ID: "4", Source: "f", Target: "a"},
						{ID: "5", Source: "f", Target: "g"},
						{ID: "6", Source: "f", Target: "h"},
						{ID: "7", Source: "g", Target: "h"},
						{ID: "8", Source: "a", Target: "j"},
					},
				},
				DefaultEdgeType: "directed",
				Mode:            "static",
			},
			Version: "1.2",
		},
		marshaled: `<gexf xmlns="http://www.gexf.net/1.2draft" version="1.2">
	<meta lastmodifieddate="2009-10-01">
		<creator>Gephi.org</creator>
		<description>A hierarchy file</description>
	</meta>
	<graph defaultedgetype="directed" mode="static">
		<nodes>
			<node id="a" label="Kevin Bacon">
				<nodes>
					<node id="b" label="God">
						<nodes>
							<node id="c" label="human1"></node>
							<node id="d" label="human2"></node>
							<node id="i" label="human3"></node>
						</nodes>
					</node>
					<node id="e" label="Me">
						<nodes>
							<node id="f" label="frog1"></node>
							<node id="g" label="frog2"></node>
							<node id="h" label="frog3"></node>
						</nodes>
					</node>
					<node id="j" label="You"></node>
				</nodes>
			</node>
		</nodes>
		<edges>
			<edge id="0" source="b" target="e"></edge>
			<edge id="1" source="c" target="d"></edge>
			<edge id="2" source="c" target="i"></edge>
			<edge id="3" source="g" target="b"></edge>
			<edge id="4" source="f" target="a"></edge>
			<edge id="5" source="f" target="g"></edge>
			<edge id="6" source="f" target="h"></edge>
			<edge id="7" source="g" target="h"></edge>
			<edge id="8" source="a" target="j"></edge>
		</edges>
	</graph>
</gexf>`,
	},
	{
		path: "hierarchy4.gexf",
		unmarshaled: Content{
			XMLName: xml.Name{Space: "http://www.gexf.net/1.2draft", Local: "gexf"},
			Meta: &Meta{
				Creator:      "Gephi.org",
				Keywords:     "",
				Description:  "A hierarchy file",
				LastModified: time.Date(2009, 10, 1, 0, 0, 0, 0, time.UTC),
			},
			Graph: Graph{
				Nodes: Nodes{
					Nodes: []Node{
						{ID: "g", Label: "frog2", ParentID: "e"},
						{ID: "a", Label: "Kevin Bacon"},
						{ID: "c", Label: "human1", ParentID: "b"},
						{ID: "b", Label: "God", ParentID: "a"},
						{ID: "e", Label: "Me", ParentID: "a"},
						{ID: "d", Label: "human2", ParentID: "b"},
						{ID: "f", Label: "frog1", ParentID: "e"},
					},
				},
				Edges: Edges{
					Edges: []Edge{
						{ID: "0", Source: "b", Target: "e"},
						{ID: "1", Source: "c", Target: "d"},
						{ID: "2", Source: "g", Target: "b"},
						{ID: "3", Source: "f", Target: "a"},
					},
				},
				DefaultEdgeType: "directed",
				Mode:            "static",
			},
			Version: "1.2",
		},
		marshaled: `<gexf xmlns="http://www.gexf.net/1.2draft" version="1.2">
	<meta lastmodifieddate="2009-10-01">
		<creator>Gephi.org</creator>
		<description>A hierarchy file</description>
	</meta>
	<graph defaultedgetype="directed" mode="static">
		<nodes>
			<node id="g" label="frog2" pid="e"></node>
			<node id="a" label="Kevin Bacon"></node>
			<node id="c" label="human1" pid="b"></node>
			<node id="b" label="God" pid="a"></node>
			<node id="e" label="Me" pid="a"></node>
			<node id="d" label="human2" pid="b"></node>
			<node id="f" label="frog1" pid="e"></node>
		</nodes>
		<edges>
			<edge id="0" source="b" target="e"></edge>
			<edge id="1" source="c" target="d"></edge>
			<edge id="2" source="g" target="b"></edge>
			<edge id="3" source="f" target="a"></edge>
		</edges>
	</graph>
</gexf>`,
	},
	{
		path: "phylogeny.gexf",
		unmarshaled: Content{
			XMLName: xml.Name{Space: "http://www.gexf.net/1.2draft", Local: "gexf"},
			Graph: Graph{
				Nodes: Nodes{
					Nodes: []Node{
						{ID: "a", Label: "cheese"},
						{ID: "b", Label: "cherry"},
						{ID: "c", Label: "cake", Parents: &Parents{
							Parents: []Parent{
								{For: "a"},
								{For: "b"},
							},
						},
						},
					},
				},
				Edges: Edges{
					Edges: nil,
					Count: 0,
				},
			},
			Version: "1.2",
		},
		marshaled: `<gexf xmlns="http://www.gexf.net/1.2draft" version="1.2">
	<graph>
		<nodes>
			<node id="a" label="cheese"></node>
			<node id="b" label="cherry"></node>
			<node id="c" label="cake">
				<parents>
					<parent for="a"></parent>
					<parent for="b"></parent>
				</parents>
			</node>
		</nodes>
		<edges></edges>
	</graph>
</gexf>`,
	},
	{
		path: "viz.gexf",
		unmarshaled: Content{
			XMLName: xml.Name{Space: "http://www.gexf.net/1.2draft", Local: "gexf"},
			Graph: Graph{
				Nodes: Nodes{
					Nodes: []Node{
						{
							ID:    "a",
							Label: "glossy",
							Color: &Color{
								R: 239,
								G: 173,
								B: 66,
								A: 0.6,
							},
							Position: &Position{
								X: 15.783598,
								Y: 40.109245,
								Z: 0,
							},
							Size: &Size{
								Value: 2.0375757,
							},
						},
					},
				},
				Edges: Edges{},
			},
			Version: "1.2",
		},
		marshaled: `<gexf xmlns="http://www.gexf.net/1.2draft" version="1.2">
	<graph>
		<nodes>
			<node id="a" label="glossy">
				<color xmlns="http://www.gexf.net/1.2draft/viz" r="239" g="173" b="66" a="0.6"></color>
				<position xmlns="http://www.gexf.net/1.2draft/viz" x="15.783598" y="40.109245" z="0"></position>
				<size xmlns="http://www.gexf.net/1.2draft/viz" value="2.0375757"></size>
			</node>
		</nodes>
		<edges></edges>
	</graph>
</gexf>`,
	},
}

func TestUnmarshal(t *testing.T) {
	for _, test := range gexfExampleTests {
		data, err := ioutil.ReadFile(filepath.Join("testdata", test.path))
		if err != nil {
			t.Errorf("failed to read %q: %v", test.path, err)
			continue
		}
		var got Content
		err = xml.Unmarshal(data, &got)
		if err != nil {
			t.Errorf("failed to unmarshal %q: %v", test.path, err)
		}
		if !reflect.DeepEqual(got, test.unmarshaled) {
			t.Errorf("unexpected result for %q:\ngot:\n%#v\nwant:\n%#v", test.path, got, test.unmarshaled)
		}
	}
}

// TODO(kortschak): Update this test when/if namespace
// prefix handling in encoding/xml is fixed.
func TestMarshal(t *testing.T) {
	for _, test := range gexfExampleTests {
		got, err := xml.MarshalIndent(test.unmarshaled, "", "\t")
		if err != nil {
			t.Errorf("failed to marshal %q: %v", test.path, err)
			continue
		}
		if string(got) != test.marshaled {
			t.Errorf("unexpected result for %q:\ngot:\n%s\nwant:\n%s", test.path, got, test.marshaled)
		}
	}
}
