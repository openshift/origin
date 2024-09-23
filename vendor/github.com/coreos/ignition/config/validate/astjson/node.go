// Copyright 2016 CoreOS, Inc.
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

package astjson

import (
	"io"

	json "github.com/ajeddeloh/go-json"
	"github.com/coreos/ignition/config/validate/astnode"
	"go4.org/errorutil"
)

type JsonNode json.Node

func FromJsonRoot(n json.Node) JsonNode {
	return JsonNode(n)
}

func (n JsonNode) ValueLineCol(source io.ReadSeeker) (int, int, string) {
	return posFromOffset(n.End, source)
}

func (n JsonNode) KeyLineCol(source io.ReadSeeker) (int, int, string) {
	return posFromOffset(n.KeyEnd, source)
}

func (n JsonNode) LiteralValue() interface{} {
	return n.Value
}

func (n JsonNode) SliceChild(index int) (astnode.AstNode, bool) {
	if slice, ok := n.Value.([]json.Node); ok {
		return JsonNode(slice[index]), true
	}
	return JsonNode{}, false
}

func (n JsonNode) KeyValueMap() (map[string]astnode.AstNode, bool) {
	if kvmap, ok := n.Value.(map[string]json.Node); ok {
		newKvmap := map[string]astnode.AstNode{}
		for k, v := range kvmap {
			newKvmap[k] = JsonNode(v)
		}
		return newKvmap, true
	}
	return nil, false
}

func (n JsonNode) Tag() string {
	return "json"
}

// wrapper for errorutil that handles missing sources sanely and resets the reader afterwards
func posFromOffset(offset int, source io.ReadSeeker) (int, int, string) {
	if source == nil {
		return 0, 0, ""
	}
	line, col, highlight := errorutil.HighlightBytePosition(source, int64(offset))
	source.Seek(0, 0) // Reset the reader to the start so the next call isn't relative to this position
	return line, col, highlight
}
