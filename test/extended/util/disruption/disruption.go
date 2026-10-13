package disruption

// TODO: this testing framework is used by many upgrade tests beyond disruption, the package is somewhat misleading
// and it should probably be relocated.

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"

	"k8s.io/kubernetes/test/e2e/chaosmonkey"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
	admissionapi "k8s.io/pod-security-admission/api"

	"github.com/openshift/origin/pkg/riskanalysis"
	"github.com/openshift/origin/pkg/test"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

const (
	// DefaultAllowedDisruption is a constant used when we cannot calculate an allowed disruption value from historical data.
	// It is used to search for our inability to do so across CI broadly.
	DefaultAllowedDisruption = 2718
)

// testWithDisplayName is implemented by tests that want more descriptive test names
// than Name() (which must be namespace safe) allows.
type testWithDisplayName interface {
	DisplayName() string
}

// additionalTest is a test summary type that allows disruption suites to report
// extra JUnit outcomes for parts of a test.
type additionalTest struct {
	Name     string
	Failure  string
	Duration time.Duration
}

func (s additionalTest) PrintHumanReadable() string { return fmt.Sprintf("%s: %s", s.Name, s.Failure) }
func (s additionalTest) SummaryKind() string        { return "AdditionalTest" }
func (s additionalTest) PrintJSON() string          { data, _ := json.Marshal(s); return string(data) }

// flakeSummary is a test summary type that allows upgrades to report violations
// without failing the upgrade test.
type flakeSummary string

func (s flakeSummary) PrintHumanReadable() string { return string(s) }
func (s flakeSummary) SummaryKind() string        { return "Flake" }
func (s flakeSummary) PrintJSON() string          { return `{"type":"Flake"}` }

// TestData is passed to the invariant tests executed during the upgrade. The default UpgradeType
// is MasterUpgrade.
type TestData struct {
	UpgradeType    upgrades.UpgradeType
	UpgradeContext upgrades.UpgradeContext
}

// Run executes the provided fn in a test context, ensuring that invariants are preserved while the
// test is being executed. Description is used to populate the JUnit suite name, and testname is
// used to define the overall test that will be run.
func Run(f *framework.Framework, description, testname string, adapter TestData, invariants []upgrades.Test, fn func()) {
	testSuite := &junitapi.JUnitTestSuite{Name: description}

	// Ensure colors aren't emitted by chaos monkey tests
	_, reporterConfig := g.GinkgoConfiguration()
	reporterConfig.NoColor = true
	g.SetReporterConfig(reporterConfig)

	cm := chaosmonkey.New(func(ctx context.Context) {
		start := time.Now()
		defer finalizeTest(start, testname, testname, testSuite, f)
		defer g.GinkgoRecover()
		fn()
	})
	runChaosmonkey(cm, adapter, invariants, testSuite, testname)
}

func runChaosmonkey(
	cm *chaosmonkey.Chaosmonkey,
	testData TestData,
	tests []upgrades.Test,
	testSuite *junitapi.JUnitTestSuite,
	packageName string,
) {
	testFrameworks := createTestFrameworks(tests)
	for _, t := range tests {
		displayName := t.Name()
		if dn, ok := t.(testWithDisplayName); ok {
			displayName = dn.DisplayName()
		}

		f, ok := testFrameworks[t.Name()]
		if !ok {
			panic(fmt.Sprintf("can't find test framework for %q", t.Name()))
		}
		cma := chaosMonkeyAdapter{
			TestData:        testData,
			framework:       f,
			test:            t,
			testName:        displayName,
			className:       "disruption_tests",
			testSuiteReport: testSuite,
		}
		cm.Register(cma.Test)
	}

	start := time.Now()
	defer func() {

		// Calculate NumFailed and NumSkipped
		for _, tc := range testSuite.TestCases {
			testSuite.NumTests++
			if tc.FailureOutput != nil {
				testSuite.NumFailed++
			}
			if tc.SkipMessage != nil {
				testSuite.NumSkipped++
			}
		}

		testSuite.Duration = time.Since(start).Seconds()

		if framework.TestContext.ReportDir != "" {
			timeSuffix := fmt.Sprintf("_%s", time.Now().UTC().Format("20060102-150405"))

			fname := filepath.Join(framework.TestContext.ReportDir, fmt.Sprintf("junit_%s_%s.xml", packageName, timeSuffix))
			f, err := os.Create(fname)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: Failed to create file %v: %v\n", fname, err)
				return
			}
			defer f.Close()
			out, err := xml.MarshalIndent(testSuite, "", "    ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: Failed to marshal junit: %v\n", err)
				return
			}
			_, err = f.Write(test.StripANSI(out))
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: Failed to write file %v: %v\n", fname, err)
			}

			// default for wasMasterNodeUpdated is empty string as that is what entries prior to adding this will have
			// we don't currently need this field set for RiskAnalysis.  We could change this logic to either
			// parse the events for the NodeUpdated interval or read the ClusterData.json from storage
			// and pass it in if needed.
			if err := riskanalysis.WriteJobRunTestFailureSummary(framework.TestContext.ReportDir, timeSuffix, testSuite, "", ""); err != nil {
				fmt.Fprintf(os.Stderr, "error: Failed to write file %v: %v\n", fname, err)
				return
			}
		}
	}()
	cm.Do(context.Background())
}

