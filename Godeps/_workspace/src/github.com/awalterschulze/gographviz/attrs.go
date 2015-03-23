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
	"fmt"
	"os"
	"sort"
)

//Represents attributes for an Edge, Node or Graph.
type Attrs map[string]string

//Creates an empty Attributes type.
func NewAttrs() Attrs {
	return make(Attrs)
}

//Adds an attribute name and value.
func (this Attrs) Add(field string, value string) {
	prev, ok := this[field]
	if ok {
		fmt.Fprintf(os.Stderr, "WARNING: overwriting field %v value %v, with value %v\n", field, prev, value)
	}
	this[field] = value
}

//Adds the attributes into this Attrs type overwriting duplicates.
func (this Attrs) Extend(more Attrs) {
	for key, value := range more {
		this.Add(key, value)
	}
}

//Only adds the missing attributes to this Attrs type.
func (this Attrs) Ammend(more Attrs) {
	for key, value := range more {
		if _, ok := this[key]; !ok {
			this.Add(key, value)
		}
	}
}

func (this Attrs) SortedNames() []string {
	keys := make([]string, 0)
	for key := range this {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (this Attrs) Copy() Attrs {
	attrs := make(Attrs)
	for k, v := range this {
		attrs[k] = v
	}
	return attrs
}
