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

//Represents a Node.
type Node struct {
	Name  string
	Attrs Attrs
}

//Represents a set of Nodes.
type Nodes struct {
	Lookup map[string]*Node
	Nodes  []*Node
}

//Creates a new set of Nodes.
func NewNodes() *Nodes {
	return &Nodes{make(map[string]*Node), make([]*Node, 0)}
}

//Adds a Node to the set of Nodes, ammending the attributes of an already existing node.
func (this *Nodes) Add(node *Node) {
	n, ok := this.Lookup[node.Name]
	if ok {
		n.Attrs.Ammend(node.Attrs)
		return
	}
	this.Lookup[node.Name] = node
	this.Nodes = append(this.Nodes, node)
}

//Returns a sorted list of nodes.
func (this Nodes) Sorted() []*Node {
	keys := make([]string, 0, len(this.Lookup))
	for key := range this.Lookup {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	nodes := make([]*Node, len(keys))
	for i := range keys {
		nodes[i] = this.Lookup[keys[i]]
	}
	return nodes
}
