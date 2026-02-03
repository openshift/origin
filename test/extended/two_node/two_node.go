package two_node

import (
	"fmt"
	"sort"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	"github.com/openshift/origin/test/extended/two_node/utils"
)

// ReportAfterSuite validates that no tests were skipped due to unmet preconditions.
// This runs after all tests complete and after AfterSuite/SynchronizedAfterSuite.
//
// The two-node test suite requires a fully healthy cluster to run disruptive recovery tests.
// Individual tests skip (not fail) when preconditions aren't met to maintain test stability,
// but this reporting hook ensures the overall suite fails with diagnostic information about
// which tests were skipped and why, making precondition failures visible to CI analysis services.
//
// This validation only triggers if SkipIfClusterIsNotHealthy() was called and recorded skips,
// which means it automatically scopes to runs that included two-node tests with health issues.
var _ = g.ReportAfterSuite("Two Node Suite Precondition Validation", func(report g.Report) {
	skips := utils.GetPreconditionSkips()

	if len(skips) == 0 {
		// No tests were skipped due to precondition failures.
		// Either no two-node tests ran, or the cluster was healthy.
		return
	}

	// Build detailed failure message
	var testNames []string
	for testName := range skips {
		testNames = append(testNames, testName)
	}
	sort.Strings(testNames)

	var messages []string
	messages = append(messages, fmt.Sprintf("\n\n%d test(s) were skipped due to unmet cluster preconditions:", len(skips)))
	messages = append(messages, "This indicates the cluster was not in a healthy state when tests attempted to run.")
	messages = append(messages, "\nSkipped tests:")

	for _, testName := range testNames {
		reason := skips[testName]
		messages = append(messages, fmt.Sprintf("\n  • %s", testName))
		messages = append(messages, fmt.Sprintf("    Reason: %s", reason))
	}

	messages = append(messages, "\n\nThe two-node test suite requires a fully healthy cluster.")
	messages = append(messages, "Please investigate and resolve the cluster health issues before running this suite.")

	failureMessage := strings.Join(messages, "\n")
	o.Expect(skips).To(o.BeEmpty(), failureMessage)
})