type chaosMonkeyAdapter struct {
	TestData

	test            upgrades.Test
	testName        string
	className       string
	testSuiteReport *junitapi.JUnitTestSuite
	framework       *framework.Framework
}

func (cma *chaosMonkeyAdapter) Test(ctx context.Context, sem *chaosmonkey.Semaphore) {
	start := time.Now()
	var once sync.Once
	ready := func() {
		once.Do(func() {
			sem.Ready()
		})
	}
	defer finalizeTest(start, cma.testName, cma.className, cma.testSuiteReport, cma.framework)
	defer ready()
	if skippable, ok := cma.test.(upgrades.Skippable); ok && skippable.Skip(cma.UpgradeContext) {
		g.By("skipping test " + cma.test.Name())
		testResult := &junitapi.JUnitTestCase{
			Name:      cma.testName,
			Classname: cma.className,
		}
		testResult.SkipMessage = &junitapi.SkipMessage{Message: "skipping test " + cma.test.Name()}
		cma.testSuiteReport.TestCases = append(cma.testSuiteReport.TestCases, testResult)
		return
	}
	cma.framework.BeforeEach(ctx)
	cma.test.Setup(ctx, cma.framework)
	defer cma.test.Teardown(ctx, cma.framework)
	ready()
	cma.test.Test(ctx, cma.framework, sem.StopCh, cma.UpgradeType)
}

func finalizeTest(start time.Time, testName, className string, ts *junitapi.JUnitTestSuite, f *framework.Framework) {
	now := time.Now().UTC()
	testDuration := now.Sub(start).Seconds()

	// r is the primary means we are informed of test results here. We expect ginko to panic on any failure so
	// if r is nil, we passed. If not, we start checking if this was a flake or fail below.
	r := recover()

	// if the framework contains additional test results, add them to the parent suite or write them to disk
	for _, summary := range f.TestSummaries {

		if test, ok := summary.(additionalTest); ok {
			tc := &junitapi.JUnitTestCase{
				Name:      test.Name,
				Classname: className,
				Duration:  test.Duration.Seconds(),
			}
			if len(test.Failure) > 0 {
				tc.FailureOutput = &junitapi.FailureOutput{Output: test.Failure}
			}
			ts.TestCases = append(ts.TestCases, tc)
			continue
		}

		// TODO: this is writing out Flake_[testname]_[timestamp].json files with content that is just: {"type":"Flake"}
		// Find out if these are used by anything, but it looks like we should find a way to silence TestSummaries
		// of type flakeSummary.
		filePath := filepath.Join(framework.TestContext.ReportDir, fmt.Sprintf("%s_%s_%s.json", summary.SummaryKind(), filesystemSafeName(testName), now.Format(time.RFC3339)))
		if err := ioutil.WriteFile(filePath, []byte(summary.PrintJSON()), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error: Failed to write file %v with test data: %v\n", filePath, err)
		}
	}

	if r == nil {
		// Test can be considered successful, but may have flaked, see below:
		ts.TestCases = append(ts.TestCases, &junitapi.JUnitTestCase{
			Name:      testName,
			Classname: className,
			Duration:  testDuration,
		})
		if f != nil {
			if message, hasFlake := hasFrameworkFlake(f); hasFlake {
				// Add another test case with the failure, this is a flake as we already added the success testcase above:
				ts.TestCases = append(ts.TestCases, &junitapi.JUnitTestCase{
					Name:          testName,
					Classname:     className,
					Duration:      testDuration,
					FailureOutput: &junitapi.FailureOutput{Output: message},
				})
			}
		}
		return
	}
	framework.Logf("recover: %v", r)

	testResult := &junitapi.JUnitTestCase{
		Name:      testName,
		Classname: className,
		Duration:  testDuration,
	}
	testResult.FailureOutput = &junitapi.FailureOutput{Output: fmt.Sprintf("%v\n\n%s", r, debug.Stack())}
	ts.TestCases = append(ts.TestCases, testResult)

	// if we have a panic, but it hasn't been recorded by ginkgo, panic now
	if !g.CurrentSpecReport().Failed() {
		framework.Logf("%q: panic: %v", testName, r)
		func() {
			defer g.GinkgoRecover()
			panic(r)
		}()
	}
}

