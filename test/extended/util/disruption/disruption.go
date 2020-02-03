package disruption

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	g "github.com/onsi/ginkgo"

	"k8s.io/kubernetes/test/e2e/chaosmonkey"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/ginkgowrapper"
	"k8s.io/kubernetes/test/e2e/upgrades"
	"k8s.io/kubernetes/test/utils/junit"
)

// flakeSummary is a test summary type that allows upgrades to report violations
// without failing the upgrade test.
type flakeSummary string

func (s flakeSummary) PrintHumanReadable() string { return string(s) }
func (s flakeSummary) SummaryKind() string        { return "Flake" }
func (s flakeSummary) PrintJSON() string          { return `{"type":"Flake"}` }

// Flakef records a flake on the current framework.
func Flakef(f *framework.Framework, format string, options ...interface{}) {
	framework.Logf(format, options...)
	f.TestSummaries = append(f.TestSummaries, flakeSummary(fmt.Sprintf(format, options...)))
}

// TestData is passed to the invariant tests executed during the upgrade. The default UpgradeType
// is MasterUpgrade.
type TestData struct {
	UpgradeType    upgrades.UpgradeType
	UpgradeContext upgrades.UpgradeContext
}

// Run executes the provided fn in a test context, ensuring that invariants are preserved while the
// test is being executed. Description is used to populate the JUnit suite name, and testname is
// used to define the overall test that will be run.
func Run(description, testname string, adapter TestData, invariants []upgrades.Test, fn func()) {
	testSuite := &junit.TestSuite{Name: description, Package: testname}
	test := &junit.TestCase{Name: testname, Classname: testname}
	testSuite.TestCases = append(testSuite.TestCases, test)
	cm := chaosmonkey.New(func() {
		start := time.Now()
		defer finalizeTest(start, test, nil)
		fn()
	})
	runChaosmonkey(cm, adapter, invariants, testSuite)
}

func runChaosmonkey(
	cm *chaosmonkey.Chaosmonkey,
	testData TestData,
	tests []upgrades.Test,
	testSuite *junit.TestSuite,
) {
	testFrameworks := createTestFrameworks(tests)
	for _, t := range tests {
		testCase := &junit.TestCase{
			Name:      t.Name(),
			Classname: "disruption_tests",
		}
		testSuite.TestCases = append(testSuite.TestCases, testCase)

		f, ok := testFrameworks[t.Name()]
		if !ok {
			panic(fmt.Sprintf("can't find test framework for %q", t.Name()))
		}
		cma := chaosMonkeyAdapter{
			TestData:   testData,
			framework:  f,
			test:       t,
			testReport: testCase,
		}
		cm.Register(cma.Test)
	}

	start := time.Now()
	defer func() {
		testSuite.Update()
		testSuite.Time = time.Since(start).Seconds()
		if framework.TestContext.ReportDir != "" {
			fname := filepath.Join(framework.TestContext.ReportDir, fmt.Sprintf("junit_%s_%d.xml", testSuite.Package, time.Now().Unix()))
			f, err := os.Create(fname)
			if err != nil {
				return
			}
			defer f.Close()
			xml.NewEncoder(f).Encode(testSuite)
		}
	}()
	cm.Do()
}

type chaosMonkeyAdapter struct {
	TestData

	test       upgrades.Test
	testReport *junit.TestCase
	framework  *framework.Framework
}

func (cma *chaosMonkeyAdapter) Test(sem *chaosmonkey.Semaphore) {
	start := time.Now()
	var once sync.Once
	ready := func() {
		once.Do(func() {
			sem.Ready()
		})
	}
	defer finalizeTest(start, cma.testReport, cma.framework)
	defer ready()
	if skippable, ok := cma.test.(upgrades.Skippable); ok && skippable.Skip(cma.UpgradeContext) {
		g.By("skipping test " + cma.test.Name())
		cma.testReport.Skipped = "skipping test " + cma.test.Name()
		return
	}
	fmt.Printf("DEBUG: starting test\n")
	cma.framework.BeforeEach()
	fmt.Printf("DEBUG: starting test, setup\n")
	cma.test.Setup(cma.framework)
	defer cma.test.Teardown(cma.framework)
	ready()
	cma.test.Test(cma.framework, sem.StopCh, cma.UpgradeType)
}

func finalizeTest(start time.Time, tc *junit.TestCase, f *framework.Framework) {
	tc.Time = time.Since(start).Seconds()
	r := recover()
	if r == nil {
		if f != nil {
			for _, summary := range f.TestSummaries {
				if summary.SummaryKind() == "Flake" {
					tc.Failures = append(tc.Failures, &junit.Failure{
						Message: summary.PrintHumanReadable(),
						Type:    "Failure",
						Value:   summary.PrintHumanReadable(),
					})
				}
			}
		}
		return
	}

	switch r := r.(type) {
	case ginkgowrapper.FailurePanic:
		tc.Failures = []*junit.Failure{
			{
				Message: r.Message,
				Type:    "Failure",
				Value:   fmt.Sprintf("%s\n\n%s", r.Message, r.FullStackTrace),
			},
		}
	case ginkgowrapper.SkipPanic:
		tc.Skipped = fmt.Sprintf("%s:%d %q", r.Filename, r.Line, r.Message)
	default:
		tc.Errors = []*junit.Error{
			{
				Message: fmt.Sprintf("%v", r),
				Type:    "Panic",
				Value:   fmt.Sprintf("%v\n\n%s", r, debug.Stack()),
			},
		}
	}
}

// TODO: accept a default framework
func createTestFrameworks(tests []upgrades.Test) map[string]*framework.Framework {
	nsFilter := regexp.MustCompile("[^[:word:]-]+") // match anything that's not a word character or hyphen
	testFrameworks := map[string]*framework.Framework{}
	for _, t := range tests {
		ns := nsFilter.ReplaceAllString(t.Name(), "-") // and replace with a single hyphen
		ns = strings.Trim(ns, "-")
		// identify tests that come from kube as strictly e2e tests so they get the correct semantics
		if strings.Contains(reflect.ValueOf(t).Elem().Type().PkgPath(), "/kubernetes/test/e2e/") {
			ns = "e2e-k8s-" + ns
		}
		testFrameworks[t.Name()] = &framework.Framework{
			BaseName:                 ns,
			AddonResourceConstraints: make(map[string]framework.ResourceConstraint),
			Options: framework.Options{
				ClientQPS:   20,
				ClientBurst: 50,
			},
		}
	}
	return testFrameworks
}
