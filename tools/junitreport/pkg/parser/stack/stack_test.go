package stack

import (
	"reflect"
	"testing"

	"github.com/openshift/origin/tools/junitreport/pkg/api"
)

func TestPush(t *testing.T) {
	var testCases = []struct {
		name            string
		stackSeed       *testSuiteNode
		testSuiteToPush *api.TestSuite
		expectedStack   *testSuiteNode
	}{
		{
			name:            "push on empty stack",
			stackSeed:       nil,
			testSuiteToPush: newTestSuite("test"),
			expectedStack:   newTestSuiteNode(newTestSuite("test"), nil),
		},
		{
			name:            "push on existing stack",
			stackSeed:       newTestSuiteNode(newTestSuite("test"), nil),
			testSuiteToPush: newTestSuite("test2"),
			expectedStack:   newTestSuiteNode(newTestSuite("test2"), newTestSuiteNode(newTestSuite("test"), nil)),
		},
		{
			name:            "push on deep stack",
			stackSeed:       newTestSuiteNode(newTestSuite("test2"), newTestSuiteNode(newTestSuite("test3"), nil)),
			testSuiteToPush: newTestSuite("test1"),
			expectedStack:   newTestSuiteNode(newTestSuite("test1"), newTestSuiteNode(newTestSuite("test2"), newTestSuiteNode(newTestSuite("test3"), nil))),
		},
	}

	for _, testCase := range testCases {
		testStack := &testSuiteStack{
			head: testCase.stackSeed,
		}
		testStack.Push(testCase.testSuiteToPush)

		if !reflect.DeepEqual(testStack.head, testCase.expectedStack) {
			t.Errorf("%s: did not get correct stack state after push:\n\texpected:\n\t%s\n\tgot:\n\t%s\n", testCase.name, testCase.expectedStack, testStack.head)
		}
	}
}

func TestPop(t *testing.T) {
	var testCases = []struct {
		name              string
		stackSeed         *testSuiteNode
		expectedTestSuite *api.TestSuite
		expectedStack     *testSuiteNode
	}{
		{
			name:              "pop on empty stack",
			stackSeed:         nil,
			expectedTestSuite: nil,
			expectedStack:     nil,
		},
		{
			name:              "pop on existing stack",
			stackSeed:         newTestSuiteNode(newTestSuite("test"), nil),
			expectedTestSuite: newTestSuite("test"),
			expectedStack:     nil,
		},
		{
			name:              "pop on deep stack",
			stackSeed:         newTestSuiteNode(newTestSuite("test"), newTestSuiteNode(newTestSuite("test2"), nil)),
			expectedTestSuite: newTestSuite("test"),
			expectedStack:     newTestSuiteNode(newTestSuite("test2"), nil),
		},
	}

	for _, testCase := range testCases {
		testStack := &testSuiteStack{
			head: testCase.stackSeed,
		}
		testSuite := testStack.Pop()
		if !reflect.DeepEqual(testSuite, testCase.expectedTestSuite) {
			t.Errorf("%s: did not get correct package from pop:\n\texpected:\n\t%s\n\tgot:\n\t%s\n", testCase.name, testCase.expectedTestSuite, testSuite)
		}
		if !reflect.DeepEqual(testStack.head, testCase.expectedStack) {
			t.Errorf("%s: did not get correct stack state after pop:\n\texpected:\n\t%s\n\tgot:\n\t%s\n", testCase.name, testCase.expectedStack, testStack.head)
		}
	}
}

func TestPeek(t *testing.T) {
	var testCases = []struct {
		name              string
		stackSeed         *testSuiteNode
		expectedTestSuite *api.TestSuite
		expectedStack     *testSuiteNode
	}{
		{
			name:              "peek on empty stack",
			stackSeed:         nil,
			expectedTestSuite: nil,
			expectedStack:     nil,
		},
		{
			name:              "peek on existing stack",
			stackSeed:         newTestSuiteNode(newTestSuite("test"), nil),
			expectedTestSuite: newTestSuite("test"),
			expectedStack:     newTestSuiteNode(newTestSuite("test"), nil),
		},
		{
			name:              "peek on deep stack",
			stackSeed:         newTestSuiteNode(newTestSuite("test"), newTestSuiteNode(newTestSuite("test2"), nil)),
			expectedTestSuite: newTestSuite("test"),
			expectedStack:     newTestSuiteNode(newTestSuite("test"), newTestSuiteNode(newTestSuite("test2"), nil)),
		},
	}

	for _, testCase := range testCases {
		testStack := &testSuiteStack{
			head: testCase.stackSeed,
		}
		testSuite := testStack.Peek()
		if !reflect.DeepEqual(testSuite, testCase.expectedTestSuite) {
			t.Errorf("%s: did not get correct package from pop:\n\texpected:\n\t%s\n\tgot:\n\t%s\n", testCase.name, testCase.expectedTestSuite, testSuite)
		}
		if !reflect.DeepEqual(testStack.head, testCase.expectedStack) {
			t.Errorf("%s: did not get correct stack state after pop:\n\texpected:\n\t%s\n\tgot:\n\t%s\n", testCase.name, testCase.expectedStack, testStack.head)
		}
	}
}

func TestIsEmpty(t *testing.T) {
	var testCases = []struct {
		name          string
		stackSeed     *testSuiteNode
		expectedState bool
		expectedStack *testSuiteNode
	}{
		{
			name:          "isempty on empty stack",
			stackSeed:     nil,
			expectedState: true,
			expectedStack: nil,
		},
		{
			name:          "isempty on existing stack",
			stackSeed:     newTestSuiteNode(newTestSuite("test"), nil),
			expectedState: false,
			expectedStack: newTestSuiteNode(newTestSuite("test"), nil),
		},
	}

	for _, testCase := range testCases {
		testStack := &testSuiteStack{
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

func newTestSuite(name string) *api.TestSuite {
	return &api.TestSuite{
		Name: name,
	}
}

func newTestSuiteNode(testSuite *api.TestSuite, next *testSuiteNode) *testSuiteNode {
	return &testSuiteNode{
		Member: testSuite,
		Next:   next,
	}
}
