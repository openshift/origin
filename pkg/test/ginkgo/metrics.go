package ginkgo

import (
	"fmt"
	"os"
	"strings"

	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

type TestMetrics map[string]string

func writeTestMetrics(outputFile string, testMetrics TestMetrics) error {
	lines := make([]string, 0)
	for k, v := range testMetrics {
		lines = append(lines, fmt.Sprintf("%s %s", k, v))
	}
	f, err := os.OpenFile(outputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open metrics file for writing: %v", err)
	}

	defer f.Close()
	fmt.Fprint(f, strings.Join(lines, "\n")+"\n")
	return nil
}

func WriteTestMetrics(outputFile string, tests []*testCase) error {
	testMetrics := make(TestMetrics)
	for _, test := range tests {
		for k, v := range test.testMetrics {
			testMetrics[k] = v
		}
	}
	return writeTestMetrics(outputFile, testMetrics)
}

func WriteSyntheticTestMetrics(outputFile string, tests []*junitapi.JUnitTestCase) error {
	testMetrics := make(TestMetrics)
	for _, test := range tests {
		for k, v := range test.TestMetrics {
			testMetrics[k] = v
		}
	}
	return writeTestMetrics(outputFile, testMetrics)
}
