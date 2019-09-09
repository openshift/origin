// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sigmajs implements marshaling and unmarshaling of Sigma.js JSON documents.
//
// See http://sigmajs.org/ for Sigma.js documentation.
package sigmajs // import "gonum.org/v1/gonum/graph/formats/sigmajs"

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Graph is a Sigma.js graph.
type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// Node is a Sigma.js node.
type Node struct {
	ID         string
	Attributes map[string]interface{}
}

var (
	_ json.Marshaler   = (*Node)(nil)
	_ json.Unmarshaler = (*Node)(nil)
)

// MarshalJSON implements the json.Marshaler interface.
func (n *Node) MarshalJSON() ([]byte, error) {
	if n.Attributes == nil {
		type node struct {
			ID string `json:"id"`
		}
		return json.Marshal(node{ID: n.ID})
	}
	n.Attributes["id"] = n.ID
	b, err := json.Marshal(n.Attributes)
	delete(n.Attributes, "id")
	return b, err
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (n *Node) UnmarshalJSON(data []byte) error {
	var attrs map[string]interface{}
	err := json.Unmarshal(data, &attrs)
	if err != nil {
		return err
	}
	id, ok := attrs["id"]
	if !ok {
		return errors.New("sigmajs: no ID")
	}
	n.ID = fmt.Sprint(id)
	delete(attrs, "id")
	if len(attrs) != 0 {
		n.Attributes = attrs
	}
	return nil
}

// Edge is a Sigma.js edge.
type Edge struct {
	ID         string
	Source     string
	Target     string
	Attributes map[string]interface{}
}

var (
	_ json.Marshaler   = (*Edge)(nil)
	_ json.Unmarshaler = (*Edge)(nil)
)

// MarshalJSON implements the json.Marshaler interface.
func (e *Edge) MarshalJSON() ([]byte, error) {
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
func (e *Edge) UnmarshalJSON(data []byte) error {
	var attrs map[string]interface{}
	err := json.Unmarshal(data, &attrs)
	if err != nil {
		return err
	}
	id, ok := attrs["id"]
	if !ok {
		return errors.New("sigmajs: no ID")
	}
	source, ok := attrs["source"]
	if !ok {
		return errors.New("sigmajs: no source")
	}
	target, ok := attrs["target"]
	if !ok {
		return errors.New("sigmajs: no target")
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
