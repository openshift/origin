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

//Package gographviz provides parsing for the DOT grammar into
//an abstract syntax tree representing a graph,
//analysis of the abstract syntax tree into a more usable structure,
//and writing back of this structure into the DOT format.
package gographviz

import (
	"github.com/awalterschulze/gographviz/ast"
	"github.com/awalterschulze/gographviz/parser"
)

var _ Interface = NewGraph()

//Implementing this interface allows you to parse the graph into your own structure.
type Interface interface {
	SetStrict(strict bool)
	SetDir(directed bool)
	SetName(name string)
	AddPortEdge(src, srcPort, dst, dstPort string, directed bool, attrs map[string]string)
	AddEdge(src, dst string, directed bool, attrs map[string]string)
	AddNode(parentGraph string, name string, attrs map[string]string)
	AddAttr(parentGraph string, field, value string)
	AddSubGraph(parentGraph string, name string, attrs map[string]string)
	String() string
}

//Parses the buffer into a abstract syntax tree representing the graph.
func Parse(buf []byte) (*ast.Graph, error) {
	return parser.ParseBytes(buf)
}

//Parses and creates a new Graph from the data.
func Read(buf []byte) (Interface, error) {
	st, err := Parse(buf)
	if err != nil {
		return nil, err
	}
	return NewAnalysedGraph(st), nil
}
