// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package graphql

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/encoding"
)

// Unmarshal parses the the JSON-encoded data and stores the result in dst.
// Node IDs are obtained from the JSON fields identified by the uid parameter.
// UIDs obtained from the JSON encoding must map to unique node ID values
// consistently across the JSON-encoded spanning tree.
func Unmarshal(data []byte, uid string, dst encoding.Builder) error {
	if uid == "" {
		return errors.New("graphql: invalid UID field name")
	}
	var src json.RawMessage
	err := json.Unmarshal(data, &src)
	if err != nil {
		return err
	}
	gen := generator{dst: dst, uidName: uid, nodes: make(map[string]graph.Node)}
	return gen.walk(src, nil, "")
}

// StringIDSetter is a graph node that can set its ID based on the given uid string.
type StringIDSetter interface {
	SetIDFromString(uid string) error
}

// LabelSetter is a graph edge that can set its label.
type LabelSetter interface {
	SetLabel(string)
}

type generator struct {
	dst encoding.Builder

	// uidName is the name of the UID field in the source JSON.
	uidName string
	// nodes maps from GraphQL UID string to graph.Node.
	nodes map[string]graph.Node
}

func (g *generator) walk(src json.RawMessage, node graph.Node, attr string) error {
	switch src[0] {
	case '{':
		var val map[string]json.RawMessage
		err := json.Unmarshal(src, &val)
		if err != nil {
			return err
		}
		if next, ok := val[g.uidName]; !ok {
			if node != nil {
				var buf bytes.Buffer
				err := json.Compact(&buf, src)
				if err != nil {
					panic(err)
				}
				return fmt.Errorf("graphql: no UID for node: `%s`", &buf)
			}
		} else {
			var v interface{}
			err = json.Unmarshal(next, &v)
			if err != nil {
				return err
			}
			value := fmt.Sprint(v)
			child, ok := g.nodes[value]
			if !ok {
				child = g.dst.NewNode()
				s, ok := child.(StringIDSetter)
				if !ok {
					return errors.New("graphql: cannot set UID")
				}
				err = s.SetIDFromString(value)
				if err != nil {
					return err
				}
				g.nodes[value] = child
				g.dst.AddNode(child)
			}
			if node != nil {
				e := g.dst.NewEdge(node, child)
				if s, ok := e.(LabelSetter); ok {
					s.SetLabel(attr)
				}
				g.dst.SetEdge(e)
			}
			node = child
		}
		for attr, src := range val {
			if attr == g.uidName {
				continue
			}
			err = g.walk(src, node, attr)
			if err != nil {
				return err
			}
		}

	case '[':
		var val []json.RawMessage
		err := json.Unmarshal(src, &val)
		if err != nil {
			return err
		}
		for _, src := range val {
			err = g.walk(src, node, attr)
			if err != nil {
				return err
			}
		}

	default:
		var v interface{}
		err := json.Unmarshal(src, &v)
		if err != nil {
			return err
		}
		if attr == g.uidName {
			value := fmt.Sprint(v)
			if s, ok := node.(StringIDSetter); ok {
				if _, ok := g.nodes[value]; !ok {
					err = s.SetIDFromString(value)
					if err != nil {
						return err
					}
					g.nodes[value] = node
				}
			} else {
				return errors.New("graphql: cannot set ID")
			}
		} else if s, ok := node.(encoding.AttributeSetter); ok {
			var value string
			if _, ok := v.(float64); ok {
				value = string(src)
			} else {
				value = fmt.Sprint(v)
			}
			err = s.SetAttribute(encoding.Attribute{Key: attr, Value: value})
			if err != nil {
				return err
			}
		}
	}

	return nil
}
