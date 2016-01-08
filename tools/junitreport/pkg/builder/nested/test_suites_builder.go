package nested

import (
	"strings"

	"github.com/openshift/origin/tools/junitreport/pkg/api"
	"github.com/openshift/origin/tools/junitreport/pkg/builder"
	"github.com/openshift/origin/tools/junitreport/pkg/errors"
)

// NewTestSuitesBuilder returns a new nested test suites builder. All test suites consumed by
// this builder will be added to a multitree of suites rooted at the suites with the given names.
func NewTestSuitesBuilder(rootSuiteNames []string) builder.TestSuitesBuilder {
	rootSuites := []*api.TestSuite{}
	for _, name := range rootSuiteNames {
		rootSuites = append(rootSuites, &api.TestSuite{Name: name})
	}

	return &nestedTestSuitesBuilder{
		restrictedRoots: len(rootSuites) > 0, // i given they are the only roots allowed
		testSuites: &api.TestSuites{
			Suites: rootSuites,
		},
	}
}

const (
	// TestSuiteNameDelimiter is the default delimeter for test suite names
	TestSuiteNameDelimiter = "/"
)

// nestedTestSuitesBuilder is a test suites builder that nests suites under a root suite
type nestedTestSuitesBuilder struct {
	// restrictedRoots determines if the builder is able to add new roots to the tree or if all
	// new suits are to be added only if they are leaves of the original set of roots created
	// by the constructor
	restrictedRoots bool

	testSuites *api.TestSuites
}

// AddSuite adds a test suite to the test suites collection being built if the suite is not in
// the collection, otherwise it overwrites the current record of the suite in the collection. In
// both cases, it updates the metrics of any parent suites to include those of the new suite. If
// the parent of the test suite to be added is found in the collection, the test suite is added
// as a child of that suite. Otherwise, parent suites are created by successively removing one
// layer of package specificity until the root name is found. For instance, if the suite named
// "root/package/subpackage/subsubpackage" were to be added to an empty collection, the suites
// named "root", "root/package", and "root/package/subpackage" would be created and added first,
// then the suite could be added as a child of the latter parent package. If roots are restricted,
// then test suites to be added are asssumed to be nested under one of the root suites created by
// the constructor method and the attempted addition of a suite not rooted in those suites will
// fail silently to allow for selective tree-building given a root.
func (b *nestedTestSuitesBuilder) AddSuite(suite *api.TestSuite) error {
	if recordedSuite := b.findSuite(suite.Name); recordedSuite != nil {
		// if we are trying to add a suite that already exists, we just need to overwrite our
		// current record with the data in the new suite to be added
		recordedSuite.NumTests = suite.NumTests
		recordedSuite.NumSkipped = suite.NumSkipped
		recordedSuite.NumFailed = suite.NumFailed
		recordedSuite.Duration = suite.Duration
		recordedSuite.Properties = suite.Properties
		recordedSuite.TestCases = suite.TestCases
		recordedSuite.Children = suite.Children
		return nil
	}

	if err := b.addToParent(suite); err != nil {
		if errors.IsSuiteOutOfBoundsError(err) {
			// if we were trying to add something out of bounds, we ignore the request but do not
			// throw an error so we can selectively build sub-trees with a set of specified roots
			return nil
		}
		return err
	}

	b.updateMetrics(suite)

	return nil
}

// addToParent will find or create the parent for the test suite and add the given suite as a child
func (b *nestedTestSuitesBuilder) addToParent(child *api.TestSuite) error {
	name := child.Name
	if !b.isChildOfRoots(name) && b.restrictedRoots {
		// if we were asked to add a new test suite that isn't a child of any current root,
		// and we aren't allowed to add new roots, we can't fulfill this request
		return errors.NewSuiteOutOfBoundsError(name)
	}

	parentName := getParentName(name)
	if len(parentName) == 0 {
		// this suite does not have a parent, we just need to add it as a root
		b.testSuites.Suites = append(b.testSuites.Suites, child)
		return nil
	}

	parent := b.findSuite(parentName)
	if parent == nil {
		// no parent is currently registered, we need to create it and add it to the tree
		parent = &api.TestSuite{
			Name:     parentName,
			Children: []*api.TestSuite{child},
		}

		return b.addToParent(parent)
	}

	parent.Children = append(parent.Children, child)
	return nil
}

// getParentName returns the name of the parent package, if it exists in the multitree
func getParentName(name string) string {
	if !strings.Contains(name, TestSuiteNameDelimiter) {
		return ""
	}

	delimeterIndex := strings.LastIndex(name, TestSuiteNameDelimiter)
	return name[0:delimeterIndex]
}

func (b *nestedTestSuitesBuilder) isChildOfRoots(name string) bool {
	for _, rootSuite := range b.testSuites.Suites {
		if strings.HasPrefix(name, rootSuite.Name) {
			return true
		}
	}
	return false
}

// findSuite finds a test suite in a collection of test suites
func (b *nestedTestSuitesBuilder) findSuite(name string) *api.TestSuite {
	return findSuite(b.testSuites.Suites, name)
}

// findSuite walks a test suite tree to find a test suite with the given name
func findSuite(suites []*api.TestSuite, name string) *api.TestSuite {
	for _, suite := range suites {
		if suite.Name == name {
			return suite
		}

		if strings.HasPrefix(name, suite.Name) {
			return findSuite(suite.Children, name)
		}
	}

	return nil
}

// updateMetrics updates the metrics for all parents of a test suite
func (b *nestedTestSuitesBuilder) updateMetrics(newSuite *api.TestSuite) {
	updateMetrics(b.testSuites.Suites, newSuite)
}

// updateMetrics walks a test suite tree to update metrics of parents of the given suite
func updateMetrics(suites []*api.TestSuite, newSuite *api.TestSuite) {
	for _, suite := range suites {
		if suite.Name == newSuite.Name || !strings.HasPrefix(newSuite.Name, suite.Name) {
			// if we're considering the suite itself or another suite that is not a pure
			// prefix of the new suite, we are not considering a parent suite and therefore
			// do not need to update any metrics
			continue
		}

		suite.NumTests += newSuite.NumTests
		suite.NumSkipped += newSuite.NumSkipped
		suite.NumFailed += newSuite.NumFailed
		suite.Duration += newSuite.Duration
		// we round to the millisecond on duration
		suite.Duration = float64(int(suite.Duration*1000)) / 1000

		updateMetrics(suite.Children, newSuite)
	}
}

// Build releases the test suites collection being built at whatever current state it is in
func (b *nestedTestSuitesBuilder) Build() *api.TestSuites {
	return b.testSuites
}
