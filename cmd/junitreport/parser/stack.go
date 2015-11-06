package parser

import (
	"fmt"

	upstream "github.com/jstemmer/go-junit-report/parser"
)

// PackageStack is a data structure that holds upstream.Package objects in a LIFO
type PackageStack interface {
	// Push adds the package to the top of the LIFO
	Push(pkg *upstream.Package)
	// Pop removes the head of the LIFO and returns it
	Pop() *upstream.Package
	// Peek returns a reference to the head of the LIFO without removing it
	Peek() *upstream.Package
	// IsEmpty determines if the stack has any members
	IsEmpty() bool
}

// NewPackageStack returns a new PackageStack
func NewPackageStack() PackageStack {
	return &packageStack{
		head: nil,
	}
}

// packageStack is an implementation of a PackageStack using a linked list
type packageStack struct {
	head *packageNode
}

// Push adds the package to the top of the LIFO
func (s *packageStack) Push(pkg *upstream.Package) {
	newNode := &packageNode{
		Member: pkg,
		Next:   s.head,
	}
	s.head = newNode
}

// Pop removes the head of the LIFO and returns it
func (s *packageStack) Pop() *upstream.Package {
	if s.IsEmpty() {
		return nil
	}
	oldNode := s.head
	s.head = s.head.Next
	return oldNode.Member
}

// Peek returns a reference to the head of the LIFO without removing it
func (s *packageStack) Peek() *upstream.Package {
	if s.IsEmpty() {
		return nil
	}
	return s.head.Member
}

// IsEmpty determines if the stack has any members
func (s *packageStack) IsEmpty() bool {
	return s.head == nil
}

type packageNode struct {
	Member *upstream.Package
	Next   *packageNode
}

func (n *packageNode) String() string {
	return fmt.Sprintf("{Member: %s, Next: %s}", n.Member, n.Next)
}
