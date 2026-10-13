package testsuites

import (
	"testing"

	"github.com/openshift/origin/pkg/test/ginkgo"
)

func TestMergeParentQualifiers(t *testing.T) {
	t.Run("child qualifiers merge into parent", func(t *testing.T) {
		suites := []*ginkgo.TestSuite{
			{Name: "parent"},
			{Name: "child", Parents: []string{"parent"}, Qualifiers: []string{"q1", "q2"}},
		}
		mergeParentQualifiers(suites)
		assertQualifiers(t, suites, "parent", []string{"q1", "q2"})
	})

	t.Run("multiple children merge into same parent", func(t *testing.T) {
		suites := []*ginkgo.TestSuite{
			{Name: "parent"},
			{Name: "child-a", Parents: []string{"parent"}, Qualifiers: []string{"qa"}},
			{Name: "child-b", Parents: []string{"parent"}, Qualifiers: []string{"qb"}},
		}
		mergeParentQualifiers(suites)
		assertQualifiers(t, suites, "parent", []string{"qa", "qb"})
	})

	t.Run("duplicate qualifiers are not added twice", func(t *testing.T) {
		suites := []*ginkgo.TestSuite{
			{Name: "parent", Qualifiers: []string{"shared"}},
			{Name: "child", Parents: []string{"parent"}, Qualifiers: []string{"shared", "unique"}},
		}
		mergeParentQualifiers(suites)
		assertQualifiers(t, suites, "parent", []string{"shared", "unique"})
	})

	t.Run("transitive parents propagate grandchild to grandparent", func(t *testing.T) {
		suites := []*ginkgo.TestSuite{
			{Name: "grandparent"},
			{Name: "parent", Parents: []string{"grandparent"}, Qualifiers: []string{"qp"}},
			{Name: "child", Parents: []string{"parent"}, Qualifiers: []string{"qc"}},
		}
		mergeParentQualifiers(suites)
		assertQualifiers(t, suites, "parent", []string{"qp", "qc"})
		assertQualifiers(t, suites, "grandparent", []string{"qp", "qc"})
	})

	t.Run("transitive parents work regardless of slice order", func(t *testing.T) {
		suites := []*ginkgo.TestSuite{
			{Name: "child", Parents: []string{"parent"}, Qualifiers: []string{"qc"}},
			{Name: "grandparent"},
			{Name: "parent", Parents: []string{"grandparent"}, Qualifiers: []string{"qp"}},
		}
		mergeParentQualifiers(suites)
		assertQualifiers(t, suites, "parent", []string{"qp", "qc"})
		assertQualifiers(t, suites, "grandparent", []string{"qp", "qc"})
	})

	t.Run("four levels deep propagates to root", func(t *testing.T) {
		suites := []*ginkgo.TestSuite{
			{Name: "root"},
			{Name: "l1", Parents: []string{"root"}, Qualifiers: []string{"q1"}},
			{Name: "l2", Parents: []string{"l1"}, Qualifiers: []string{"q2"}},
			{Name: "l3", Parents: []string{"l2"}, Qualifiers: []string{"q3"}},
		}
		mergeParentQualifiers(suites)
		assertQualifiers(t, suites, "l2", []string{"q2", "q3"})
		assertQualifiers(t, suites, "l1", []string{"q1", "q2", "q3"})
		assertQualifiers(t, suites, "root", []string{"q1", "q2", "q3"})
	})

	t.Run("child with multiple parents merges into all", func(t *testing.T) {
		suites := []*ginkgo.TestSuite{
			{Name: "parent-a"},
			{Name: "parent-b"},
			{Name: "child", Parents: []string{"parent-a", "parent-b"}, Qualifiers: []string{"qc"}},
		}
		mergeParentQualifiers(suites)
		assertQualifiers(t, suites, "parent-a", []string{"qc"})
		assertQualifiers(t, suites, "parent-b", []string{"qc"})
	})

	t.Run("child qualifiers are unchanged", func(t *testing.T) {
		suites := []*ginkgo.TestSuite{
			{Name: "parent"},
			{Name: "child", Parents: []string{"parent"}, Qualifiers: []string{"qc"}},
		}
		mergeParentQualifiers(suites)
		assertQualifiers(t, suites, "child", []string{"qc"})
	})

	t.Run("suite with no parents is unaffected", func(t *testing.T) {
		suites := []*ginkgo.TestSuite{
			{Name: "standalone", Qualifiers: []string{"qs"}},
			{Name: "other", Qualifiers: []string{"qo"}},
		}
		mergeParentQualifiers(suites)
		assertQualifiers(t, suites, "standalone", []string{"qs"})
		assertQualifiers(t, suites, "other", []string{"qo"})
	})

	t.Run("parent with no children keeps its own qualifiers", func(t *testing.T) {
		suites := []*ginkgo.TestSuite{
			{Name: "parent", Qualifiers: []string{"qp"}},
		}
		mergeParentQualifiers(suites)
		assertQualifiers(t, suites, "parent", []string{"qp"})
	})
}

func TestConformanceInheritsFromChildren(t *testing.T) {
	suites := InternalTestSuites()

	var conformance, parallel, serial *ginkgo.TestSuite
	for _, s := range suites {
		switch s.Name {
		case "openshift/conformance":
			conformance = s
		case "openshift/conformance/parallel":
			parallel = s
		case "openshift/conformance/serial":
			serial = s
		}
	}

	if conformance == nil || parallel == nil || serial == nil {
		t.Fatal("expected to find conformance, parallel, and serial suites")
	}

	if len(parallel.Qualifiers) == 0 {
		t.Fatal("parallel suite has no qualifiers")
	}
	if len(serial.Qualifiers) == 0 {
		t.Fatal("serial suite has no qualifiers")
	}

	qualifierSet := make(map[string]bool, len(conformance.Qualifiers))
	for _, q := range conformance.Qualifiers {
		qualifierSet[q] = true
	}

	for _, q := range parallel.Qualifiers {
		if !qualifierSet[q] {
			t.Errorf("conformance suite missing qualifier from parallel: %s", q)
		}
	}
	for _, q := range serial.Qualifiers {
		if !qualifierSet[q] {
			t.Errorf("conformance suite missing qualifier from serial: %s", q)
		}
	}
}

func findSuite(suites []*ginkgo.TestSuite, name string) *ginkgo.TestSuite {
	for _, s := range suites {
		if s.Name == name {
			return s
		}
	}
	return nil
}

func assertQualifiers(t *testing.T, suites []*ginkgo.TestSuite, suiteName string, expected []string) {
	t.Helper()
	suite := findSuite(suites, suiteName)
	if suite == nil {
		t.Fatalf("suite %q not found", suiteName)
	}
	if len(suite.Qualifiers) != len(expected) {
		t.Errorf("suite %q: expected %d qualifiers %v, got %d: %v",
			suiteName, len(expected), expected, len(suite.Qualifiers), suite.Qualifiers)
		return
	}
	for i, q := range expected {
		if suite.Qualifiers[i] != q {
			t.Errorf("suite %q qualifier[%d]: expected %q, got %q",
				suiteName, i, q, suite.Qualifiers[i])
		}
	}
}
