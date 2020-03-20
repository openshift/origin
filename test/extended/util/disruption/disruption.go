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

	"github.com/openshift/origin/pkg/monitor"
)

// testWithDisplayName is implemented by tests that want more descriptive test names
// than Name() (which must be namespace safe) allows.
type testWithDisplayName interface {
	DisplayName() string
}

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
		defer g.GinkgoRecover()
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
		displayName := t.Name()
		if dn, ok := t.(testWithDisplayName); ok {
			displayName = dn.DisplayName()
		}
		testCase := &junit.TestCase{
			Name:      displayName,
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
	cma.framework.BeforeEach()
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
	framework.Logf("recover: %v", r)

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
	// if we have a panic but it hasn't been recorded by ginkgo, panic now
	if !g.CurrentGinkgoTestDescription().Failed {
		framework.Logf("%q: panic: %v", tc.Name, r)
		func() {
			defer g.GinkgoRecover()
			panic(r)
		}()
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

// ExpectNoDisruption fails if the sum of the duration of all events exceeds tolerate as a fraction ([0-1]) of total, reports a
// disruption flake if any disruption occurs, and uses reason to prefix the message. I.e. tolerate 0.1 of 10m total will fail
// if the sum of the intervals is greater than 1m, or report a flake if any interval is found.
func ExpectNoDisruption(f *framework.Framework, tolerate float64, total time.Duration, events monitor.EventIntervals, reason string) {
	var duration time.Duration
	var describe []string
	for _, interval := range events {
		describe = append(describe, interval.String())
		i := interval.To.Sub(interval.From)
		if i < time.Second {
			i = time.Second
		}
		if interval.Condition.Level > monitor.Info {
			duration += i
		}
	}
	if percent := float64(duration) / float64(total); percent > tolerate {
		framework.Failf("%s for at least %s of %s (%0.0f%%):\n\n%s", reason, duration.Truncate(time.Second), total.Truncate(time.Second), percent*100, strings.Join(describe, "\n"))
	} else if duration > 0 {
		Flakef(f, "%s for at least %s of %s (%0.0f%%):\n\n%s", reason, duration.Truncate(time.Second), total.Truncate(time.Second), percent*100, strings.Join(describe, "\n"))
	}
}
