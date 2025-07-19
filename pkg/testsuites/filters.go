package testsuites

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/openshift/origin/pkg/test/ginkgo"
)

// SuitesString returns a string with the provided suites formatted. Prefix is
// printed at the beginning of the output.
func SuitesString(suites []*ginkgo.TestSuite, prefix string) string {
	buf := &bytes.Buffer{}
	fmt.Fprint(buf, prefix)
	for _, suite := range suites {
		fmt.Fprintf(buf, "%s\n", suite.Name)

		// Add source information
		if suite.Extension != nil {
			fmt.Fprintf(buf, "  Source: Extension (%s:%s:%s)\n", suite.Extension.Component.Product, suite.Extension.Component.Kind, suite.Extension.Component.Name)
			if suite.Extension.Source.SourceImage != "" {
				fmt.Fprintf(buf, "  Image: %s\n", suite.Extension.Source.SourceImage)
			}
			if suite.Extension.Source.SourceURL != "" {
				fmt.Fprintf(buf, "  URL: %s\n", suite.Extension.Source.SourceURL)
			}
		} else {
			fmt.Fprintf(buf, "  Source: Internal\n")
		}

		// Add description with proper indentation
		if suite.Description != "" {
			// Split description into lines and indent each line
			lines := strings.Split(strings.TrimSpace(suite.Description), "\n")
			fmt.Fprintf(buf, "  Description:\n")
			for _, line := range lines {
				trimmedLine := strings.TrimSpace(line)
				if trimmedLine != "" {
					fmt.Fprintf(buf, "    %s\n", trimmedLine)
				}
			}
		}

		fmt.Fprintf(buf, "\n")
	}
	return buf.String()
}

// isStandardEarlyTest returns true if a test is considered part of the normal
// pre or post condition tests.
func isStandardEarlyTest(name string) bool {
	if !strings.Contains(name, "[Early]") {
		return false
	}
	return strings.Contains(name, "[Suite:openshift/conformance/parallel")
}

// isStandardEarlyOrLateTest returns true if a test is considered part of the normal
// pre or post condition tests.
func isStandardEarlyOrLateTest(name string) bool {
	if !strings.Contains(name, "[Early]") && !strings.Contains(name, "[Late]") {
		return false
	}
	return strings.Contains(name, "[Suite:openshift/conformance/parallel")
}

// withStandardEarlyOrLateTests combines a CEL expression with the standard early/late test logic.
// It returns a CEL expression that matches tests that either satisfy the provided expression
// OR are standard early/late tests.
func withStandardEarlyOrLateTests(expr string) string {
	earlyLateExpr := `(name.contains("[Early]") || name.contains("[Late]")) && name.contains("[Suite:openshift/conformance/parallel")`

	if expr == "" {
		return earlyLateExpr
	}

	return fmt.Sprintf("(%s) || (%s)", expr, earlyLateExpr)
}

// withStandardEarlyTests combines a CEL expression with the standard early test logic.
// It returns a CEL expression that matches tests that either satisfy the provided expression
// OR are standard early tests.
func withStandardEarlyTests(expr string) string {
	earlyExpr := `name.contains("[Early]") && name.contains("[Suite:openshift/conformance/parallel")`

	if expr == "" {
		return earlyExpr
	}

	return fmt.Sprintf("(%s) || (%s)", expr, earlyExpr)
}
