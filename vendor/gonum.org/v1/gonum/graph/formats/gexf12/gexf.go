// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gexf12 implements marshaling and unmarshaling of GEXF1.2 documents.
//
// For details of GEXF see https://gephi.org/gexf/format/.
package gexf12 // import "gonum.org/v1/gonum/graph/formats/gexf12"

import (
	"bytes"
	"encoding/xml"
	"time"
)

// BUG(kortschak): The namespace for GEFX1.2 is 1.2draft, though it has
// already been deprecated. There is no specification for 1.3, although
// it is being used in the wild.

// Content holds a GEFX graph and metadata.
type Content struct {
	XMLName xml.Name `xml:"http://www.gexf.net/1.2draft gexf"`
	Meta    *Meta    `xml:"meta,omitempty"`
	Graph   Graph    `xml:"graph"`
	// Version must be "1.2".
	Version string `xml:"version,attr"`
	Variant string `xml:"variant,attr,omitempty"`
}

// Meta holds optional metadata associated with the graph.
type Meta struct {
	Creator      string    `xml:"creator,omitempty"`
	Keywords     string    `xml:"keywords,omitempty"`
	Description  string    `xml:"description,omitempty"`
	LastModified time.Time `xml:"lastmodifieddate,attr,omitempty"`
}

// MarshalXML implements the xml.Marshaler interface.
func (t *Meta) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	type T Meta
	var layout struct {
		*T
		LastModified *xsdDate `xml:"lastmodifieddate,attr,omitempty"`
	}
	layout.T = (*T)(t)
	layout.LastModified = (*xsdDate)(&layout.T.LastModified)
	return e.EncodeElement(layout, start)
}

// UnmarshalXML implements the xml.Unmarshaler interface.
func (t *Meta) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type T Meta
	var overlay struct {
		*T
		LastModified *xsdDate `xml:"lastmodifieddate,attr,omitempty"`
	}
	overlay.T = (*T)(t)
	overlay.LastModified = (*xsdDate)(&overlay.T.LastModified)
	return d.DecodeElement(&overlay, &start)
}

// Graph stores the graph nodes, edges, dynamics and visualization data.
type Graph struct {
	Attributes []Attributes `xml:"attributes"`
	Nodes      Nodes        `xml:"nodes"`
	Edges      Edges        `xml:"edges"`
	// TimeFormat may be one of "integer", "double", "date" or "dateTime".
	TimeFormat string `xml:"timeformat,attr,omitempty"`
	Start      string `xml:"start,attr,omitempty"`
	StartOpen  string `xml:"startopen,attr,omitempty"`
	End        string `xml:"end,attr,omitempty"`
	EndOpen    string `xml:"endopen,attr,omitempty"`
	// DefaultEdgeType may be one of "directed", "undirected" or "mutual".
	DefaultEdgeType string `xml:"defaultedgetype,attr,omitempty"`
	// IDType may be one of "integer" or "string".
	IDType string `xml:"idtype,attr,omitempty"`
	// Mode may be "static" or "dynamic".
	Mode string `xml:"mode,attr,omitempty"`
}

// Attributes holds a collection of potentially dynamic attributes
// associated with a graph.
type Attributes struct {
	Attributes []Attribute `xml:"attribute,omitempty"`
	// Class be one of "node" or "edge".
	Class string `xml:"class,attr"`
	// Mode may be "static" or "dynamic".
	Mode      string `xml:"mode,attr,omitempty"`
	Start     string `xml:"start,attr,omitempty"`
	StartOpen string `xml:"startopen,attr,omitempty"`
	End       string `xml:"end,attr,omitempty"`
	EndOpen   string `xml:"endopen,attr,omitempty"`
}

// Attribute holds a single graph attribute.
type Attribute struct {
	ID    string `xml:"id,attr"`
	Title string `xml:"title,attr"`
	// Type may be one of "integer", "long", "double", "float",
	// "boolean", "liststring", "string", or "anyURI".
	Type    string `xml:"type,attr"`
	Default string `xml:"default,omitempty"`
	Options string `xml:"options,omitempty"`
}

