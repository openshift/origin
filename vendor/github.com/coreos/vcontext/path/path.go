// Copyright 2019 Red Hat, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.)

package path

import (
	"fmt"
	"strings"
)

type ContextPath struct {
	Path []interface{}
	Tag  string
}

// New returns a new ContextPath with the given tag and path. It's a helper since go's
// literal syntax is quite verbose, especially with []interface{}{...}.
func New(tag string, path ...interface{}) ContextPath {
	return ContextPath{
		Tag:  tag,
		Path: path,
	}
}

func (c ContextPath) String() string {
	strs := []string{"$"}
	for _, e := range c.Path {
		strs = append(strs, fmt.Sprintf("%v", e))
	}
	return strings.Join(strs, ".")
}

// Append returns a new ContextPath with the specified elements appended.
// The underlying array is sometimes reused, so if the original path might
// be used in future Append operations, the returned ContextPath should not
// be stored into a long-lived data structure.  (Store a copy instead.)
func (c ContextPath) Append(e ...interface{}) ContextPath {
	return ContextPath{
		Path: append(c.Path, e...),
		Tag:  c.Tag,
	}
}

// Copy returns an identical ContextPath that shares no state with the
// original.  When storing a ContextPath into a data structure, usually a
// copy should be stored instead of the original (for example, Report does
// this), since Append sometimes modifies the path's underlying array in
// place.
func (c ContextPath) Copy() ContextPath {
	// make sure to preserve reflect.DeepEqual() equality
	var path []interface{}
	if c.Path != nil {
		path = append(path, c.Path...)
	}
	return ContextPath{
		Path: path,
		Tag:  c.Tag,
	}
}

// Head returns the first element in the path, panics if empty.
func (c ContextPath) Head() interface{} {
	return c.Path[0]
}

func (c ContextPath) Tail() ContextPath {
	if len(c.Path) == 0 {
		return ContextPath{}
	}
	return ContextPath{
		Path: c.Path[1:],
		Tag:  c.Tag,
	}
}

func (c ContextPath) Pop() ContextPath {
	if len(c.Path) == 0 {
		return ContextPath{}
	}
	return ContextPath{
		Path: c.Path[:c.Len()-1],
		Tag:  c.Tag,
	}
}

func (c ContextPath) Len() int {
	return len(c.Path)
}
