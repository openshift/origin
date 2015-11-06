package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/jstemmer/go-junit-report/formatter"

	"github.com/openshift/origin/cmd/junitreport/parser"
)

var (
	noXMLHeader bool
	setExitCode bool
	fileName    string
)

func init() {
	flag.BoolVar(&noXMLHeader, "no-xml-header", false, "do not print xml header")
	flag.BoolVar(&setExitCode, "set-exit-code", false, "set exit code to 1 if tests failed")
	flag.StringVar(&fileName, "file", "", "path to the test output file to parse")
}

func main() {
	flag.Parse()

	// Read input
	var input io.Reader
	var err error
	if len(fileName) > 0 {
		input, err = os.Open(fileName)
		if err != nil {
			fmt.Printf("could not open the input file: %v\n", err)
			os.Exit(1)
		}
	} else {
		input = os.Stdin
	}

	report, err := parser.Parse(input)
	if err != nil {
		fmt.Printf("Error reading input: %s\n", err)
		os.Exit(1)
	}
	// Write xml
	err = formatter.JUnitReportXML(report, noXMLHeader, os.Stdout)
	if err != nil {
		fmt.Printf("Error writing XML: %s\n", err)
		os.Exit(1)
	}

	if setExitCode && report.Failures() > 0 {
		os.Exit(1)
	}
}
