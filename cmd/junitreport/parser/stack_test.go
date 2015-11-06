package parser

import (
	"reflect"
	"testing"

	upstream "github.com/jstemmer/go-junit-report/parser"
)

func TestPush(t *testing.T) {
	var testCases = []struct {
		name          string
		stackSeed     *packageNode
		packageToPush *upstream.Package
		expectedStack *packageNode
	}{
		{
			name:          "push on empty stack",
			stackSeed:     nil,
			packageToPush: newPackage("test"),
			expectedStack: newPackageNode(newPackage("test"), nil),
		},
		{
			name:          "push on existing stack",
			stackSeed:     newPackageNode(newPackage("test"), nil),
			packageToPush: newPackage("test2"),
			expectedStack: newPackageNode(newPackage("test2"), newPackageNode(newPackage("test"), nil)),
		},
		{
			name:          "push on deep stack",
			stackSeed:     newPackageNode(newPackage("test2"), newPackageNode(newPackage("test3"), nil)),
			packageToPush: newPackage("test1"),
			expectedStack: newPackageNode(newPackage("test1"), newPackageNode(newPackage("test2"), newPackageNode(newPackage("test3"), nil))),
		},
	}

	for _, testCase := range testCases {
		testStack := &packageStack{
			head: testCase.stackSeed,
		}
		testStack.Push(testCase.packageToPush)

		if !reflect.DeepEqual(testStack.head, testCase.expectedStack) {
			t.Errorf("%s: did not get correct stack state after push:\n\texpected:\n\t%s\n\tgot:\n\t%s\n", testCase.name, testCase.expectedStack, testStack.head)
		}
	}
}

func TestPop(t *testing.T) {
	var testCases = []struct {
		name            string
		stackSeed       *packageNode
		expectedPackage *upstream.Package
		expectedStack   *packageNode
	}{
		{
			name:            "pop on empty stack",
			stackSeed:       nil,
			expectedPackage: nil,
			expectedStack:   nil,
		},
		{
			name:            "pop on existing stack",
			stackSeed:       newPackageNode(newPackage("test"), nil),
			expectedPackage: newPackage("test"),
			expectedStack:   nil,
		},
		{
			name:            "pop on deep stack",
			stackSeed:       newPackageNode(newPackage("test"), newPackageNode(newPackage("test2"), nil)),
			expectedPackage: newPackage("test"),
			expectedStack:   newPackageNode(newPackage("test2"), nil),
		},
	}

	for _, testCase := range testCases {
		testStack := &packageStack{
			head: testCase.stackSeed,
		}
		pkg := testStack.Pop()
		if !reflect.DeepEqual(pkg, testCase.expectedPackage) {
			t.Errorf("%s: did not get correct package from pop:\n\texpected:\n\t%s\n\tgot:\n\t%s\n", testCase.name, testCase.expectedPackage, pkg)
		}
		if !reflect.DeepEqual(testStack.head, testCase.expectedStack) {
			t.Errorf("%s: did not get correct stack state after pop:\n\texpected:\n\t%s\n\tgot:\n\t%s\n", testCase.name, testCase.expectedStack, testStack.head)
		}
	}
}

func TestPeek(t *testing.T) {
	var testCases = []struct {
		name            string
		stackSeed       *packageNode
		expectedPackage *upstream.Package
		expectedStack   *packageNode
	}{
		{
			name:            "peek on empty stack",
			stackSeed:       nil,
			expectedPackage: nil,
			expectedStack:   nil,
		},
		{
			name:            "peek on existing stack",
			stackSeed:       newPackageNode(newPackage("test"), nil),
			expectedPackage: newPackage("test"),
			expectedStack:   newPackageNode(newPackage("test"), nil),
		},
		{
			name:            "peek on deep stack",
			stackSeed:       newPackageNode(newPackage("test"), newPackageNode(newPackage("test2"), nil)),
			expectedPackage: newPackage("test"),
			expectedStack:   newPackageNode(newPackage("test"), newPackageNode(newPackage("test2"), nil)),
		},
	}

	for _, testCase := range testCases {
		testStack := &packageStack{
			head: testCase.stackSeed,
		}
		pkg := testStack.Peek()
		if !reflect.DeepEqual(pkg, testCase.expectedPackage) {
			t.Errorf("%s: did not get correct package from pop:\n\texpected:\n\t%s\n\tgot:\n\t%s\n", testCase.name, testCase.expectedPackage, pkg)
		}
		if !reflect.DeepEqual(testStack.head, testCase.expectedStack) {
			t.Errorf("%s: did not get correct stack state after pop:\n\texpected:\n\t%s\n\tgot:\n\t%s\n", testCase.name, testCase.expectedStack, testStack.head)
		}
	}
}

func TestIsEmpty(t *testing.T) {
	var testCases = []struct {
		name          string
		stackSeed     *packageNode
		expectedState bool
		expectedStack *packageNode
	}{
		{
			name:          "isempty on empty stack",
			stackSeed:     nil,
			expectedState: true,
			expectedStack: nil,
		},
		{
			name:          "isempty on existing stack",
			stackSeed:     newPackageNode(newPackage("test"), nil),
			expectedState: false,
			expectedStack: newPackageNode(newPackage("test"), nil),
		},
	}

	for _, testCase := range testCases {
		testStack := &packageStack{
			head: testCase.stackSeed,
		}
		state := testStack.IsEmpty()

		if state != testCase.expectedState {
			t.Errorf("%s: did not get correct stack emptiness after push: expected: %t got: %t\n", testCase.name, testCase.expectedState, state)
		}

		if !reflect.DeepEqual(testStack.head, testCase.expectedStack) {
			t.Errorf("%s: did not get correct stack state after push:\n\texpected:\n\t%s\n\tgot:\n\t%s\n", testCase.name, testCase.expectedStack, testStack.head)
		}
	}
}

func newPackage(name string) *upstream.Package {
	return &upstream.Package{
		Name: name,
	}
}

func newPackageNode(pkg *upstream.Package, next *packageNode) *packageNode {
	return &packageNode{
		Member: pkg,
		Next:   next,
	}
}