var (
	reFilesystemSafe      = regexp.MustCompile(`[^a-zA-Z1-9_]`)
	reFilesystemDuplicate = regexp.MustCompile(`_+`)
)

func filesystemSafeName(s string) string {
	s = reFilesystemSafe.ReplaceAllString(s, "_")
	return reFilesystemDuplicate.ReplaceAllString(s, "_")
}

// isGoModulePath returns true if the packagePath reported by reflection is within a
// module and given module path. When go mod is in use, module and modulePath are not
// contiguous as they were in older golang versions with vendoring, so naive contains
// tests fail.
//
// historically: ".../vendor/k8s.io/kubernetes/test/e2e"
// go.mod:       "k8s.io/kubernetes@0.18.4/test/e2e"
func isGoModulePath(packagePath, module, modulePath string) bool {
	return regexp.MustCompile(fmt.Sprintf(`\b%s(@[^/]*|)/%s\b`, regexp.QuoteMeta(module), regexp.QuoteMeta(modulePath))).MatchString(packagePath)
}

// TODO: accept a default framework
func createTestFrameworks(tests []upgrades.Test) map[string]*framework.Framework {
	nsFilter := regexp.MustCompile("[^[:word:]-]+") // match anything that's not a word character or hyphen
	testFrameworks := map[string]*framework.Framework{}
	for _, t := range tests {
		ns := nsFilter.ReplaceAllString(t.Name(), "-") // and replace with a single hyphen
		ns = strings.Trim(ns, "-")
		// identify tests that come from kube as strictly e2e tests so they get the correct semantics,
		// which includes privileged namespace access, like all the other k8s e2e-s
		if isGoModulePath(reflect.ValueOf(t).Elem().Type().PkgPath(), "k8s.io/kubernetes", "test/e2e") {
			ns = "e2e-k8s-" + ns
		}

		testFrameworks[t.Name()] = &framework.Framework{
			BaseName: ns,
			Options: framework.Options{
				ClientQPS:   20,
				ClientBurst: 50,
			},
			Timeouts: framework.NewTimeoutContext(),
			// This is similar to https://github.com/kubernetes/kubernetes/blob/f33ca2306548719e5116b53fccfc278bffb809a8/test/e2e/upgrades/upgrade_suite.go#L106,
			// where centrally all upgrade tests are being instantiated.
			NamespacePodSecurityLevel: admissionapi.LevelPrivileged,
		}
	}
	return testFrameworks
}

// FrameworkFlakef records a flake on the current framework.
func FrameworkFlakef(f *framework.Framework, format string, options ...interface{}) {
	framework.Logf(format, options...)
	f.TestSummaries = append(f.TestSummaries, flakeSummary(fmt.Sprintf(format, options...)))
}

// hasFrameworkFlake returns true if the framework recorded a flake message generated by
// Flakef during the test run.
func hasFrameworkFlake(f *framework.Framework) (string, bool) {
	for _, summary := range f.TestSummaries {
		s, ok := summary.(flakeSummary)
		if !ok {
			continue
		}
		return string(s), true
	}
	return "", false
}

// RecordJUnit will capture the result of invoking fn as either a passing or failing JUnit test
// that will be recorded alongside the current test with name. These methods only work in the
// context of a disruption test suite today and will not be reported as JUnit failures when
// used within normal ginkgo suites.
func RecordJUnit(f *framework.Framework, name string, fn func() (err error, flake bool)) error {
	start := time.Now()
	err, flake := fn()
	duration := time.Now().Sub(start)
	var failure string
	if err != nil {
		failure = err.Error()
	}
	f.TestSummaries = append(f.TestSummaries, additionalTest{
		Name:     name,
		Duration: duration,
		Failure:  failure,
	})
	if flake {
		// Append an additional result with empty failure to trigger a flake.
		f.TestSummaries = append(f.TestSummaries, additionalTest{
			Name:     name,
			Duration: duration,
		})
		return nil
	}
	return err
}

// RecordJUnitResult will output a junit result within a disruption test with the given name,
// duration, and failure string. If the failure string is set, the test is considered to have
// failed, otherwise the test is considered to have passed. These methods only work in the
// context of a disruption test suite today and will not be reported as JUnit failures when
// used within normal ginkgo suties.
func RecordJUnitResult(f *framework.Framework, name string, duration time.Duration, failure string) {
	f.TestSummaries = append(f.TestSummaries, additionalTest{
		Name:     name,
		Duration: duration,
		Failure:  failure,
	})
}
