//Copyright 2013 Vastech SA (PTY) LTD
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.

package gographviz

import (
	"sort"
)

//Represents an Edge.
type Edge struct {
	Src     string
	SrcPort string
	Dst     string
	DstPort string
	Dir     bool
	Attrs   Attrs
}

//Represents a set of Edges.
type Edges struct {
	SrcToDsts map[string]map[string]*Edge
	DstToSrcs map[string]map[string]*Edge
	Edges     []*Edge
}

//Creates a blank set of Edges.
func NewEdges() *Edges {
	return &Edges{make(map[string]map[string]*Edge), make(map[string]map[string]*Edge), make([]*Edge, 0)}
}

//Adds an Edge to the set of Edges.
func (this *Edges) Add(edge *Edge) {
	if _, ok := this.SrcToDsts[edge.Src]; !ok {
		this.SrcToDsts[edge.Src] = make(map[string]*Edge)
	}
	if _, ok := this.SrcToDsts[edge.Src][edge.Dst]; !ok {
		this.SrcToDsts[edge.Src][edge.Dst] = edge
	} else {
		this.SrcToDsts[edge.Src][edge.Dst].Attrs.Extend(edge.Attrs)
	}
	if _, ok := this.DstToSrcs[edge.Dst]; !ok {
		this.DstToSrcs[edge.Dst] = make(map[string]*Edge)
	}
	if _, ok := this.DstToSrcs[edge.Dst][edge.Src]; !ok {
		this.DstToSrcs[edge.Dst][edge.Src] = edge
	}
	this.Edges = append(this.Edges, edge)
}

//Retrusn a sorted list of Edges.
func (this Edges) Sorted() []*Edge {
	srcs := make([]string, 0, len(this.SrcToDsts))
	for src := range this.SrcToDsts {
		srcs = append(srcs, src)
	}
	sort.Strings(srcs)
	edges := make([]*Edge, 0, len(srcs))
	for _, src := range srcs {
		dsts := make([]string, 0, len(this.SrcToDsts[src]))
		for dst := range this.SrcToDsts[src] {
			dsts = append(dsts, dst)
		}
		sort.Strings(dsts)
		for _, dst := range dsts {
			edges = append(edges, this.SrcToDsts[src][dst])
		}
	}
	return edges
}
