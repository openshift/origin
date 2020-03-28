package h2spec

import (
	"encoding/xml"
	"io"
)

// DecodeJUnitReport decodes the test results from reader.
func DecodeJUnitReport(reader io.Reader) ([]*JUnitTestSuite, error) {
	var report JUnitTestReport

	if err := xml.NewDecoder(reader).Decode(&report); err != nil {
		return nil, err
	}

	return report.TestSuites, nil
}
