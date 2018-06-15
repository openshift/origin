package nested

import (
	"sort"
	"strings"

	"github.com/openshift/origin/tools/junitreport/pkg/api"
	"github.com/openshift/origin/tools/junitreport/pkg/builder"
)

// NewTestSuitesBuilder returns a new nested test suites builder. All test suites consumed by
// this builder will be added to a multitree of suites rooted at the suites with the given names.
func NewTestSuitesBuilder(rootSuiteNames []string) builder.TestSuitesBuilder {
	restrictedRoots := []*treeNode{}
	nodes := map[string]*treeNode{}
	for _, name := range rootSuiteNames {
		root := &treeNode{suite: &api.TestSuite{Name: name}}
		restrictedRoots = append(restrictedRoots, root)
		nodes[name] = root
	}

	return &nestedTestSuitesBuilder{
		restrictedRoots: restrictedRoots,
		nodes:           nodes,
	}
}

const (
	// TestSuiteNameDelimiter is the default delimeter for test suite names
	TestSuiteNameDelimiter = "/"
)

// nestedTestSuitesBuilder is a test suites builder that nests suites under a root suite
type nestedTestSuitesBuilder struct {
	// restrictedRoots is the original set of roots created by the constructor, and is populated only
	// if the builder is not able to add new roots to the tree, instead needing to add all nodes as
	// children of these restricted roots
	restrictedRoots []*treeNode

	nodes map[string]*treeNode
}

type treeNode struct {
	// suite is the test suite in this node
	suite *api.TestSuite

	// children are child nodes in the tree. this field will remain empty until the tree is built
	children []*treeNode

	// parent is the parent of this node. this field can be null
	parent *treeNode
}

// AddSuite adds a suite, encapsulated in a treeNode, to the list of suites that this builder cares about.
// If a suite already exists with the same name as that which is being added, the existing record is over-
// written. If roots are restricted, then test suites to be added are asssumed to be nested under one of
// the root suites created by the constructor method and the attempted addition of a suite not rooted in
// those suites will fail silently to allow for selective tree-building given a root.
func (b *nestedTestSuitesBuilder) AddSuite(suite *api.TestSuite) {
	if !allowedToCreate(suite.Name, b.restrictedRoots) {
		return
	}

	oldVersion, exists := b.nodes[suite.Name]
	if exists {
		oldVersion.suite = suite
		return
	}

	b.nodes[suite.Name] = &treeNode{suite: suite}
}

// allowedToCreate determines if the given name is allowed to be created in light of the restricted roots
func allowedToCreate(name string, restrictedRoots []*treeNode) bool {
	if len(restrictedRoots) == 0 {
		return true
	}

	for _, root := range restrictedRoots {
		if strings.HasPrefix(name, root.suite.Name) {
			return true
		}
	}

	return false
}

// Build builds an api.TestSuites from the list of nodes that is contained in the builder.
func (b *nestedTestSuitesBuilder) Build() *api.TestSuites {
	// build a tree from our list of nodes
	nodesToAdd := []*treeNode{}
	for _, node := range b.nodes {
		// make a copy of which nodes we're interested in, otherwise we'll be concurrently modifying b.nodes
		nodesToAdd = append(nodesToAdd, node)
	}

	for _, node := range nodesToAdd {
		parentNode, exists := b.nodes[getParentName(node.suite.Name)]
		if !exists {
			makeParentsFor(node, b.nodes, b.restrictedRoots)
			continue
		}

		parentNode.children = append(parentNode.children, node)
		node.parent = parentNode
	}

	// find the tree's roots
	roots := []*treeNode{}
	for _, node := range b.nodes {
		if node.parent == nil {
			roots = append(roots, node)
		}
	}

	// update all metrics inside of test suites so they encompass those of their children
	rootSuites := []*api.TestSuite{}
	for _, root := range roots {
		updateMetrics(root)
		rootSuites = append(rootSuites, root.suite)
	}

	// we need to sort our children so that we can ensure reproducible output for testing
	sort.Sort(api.ByName(rootSuites))

	return &api.TestSuites{Suites: rootSuites}
}

// makeParentsFor recursively creates parents for the child node until a parent is created that doesn't
// contain the delimiter in its name or a restricted root is reached.
func makeParentsFor(child *treeNode, register map[string]*treeNode, restrictedRoots []*treeNode) {
	parentName := getParentName(child.suite.Name)
	if parentName == "" {
		// if there is no parent for this child, we give up
		return
	}

	if parentNode, exists := register[parentName]; exists {
		// if the parent we're trying to add already exists, just use it
		parentNode.children = append(parentNode.children, child)
		child.parent = parentNode
		return
	}

	if !allowedToCreate(parentName, restrictedRoots) {
		// if the parent we're trying to create doesn't exist but we can't make it, give up
		return
	}

	parentNode := &treeNode{
		suite:    &api.TestSuite{Name: parentName},
		children: []*treeNode{child},
	}
	child.parent = parentNode
	register[parentName] = parentNode

	makeParentsFor(parentNode, register, restrictedRoots)
}

// getParentName returns the name of the parent package, regardless of if it exists in the multitree
func getParentName(name string) string {
	if !strings.Contains(name, TestSuiteNameDelimiter) {
		return ""
	}

	delimeterIndex := strings.LastIndex(name, TestSuiteNameDelimiter)
	return name[0:delimeterIndex]
}

// updateMetrics recursively updates all fields in a treeNode's TestSuite
func updateMetrics(root *treeNode) {
	for _, child := range root.children {
		updateMetrics(child)
		// we should be building a tree, so updates on children are independent and we can bring
		// in the updated data for this child right away
		root.suite.NumTests += child.suite.NumTests
		root.suite.NumSkipped += child.suite.NumSkipped
		root.suite.NumFailed += child.suite.NumFailed
		root.suite.Duration += child.suite.Duration
		root.suite.Children = append(root.suite.Children, child.suite)
	}

	// we need to sort our children so that we can ensure reproducible output for testing
	sort.Sort(api.ByName(root.suite.Children))
}
