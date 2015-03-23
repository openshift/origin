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

//Abstract Syntax Tree representing the DOT grammar
package ast

import (
	"errors"
	"fmt"
	"github.com/awalterschulze/gographviz/token"
	"math/rand"
	"sort"
	"strings"
)

var (
	r = rand.New(rand.NewSource(1234))
)

type Visitor interface {
	Visit(e Elem) Visitor
}

type Elem interface {
	String() string
}

type Walkable interface {
	Walk(v Visitor)
}

type Bool bool

const (
	FALSE = Bool(false)
	TRUE  = Bool(true)
)

func (this Bool) String() string {
	switch this {
	case false:
		return "false"
	case true:
		return "true"
	}
	panic("unreachable")
}

func (this Bool) Walk(v Visitor) {
	if v == nil {
		return
	}
	v.Visit(this)
}

type GraphType bool

const (
	GRAPH   = GraphType(false)
	DIGRAPH = GraphType(true)
)

func (this GraphType) String() string {
	switch this {
	case false:
		return "graph"
	case true:
		return "digraph"
	}
	panic("unreachable")
}

func (this GraphType) Walk(v Visitor) {
	if v == nil {
		return
	}
	v.Visit(this)
}

type Graph struct {
	Type     GraphType
	Strict   bool
	Id       Id
	StmtList StmtList
}

func NewGraph(t, strict, id, l Elem) (*Graph, error) {
	g := &Graph{Type: t.(GraphType), Strict: bool(strict.(Bool)), Id: Id("")}
	if id != nil {
		g.Id = id.(Id)
	}
	if l != nil {
		g.StmtList = l.(StmtList)
	}
	return g, nil
}

func (this *Graph) String() string {
	s := this.Type.String() + " " + this.Id.String() + " {\n"
	if this.StmtList != nil {
		s += this.StmtList.String()
	}
	s += "\n}\n"
	return s
}

func (this *Graph) Walk(v Visitor) {
	if v == nil {
		return
	}
	v = v.Visit(this)
	this.Type.Walk(v)
	this.Id.Walk(v)
	this.StmtList.Walk(v)
}

type StmtList []Stmt

func NewStmtList(s Elem) (StmtList, error) {
	ss := make(StmtList, 1)
	ss[0] = s.(Stmt)
	return ss, nil
}

func AppendStmtList(ss, s Elem) (StmtList, error) {
	this := ss.(StmtList)
	this = append(this, s.(Stmt))
	return this, nil
}

func (this StmtList) String() string {
	if len(this) == 0 {
		return ""
	}
	s := ""
	for i := 0; i < len(this); i++ {
		ss := this[i].String()
		if len(ss) > 0 {
			s += "\t" + ss + ";\n"
		}
	}
	return s
}

func (this StmtList) Walk(v Visitor) {
	if v == nil {
		return
	}
	v = v.Visit(this)
	for i := range this {
		this[i].Walk(v)
	}
}

type Stmt interface {
	Elem
	Walkable
	isStmt()
}

func (this NodeStmt) isStmt()   {}
func (this EdgeStmt) isStmt()   {}
func (this EdgeAttrs) isStmt()  {}
func (this NodeAttrs) isStmt()  {}
func (this GraphAttrs) isStmt() {}
func (this *SubGraph) isStmt()  {}
func (this *Attr) isStmt()      {}

type SubGraph struct {
	Id       Id
	StmtList StmtList
}

func NewSubGraph(id, l Elem) (*SubGraph, error) {
	g := &SubGraph{Id: Id(fmt.Sprintf("anon%d", r.Int63()))}
	if id != nil {
		if len(id.(Id)) > 0 {
			g.Id = id.(Id)
		}
	}
	if l != nil {
		g.StmtList = l.(StmtList)
	}
	return g, nil
}

func (this *SubGraph) GetId() Id {
	return this.Id
}

func (this *SubGraph) GetPort() Port {
	port, err := NewPort(nil, nil)
	if err != nil {
		panic(err)
	}
	return port
}

func (this *SubGraph) String() string {
	gName := this.Id.String()
	if strings.HasPrefix(gName, "anon") {
		gName = ""
	}
	s := "subgraph " + this.Id.String() + " {\n"
	if this.StmtList != nil {
		s += this.StmtList.String()
	}
	s += "\n}\n"
	return s
}

func (this *SubGraph) Walk(v Visitor) {
	if v == nil {
		return
	}
	v = v.Visit(this)
	this.Id.Walk(v)
	this.StmtList.Walk(v)
}

type EdgeAttrs AttrList

func NewEdgeAttrs(a Elem) (EdgeAttrs, error) {
	return EdgeAttrs(a.(AttrList)), nil
}

func (this EdgeAttrs) String() string {
	s := AttrList(this).String()
	if len(s) == 0 {
		return ""
	}
	return `edge ` + s
}

