package stack

import (
	"fmt"

	"github.com/openshift/origin/tools/junitreport/pkg/api"
)

// TestSuiteStack is a data structure that holds api.TestSuite objects in a LIFO
type TestSuiteStack interface {
	// Push adds the testSuite to the top of the LIFO
	Push(pkg *api.TestSuite)
	// Pop removes the head of the LIFO and returns it
	Pop() *api.TestSuite
	// Peek returns a reference to the head of the LIFO without removing it
	Peek() *api.TestSuite
	// IsEmpty determines if the stack has any members
	IsEmpty() bool
}

// NewTestSuiteStack returns a new TestSuiteStack
func NewTestSuiteStack() TestSuiteStack {
	return &testSuiteStack{
		head: nil,
	}
}

// testSuiteStack is an implementation of a TestSuiteStack using a linked list
type testSuiteStack struct {
	head *testSuiteNode
}

// Push adds the testSuite to the top of the LIFO
func (s *testSuiteStack) Push(data *api.TestSuite) {
	newNode := &testSuiteNode{
		Member: data,
		Next:   s.head,
	}
	s.head = newNode
}

// Pop removes the head of the LIFO and returns it
func (s *testSuiteStack) Pop() *api.TestSuite {
	if s.IsEmpty() {
		return nil
	}
	oldNode := s.head
	s.head = s.head.Next
	return oldNode.Member
}

// Peek returns a reference to the head of the LIFO without removing it
func (s *testSuiteStack) Peek() *api.TestSuite {
	if s.IsEmpty() {
		return nil
	}
	return s.head.Member
}

// IsEmpty determines if the stack has any members
func (s *testSuiteStack) IsEmpty() bool {
	return s.head == nil
}

// testSuiteNode is a node in a singly-linked list
type testSuiteNode struct {
	Member *api.TestSuite
	Next   *testSuiteNode
}

func (n *testSuiteNode) String() string {
	return fmt.Sprintf("{Member: %s, Next: %s}", n.Member, n.Next.String())
}
