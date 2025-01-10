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

package json

import (
	"github.com/coreos/vcontext/tree"
	// todo: rewrite this dep
	json "github.com/coreos/go-json"
)

func UnmarshalToContext(raw []byte) (tree.Node, error) {
	var ast json.Node
	if err := json.Unmarshal(raw, &ast); err != nil {
		return nil, err
	}
	node := fromJsonNode(ast)
	tree.FixLineColumn(node, raw)
	return node, nil
}

func fromJsonNode(n json.Node) tree.Node {
	m := tree.MarkerFromIndices(int64(n.Start), int64(n.End))

	switch v := n.Value.(type) {
	case map[string]json.Node:
		ret := tree.MapNode{
			Marker:   m,
			Children: make(map[string]tree.Node, len(v)),
			Keys:     make(map[string]tree.Leaf, len(v)),
		}
		for key, child := range v {
			ret.Children[key] = fromJsonNode(child)
			ret.Keys[key] = tree.Leaf{
				Marker: tree.MarkerFromIndices(int64(child.KeyStart), int64(child.KeyEnd)),
			}
		}
		return ret
	case []json.Node:
		ret := tree.SliceNode{
			Marker:   m,
			Children: make([]tree.Node, 0, len(v)),
		}
		for _, child := range v {
			ret.Children = append(ret.Children, fromJsonNode(child))
		}
		return ret
	default:
		return tree.Leaf{
			Marker: m,
		}
	}
}