func (this EdgeAttrs) Walk(v Visitor) {
	if v == nil {
		return
	}
	v = v.Visit(this)
	for i := range this {
		this[i].Walk(v)
	}
}

type NodeAttrs AttrList

func NewNodeAttrs(a Elem) (NodeAttrs, error) {
	return NodeAttrs(a.(AttrList)), nil
}

func (this NodeAttrs) String() string {
	s := AttrList(this).String()
	if len(s) == 0 {
		return ""
	}
	return `node ` + s
}

func (this NodeAttrs) Walk(v Visitor) {
	if v == nil {
		return
	}
	v = v.Visit(this)
	for i := range this {
		this[i].Walk(v)
	}
}

type GraphAttrs AttrList

func NewGraphAttrs(a Elem) (GraphAttrs, error) {
	return GraphAttrs(a.(AttrList)), nil
}

func (this GraphAttrs) String() string {
	s := AttrList(this).String()
	if len(s) == 0 {
		return ""
	}
	return `graph ` + s
}

func (this GraphAttrs) Walk(v Visitor) {
	if v == nil {
		return
	}
	v = v.Visit(this)
	for i := range this {
		this[i].Walk(v)
	}
}

type AttrList []AList

func NewAttrList(a Elem) (AttrList, error) {
	as := make(AttrList, 0)
	if a != nil {
		as = append(as, a.(AList))
	}
	return as, nil
}

func AppendAttrList(as, a Elem) (AttrList, error) {
	this := as.(AttrList)
	if a == nil {
		return this, nil
	}
	this = append(this, a.(AList))
	return this, nil
}

func (this AttrList) String() string {
	s := ""
	for _, alist := range this {
		ss := alist.String()
		if len(ss) > 0 {
			s += "[ " + ss + " ] "
		}
	}
	if len(s) == 0 {
		return ""
	}
	return s
}

func (this AttrList) Walk(v Visitor) {
	if v == nil {
		return
	}
	v = v.Visit(this)
	for i := range this {
		this[i].Walk(v)
	}
}

func PutMap(attrmap map[string]string) AttrList {
	attrlist := make(AttrList, 1)
	attrlist[0] = make(AList, 0)
	keys := make([]string, 0, len(attrmap))
	for key := range attrmap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, name := range keys {
		value := attrmap[name]
		attrlist[0] = append(attrlist[0], &Attr{Id(name), Id(value)})
	}
	return attrlist
}

func (this AttrList) GetMap() map[string]string {
	attrs := make(map[string]string)
	for _, alist := range this {
		for _, attr := range alist {
			attrs[attr.Field.String()] = attr.Value.String()
		}
	}
	return attrs
}

type AList []*Attr

func NewAList(a Elem) (AList, error) {
	as := make(AList, 1)
	as[0] = a.(*Attr)
	return as, nil
}

func AppendAList(as, a Elem) (AList, error) {
	this := as.(AList)
	attr := a.(*Attr)
	this = append(this, attr)
	return this, nil
}

func (this AList) String() string {
	if len(this) == 0 {
		return ""
	}
	str := this[0].String()
	for i := 1; i < len(this); i++ {
		str += `, ` + this[i].String()
	}
	return str
}

func (this AList) Walk(v Visitor) {
	v = v.Visit(this)
	for i := range this {
		this[i].Walk(v)
	}
}

type Attr struct {
	Field Id
	Value Id
}

func NewAttr(f, v Elem) (*Attr, error) {
	a := &Attr{Field: f.(Id)}
	a.Value = Id("true")
	if v != nil {
		ok := false
		a.Value, ok = v.(Id)
		if !ok {
			return nil, errors.New(fmt.Sprintf("value = %v", v))
		}
	}
	return a, nil
}

func (this *Attr) String() string {
	return this.Field.String() + `=` + this.Value.String()
}

func (this *Attr) Walk(v Visitor) {
	if v == nil {
		return
	}
	v = v.Visit(this)
	this.Field.Walk(v)
	this.Value.Walk(v)
}

type Location interface {
	Elem
	Walkable
	isLocation()
	GetId() Id
	GetPort() Port
	IsNode() bool
}

func (this *NodeId) isLocation()    {}
func (this *NodeId) IsNode() bool   { return true }
func (this *SubGraph) isLocation()  {}
func (this *SubGraph) IsNode() bool { return false }

type EdgeStmt struct {
	Source  Location
	EdgeRHS EdgeRHS
	Attrs   AttrList
}

func NewEdgeStmt(id, e, attrs Elem) (*EdgeStmt, error) {
	var a AttrList = nil
	var err error = nil
	if attrs == nil {
		a, err = NewAttrList(nil)
		if err != nil {
			return nil, err
		}
	} else {
		a = attrs.(AttrList)
	}
	return &EdgeStmt{id.(Location), e.(EdgeRHS), a}, nil
}

func (this EdgeStmt) String() string {
	return strings.TrimSpace(this.Source.String() + this.EdgeRHS.String() + this.Attrs.String())
}

