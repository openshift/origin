package ginkgo

import (
	"fmt"
	"os"
	"strings"
)

type TestMetrics map[string]string

func WriteTestMetrics(outputFile string, tests []*testCase) error {
	testMetrics := make(TestMetrics)
	for _, test := range tests {
		for k, v := range test.testMetrics {
			testMetrics[k] = v
		}
	}
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
