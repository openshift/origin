// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package cytoscapejs implements marshaling and unmarshaling of Cytoscape.js JSON documents.
//
// See http://js.cytoscape.org/ for Cytoscape.js documentation.
package cytoscapejs // import "gonum.org/v1/gonum/graph/formats/cytoscapejs"

import (
	"encoding/json"
	"errors"
	"fmt"
)

// GraphElem is a Cytoscape.js graph with mixed graph elements.
type GraphElem struct {
	Elements []Element     `json:"elements"`
	Layout   interface{}   `json:"layout,omitempty"`
	Style    []interface{} `json:"style,omitempty"`
}

// Element is a mixed graph element.
type Element struct {
	Group            string      `json:"group,omitempty"`
	Data             ElemData    `json:"data"`
	Position         *Position   `json:"position,omitempty"`
	RenderedPosition *Position   `json:"renderedPosition,omitempty"`
	Selected         bool        `json:"selected,omitempty"`
	Selectable       bool        `json:"selectable,omitempty"`
	Locked           bool        `json:"locked,omitempty"`
	Grabbable        bool        `json:"grabbable,omitempty"`
	Classes          string      `json:"classes,omitempty"`
	Scratch          interface{} `json:"scratch,omitempty"`
}

// ElemType describes an Element type.
type ElemType int

const (
	InvalidElement ElemType = iota - 1
	NodeElement
	EdgeElement
)

// Type returns the element type of the receiver. It returns an error if the Element Group
// is invalid or does not match the Element Data, or if the Elelement Data is an incomplete
// edge.
func (e Element) Type() (ElemType, error) {
	et := InvalidElement
	switch {
	case e.Data.Source == "" && e.Data.Target == "":
		et = NodeElement
	case e.Data.Source != "" && e.Data.Target != "":
		et = EdgeElement
	default:
		return et, errors.New("cytoscapejs: invalid element: incomplete edge")
	}
	switch {
	case e.Group == "":
		return et, nil
	case e.Group == "node" && et == NodeElement:
		return NodeElement, nil
	case e.Group == "edge" && et == EdgeElement:
		return NodeElement, nil
	default:
		return InvalidElement, errors.New("cytoscapejs: invalid element: mismatched group")
	}
}

// ElemData is a graph element's data container.
type ElemData struct {
	ID         string
	Source     string
	Target     string
	Parent     string
	Attributes map[string]interface{}
}

var (
	_ json.Marshaler   = (*ElemData)(nil)
	_ json.Unmarshaler = (*ElemData)(nil)
)