// Nodes holds a collection of nodes constituting a graph or subgraph.
type Nodes struct {
	Count int    `xml:"count,attr,omitempty"`
	Nodes []Node `xml:"node,omitempty"`
}

// Node is a single node and its associated attributes.
type Node struct {
	ID        string     `xml:"id,attr,omitempty"`
	Label     string     `xml:"label,attr,omitempty"`
	AttValues *AttValues `xml:"attvalues"`
	Spells    *Spells    `xml:"spells"`
	Nodes     *Nodes     `xml:"nodes"`
	Edges     *Edges     `xml:"edges"`
	ParentID  string     `xml:"pid,attr,omitempty"`
	Parents   *Parents   `xml:"parents"`
	Color     *Color     `xml:"http://www.gexf.net/1.2draft/viz color"`
	Position  *Position  `xml:"http://www.gexf.net/1.2draft/viz position"`
	Size      *Size      `xml:"http://www.gexf.net/1.2draft/viz size"`
	Shape     *NodeShape `xml:"http://www.gexf.net/1.2draft/viz shape"`
	Start     string     `xml:"start,attr,omitempty"`
	StartOpen string     `xml:"startopen,attr,omitempty"`
	End       string     `xml:"end,attr,omitempty"`
	EndOpen   string     `xml:"endopen,attr,omitempty"`
}

// NodeShape holds the visual representation of a node with associated
// dynamics.
type NodeShape struct {
	Spells *Spells `xml:"spells,omitempty"`
	// Value be one of "disc", "square", "triangle",
	// "diamond" or "image".
	Shape     string `xml:"value,attr"`
	URI       string `xml:"uri,attr,omitempty"`
	Start     string `xml:"start,attr,omitempty"`
	StartOpen string `xml:"startopen,attr,omitempty"`
	End       string `xml:"end,attr,omitempty"`
	EndOpen   string `xml:"endopen,attr,omitempty"`
}

// Color represents a node or edge color and its associated dynamics.
type Color struct {
	Spells    *Spells `xml:"spells,omitempty"`
	R         byte    `xml:"r,attr"`
	G         byte    `xml:"g,attr"`
	B         byte    `xml:"b,attr"`
	A         float64 `xml:"a,attr,omitempty"`
	Start     string  `xml:"start,attr,omitempty"`
	StartOpen string  `xml:"startopen,attr,omitempty"`
	End       string  `xml:"end,attr,omitempty"`
	EndOpen   string  `xml:"endopen,attr,omitempty"`
}

// Edges holds a collection of edges constituting a graph or subgraph.
type Edges struct {
	Count int    `xml:"count,attr,omitempty"`
	Edges []Edge `xml:"edge,omitempty"`
}

// Edge is a single edge and its associated attributes.
type Edge struct {
	ID        string     `xml:"id,attr,omitempty"`
	AttValues *AttValues `xml:"attvalues"`
	Spells    *Spells    `xml:"spells"`
	Color     *Color     `xml:"http://www.gexf.net/1.2draft/viz color"`
	Thickness *Thickness `xml:"http://www.gexf.net/1.2draft/viz thickness"`
	Shape     *Edgeshape `xml:"http://www.gexf.net/1.2draft/viz shape"`
	Start     string     `xml:"start,attr,omitempty"`
	StartOpen string     `xml:"startopen,attr,omitempty"`
	End       string     `xml:"end,attr,omitempty"`
	EndOpen   string     `xml:"endopen,attr,omitempty"`
	// Type may be one of directed, undirected, mutual
	Type   string  `xml:"type,attr,omitempty"`
	Label  string  `xml:"label,attr,omitempty"`
	Source string  `xml:"source,attr"`
	Target string  `xml:"target,attr"`
	Weight float64 `xml:"weight,attr,omitempty"`
}

// AttVlues holds a collection of attribute values.
type AttValues struct {
	AttValues []AttValue `xml:"attvalue,omitempty"`
}