func (this EdgeStmt) Walk(v Visitor) {
	if v == nil {
		return
	}
	v = v.Visit(this)
	this.Source.Walk(v)
	this.EdgeRHS.Walk(v)
	this.Attrs.Walk(v)
}

type EdgeRHS []*EdgeRH

func NewEdgeRHS(op, id Elem) (EdgeRHS, error) {
	return EdgeRHS{&EdgeRH{op.(EdgeOp), id.(Location)}}, nil
}

func AppendEdgeRHS(e, op, id Elem) (EdgeRHS, error) {
	erhs := e.(EdgeRHS)
	erhs = append(erhs, &EdgeRH{op.(EdgeOp), id.(Location)})
	return erhs, nil
}

func (this EdgeRHS) String() string {
	s := ""
	for i := range this {
		s += this[i].String()
	}
	return strings.TrimSpace(s)
}

func (this EdgeRHS) Walk(v Visitor) {
	if v == nil {
		return
	}
	v = v.Visit(this)
	for i := range this {
		this[i].Walk(v)
	}
}

type EdgeRH struct {
	Op          EdgeOp
	Destination Location
}

func (this *EdgeRH) String() string {
	return strings.TrimSpace(this.Op.String() + this.Destination.String())
}

func (this *EdgeRH) Walk(v Visitor) {
	if v == nil {
		return
	}
	v = v.Visit(this)
	this.Op.Walk(v)
	this.Destination.Walk(v)
}

type NodeStmt struct {
	NodeId *NodeId
	Attrs  AttrList
}

func NewNodeStmt(id, attrs Elem) (*NodeStmt, error) {
	nid := id.(*NodeId)
	var a AttrList = nil
	var err error = nil
	if attrs == nil {
		a, err = NewAttrList(nil)
		if err != nil {
			return nil, err
		}
	} else {
		a = attrs.(AttrList)
	}
	return &NodeStmt{nid, a}, nil
}

func (this NodeStmt) String() string {
	return strings.TrimSpace(this.NodeId.String() + ` ` + this.Attrs.String())
}

func (this NodeStmt) Walk(v Visitor) {
	if v == nil {
		return
	}
	v = v.Visit(this)
	this.NodeId.Walk(v)
	this.Attrs.Walk(v)
}

type EdgeOp bool

const (
	DIRECTED   EdgeOp = true
	UNDIRECTED EdgeOp = false
)

func (this EdgeOp) String() string {
	switch this {
	case DIRECTED:
		return "->"
	case UNDIRECTED:
		return "--"
	}
	panic("unreachable")
}

func (this EdgeOp) Walk(v Visitor) {
	if v == nil {
		return
	}
	v.Visit(this)
}

type NodeId struct {
	Id   Id
	Port Port
}

func NewNodeId(id Elem, port Elem) (*NodeId, error) {
	if port == nil {
		return &NodeId{id.(Id), Port{"", ""}}, nil
	}
	return &NodeId{id.(Id), port.(Port)}, nil
}

func MakeNodeId(id string, port string) *NodeId {
	p := Port{"", ""}
	if len(port) > 0 {
		ps := strings.Split(port, ":")
		p.Id1 = Id(ps[1])
		if len(ps) > 2 {
			p.Id2 = Id(ps[2])
		}
	}
	return &NodeId{Id(id), p}
}

func (this *NodeId) String() string {
	return this.Id.String() + this.Port.String()
}

func (this *NodeId) GetId() Id {
	return this.Id
}

func (this *NodeId) GetPort() Port {
	return this.Port
}

func (this *NodeId) Walk(v Visitor) {
	if v == nil {
		return
	}
	v = v.Visit(this)
	this.Id.Walk(v)
	this.Port.Walk(v)
}

//TODO semantic analysis should decide which Id is an Id and which is a Compass Point
type Port struct {
	Id1 Id
	Id2 Id
}

func NewPort(id1, id2 Elem) (Port, error) {
	port := Port{Id(""), Id("")}
	if id1 != nil {
		port.Id1 = id1.(Id)
	}
	if id2 != nil {
		port.Id2 = id2.(Id)
	}
	return port, nil
}

func (this Port) String() string {
	if len(this.Id1) == 0 {
		return ""
	}
	s := ":" + this.Id1.String()
	if len(this.Id2) > 0 {
		s += ":" + this.Id2.String()
	}
	return s
}

func (this Port) Walk(v Visitor) {
	if v == nil {
		return
	}
	v = v.Visit(this)
	this.Id1.Walk(v)
	this.Id2.Walk(v)
}

type Id string

func NewId(id Elem) (Id, error) {
	if id == nil {
		return Id(""), nil
	}
	id_lit := string(id.(*token.Token).Lit)
	return Id(id_lit), nil
}

func (this Id) String() string {
	return string(this)
}

func (this Id) Walk(v Visitor) {
	if v == nil {
		return
	}
	v.Visit(this)
}
