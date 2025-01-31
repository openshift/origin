// Copyright 2017 CoreOS, Inc.
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
// limitations under the License.

package astnode

import (
	"io"
)

// AstNode abstracts the differences between yaml and json nodes, providing a
// common interface
type AstNode interface {
	// ValueLineCol returns the line, column, and highlight string of the value of
	// this node in the source.
	ValueLineCol(source io.ReadSeeker) (int, int, string)

	// KeyLineCol returns the line, column, and highlight string of the key for the
	// value of this node in the source.
	KeyLineCol(source io.ReadSeeker) (int, int, string)

	// LiteralValue returns the value of this node.
	LiteralValue() interface{}

	// SliceChild returns the child node at the index specified. If this node is not
	// a slice node, an empty AstNode and false is returned.
	SliceChild(index int) (AstNode, bool)

	// KeyValueMap returns a map of keys and values. If this node is not a mapping
	// node, nil and false are returned.
	KeyValueMap() (map[string]AstNode, bool)

	// Tag returns the struct tag used in the config structure used to unmarshal.
	Tag() string
}