// MarshalJSON implements the json.Marshaler interface.
func (e *ElemData) MarshalJSON() ([]byte, error) {
	if e.Attributes == nil {
		type elem struct {
			ID     string `json:"id"`
			Source string `json:"source,omitempty"`
			Target string `json:"target,omitempty"`
			Parent string `json:"parent,omitempty"`
		}
		return json.Marshal(elem{ID: e.ID, Source: e.Source, Target: e.Target, Parent: e.Parent})
	}
	e.Attributes["id"] = e.ID
	if e.Source != "" {
		e.Attributes["source"] = e.Source
	}
	if e.Target != "" {
		e.Attributes["target"] = e.Target
	}
	if e.Parent != "" {
		e.Attributes["parent"] = e.Parent
	}
	b, err := json.Marshal(e.Attributes)
	delete(e.Attributes, "id")
	if e.Source != "" {
		delete(e.Attributes, "source")
	}
	if e.Target != "" {
		delete(e.Attributes, "target")
	}
	if e.Parent != "" {
		delete(e.Attributes, "parent")
	}
	return b, err
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (e *ElemData) UnmarshalJSON(data []byte) error {
	var attrs map[string]interface{}
	err := json.Unmarshal(data, &attrs)
	if err != nil {
		return err
	}
	id, ok := attrs["id"]
	if !ok {
		return errors.New("cytoscapejs: no ID")
	}
	e.ID = fmt.Sprint(id)
	source, ok := attrs["source"]
	if ok {
		e.Source = fmt.Sprint(source)
	}
	target, ok := attrs["target"]
	if ok {
		e.Target = fmt.Sprint(target)
	}
	p, ok := attrs["parent"]
	if ok {
		e.Parent = fmt.Sprint(p)
	}
	delete(attrs, "id")
	delete(attrs, "source")
	delete(attrs, "target")
	delete(attrs, "parent")
	if len(attrs) != 0 {
		e.Attributes = attrs
	}
	return nil
}

// GraphNodeEdge is a Cytoscape.js graph with separated nodes and edges.
type GraphNodeEdge struct {
	Elements Elements      `json:"elements"`
	Layout   interface{}   `json:"layout,omitempty"`
	Style    []interface{} `json:"style,omitempty"`
}

// Elements contains the nodes and edges of a GraphNodeEdge.
type Elements struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// Node is a Cytoscape.js node.
type Node struct {
	Data             NodeData    `json:"data"`
	Position         *Position   `json:"position,omitempty"`
	RenderedPosition *Position   `json:"renderedPosition,omitempty"`
	Selected         bool        `json:"selected,omitempty"`
	Selectable       bool        `json:"selectable,omitempty"`
	Locked           bool        `json:"locked,omitempty"`
	Grabbable        bool        `json:"grabbable,omitempty"`
	Classes          string      `json:"classes,omitempty"`
	Scratch          interface{} `json:"scratch,omitempty"`
}

// NodeData is a graph node's data container.
type NodeData struct {
	ID         string
	Parent     string
	Attributes map[string]interface{}
}

var (
	_ json.Marshaler   = (*NodeData)(nil)
	_ json.Unmarshaler = (*NodeData)(nil)
)

// MarshalJSON implements the json.Marshaler interface.
func (n *NodeData) MarshalJSON() ([]byte, error) {
	if n.Attributes == nil {
		type node struct {
			ID     string `json:"id"`
			Parent string `json:"parent,omitempty"`
		}
		return json.Marshal(node{ID: n.ID, Parent: n.Parent})
	}
	n.Attributes["id"] = n.ID
	n.Attributes["parent"] = n.Parent
	b, err := json.Marshal(n.Attributes)
	delete(n.Attributes, "id")
	delete(n.Attributes, "parent")
	return b, err
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (n *NodeData) UnmarshalJSON(data []byte) error {
	var attrs map[string]interface{}
	err := json.Unmarshal(data, &attrs)
	if err != nil {
		return err
	}
	id, ok := attrs["id"]
	if !ok {
		return errors.New("cytoscapejs: no ID")
	}
	n.ID = fmt.Sprint(id)
	delete(attrs, "id")
	p, ok := attrs["parent"]
	if ok {
		n.Parent = fmt.Sprint(p)
	}
	delete(attrs, "parent")
	if len(attrs) != 0 {
		n.Attributes = attrs
	}
	return nil
}

// Edge is a Cytoscape.js edge.
type Edge struct {
	Data       EdgeData    `json:"data"`
	Selected   bool        `json:"selected,omitempty"`
	Selectable bool        `json:"selectable,omitempty"`
	Classes    string      `json:"classes,omitempty"`
	Scratch    interface{} `json:"scratch,omitempty"`
}

// EdgeData is a graph edge's data container.
type EdgeData struct {
	ID         string
	Source     string
	Target     string
	Attributes map[string]interface{}
}

var (
	_ json.Marshaler   = (*EdgeData)(nil)
	_ json.Unmarshaler = (*EdgeData)(nil)
)

// MarshalJSON implements the json.Marshaler interface.
func (e *EdgeData) MarshalJSON() ([]byte, error) {
	if e.Attributes == nil {
		type edge struct {
			ID     string `json:"id"`
			Source string `json:"source"`
			Target string `json:"target"`
		}
		return json.Marshal(edge{ID: e.ID, Source: e.Source, Target: e.Target})
	}
	e.Attributes["id"] = e.ID
	e.Attributes["source"] = e.Source
	e.Attributes["target"] = e.Target
	b, err := json.Marshal(e.Attributes)
	delete(e.Attributes, "id")
	delete(e.Attributes, "source")
	delete(e.Attributes, "target")
	return b, err
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (e *EdgeData) UnmarshalJSON(data []byte) error {
	var attrs map[string]interface{}
	err := json.Unmarshal(data, &attrs)
	if err != nil {
		return err
	}
	id, ok := attrs["id"]
	if !ok {
		return errors.New("cytoscapejs: no ID")
	}
	source, ok := attrs["source"]
	if !ok {
		return errors.New("cytoscapejs: no source")
	}
	target, ok := attrs["target"]
	if !ok {
		return errors.New("cytoscapejs: no target")
	}
	e.ID = fmt.Sprint(id)
	e.Source = fmt.Sprint(source)
	e.Target = fmt.Sprint(target)
	delete(attrs, "id")
	delete(attrs, "source")
	delete(attrs, "target")
	if len(attrs) != 0 {
		e.Attributes = attrs
	}
	return nil
}

// Position is a node position.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
