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

//Represents the relations between graphs and nodes.
//Each node belongs the main graph or a subgraph.
type Relations struct {
	ParentToChildren map[string]map[string]bool
	ChildToParents   map[string]map[string]bool
}

//Creates an empty set of relations.
func NewRelations() *Relations {
	return &Relations{make(map[string]map[string]bool), make(map[string]map[string]bool)}
}

//Adds a node to a parent graph.
func (this *Relations) Add(parent string, child string) {
	if _, ok := this.ParentToChildren[parent]; !ok {
		this.ParentToChildren[parent] = make(map[string]bool)
	}
	this.ParentToChildren[parent][child] = true
	if _, ok := this.ChildToParents[child]; !ok {
		this.ChildToParents[child] = make(map[string]bool)
	}
	this.ChildToParents[child][parent] = true
}

func (this *Relations) SortedChildren(parent string) []string {
	keys := make([]string, 0)
	for key := range this.ParentToChildren[parent] {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