// AttValues holds a single attribute value and its associated dynamics.
type AttValue struct {
	For       string `xml:"for,attr"`
	Value     string `xml:"value,attr"`
	Start     string `xml:"start,attr,omitempty"`
	StartOpen string `xml:"startopen,attr,omitempty"`
	End       string `xml:"end,attr,omitempty"`
	EndOpen   string `xml:"endopen,attr,omitempty"`
}

// EdgeShape holds the visual representation of an edge with associated
// dynamics.
type Edgeshape struct {
	// Shape be one of solid, dotted, dashed, double
	Shape     string  `xml:"value,attr"`
	Spells    *Spells `xml:"spells,omitempty"`
	Start     string  `xml:"start,attr,omitempty"`
	StartOpen string  `xml:"startopen,attr,omitempty"`
	End       string  `xml:"end,attr,omitempty"`
	EndOpen   string  `xml:"endopen,attr,omitempty"`
}

// Parents holds parent relationships between nodes in a hierarchical
// graph.
type Parents struct {
	Parents []Parent `xml:"parent,omitempty"`
}

// Parent is a single parent relationship.
type Parent struct {
	For string `xml:"for,attr"`
}

// Position hold the spatial position of a node and its dynamics.
type Position struct {
	X         float64 `xml:"x,attr"`
	Y         float64 `xml:"y,attr"`
	Z         float64 `xml:"z,attr"`
	Spells    *Spells `xml:"spells,omitempty"`
	Start     string  `xml:"start,attr,omitempty"`
	StartOpen string  `xml:"startopen,attr,omitempty"`
	End       string  `xml:"end,attr,omitempty"`
	EndOpen   string  `xml:"endopen,attr,omitempty"`
}

// Size hold the visual size of a node and its dynamics.
type Size struct {
	Value     float64 `xml:"value,attr"`
	Spells    *Spells `xml:"http://www.gexf.net/1.2draft/viz spells,omitempty"`
	Start     string  `xml:"start,attr,omitempty"`
	StartOpen string  `xml:"startopen,attr,omitempty"`
	End       string  `xml:"end,attr,omitempty"`
	EndOpen   string  `xml:"endopen,attr,omitempty"`
}

// Thickness hold the visual thickness of an edge and its dynamics.
type Thickness struct {
	Value     float64 `xml:"value,attr"`
	Spells    *Spells `xml:"http://www.gexf.net/1.2draft/viz spells,omitempty"`
	Start     string  `xml:"start,attr,omitempty"`
	StartOpen string  `xml:"startopen,attr,omitempty"`
	End       string  `xml:"end,attr,omitempty"`
	EndOpen   string  `xml:"endopen,attr,omitempty"`
}

// Spells holds a collection of time dynamics for a graph entity.
type Spells struct {
	Spells []Spell `xml:"spell"`
}

// Spell is a time interval.
type Spell struct {
	Start     string `xml:"start,attr,omitempty"`
	StartOpen string `xml:"startopen,attr,omitempty"`
	End       string `xml:"end,attr,omitempty"`
	EndOpen   string `xml:"endopen,attr,omitempty"`
}

type xsdDate time.Time

func (t *xsdDate) UnmarshalText(text []byte) error {
	return _unmarshalTime(text, (*time.Time)(t), "2006-01-02")
}

func (t xsdDate) MarshalText() ([]byte, error) {
	return []byte((time.Time)(t).Format("2006-01-02")), nil
}

func (t xsdDate) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if (time.Time)(t).IsZero() {
		return nil
	}
	m, err := t.MarshalText()
	if err != nil {
		return err
	}
	return e.EncodeElement(m, start)
}

func (t xsdDate) MarshalXMLAttr(name xml.Name) (xml.Attr, error) {
	if (time.Time)(t).IsZero() {
		return xml.Attr{}, nil
	}
	m, err := t.MarshalText()
	return xml.Attr{Name: name, Value: string(m)}, err
}

func _unmarshalTime(text []byte, t *time.Time, format string) (err error) {
	s := string(bytes.TrimSpace(text))
	*t, err = time.Parse(format, s)
	if _, ok := err.(*time.ParseError); ok {
		*t, err = time.Parse(format+"Z07:00", s)
	}
	return err
}
