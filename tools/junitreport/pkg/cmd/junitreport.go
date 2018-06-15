package cmd

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io"

	"github.com/openshift/origin/tools/junitreport/pkg/builder"
	"github.com/openshift/origin/tools/junitreport/pkg/builder/flat"
	"github.com/openshift/origin/tools/junitreport/pkg/builder/nested"
	"github.com/openshift/origin/tools/junitreport/pkg/parser"
	"github.com/openshift/origin/tools/junitreport/pkg/parser/gotest"
	"github.com/openshift/origin/tools/junitreport/pkg/parser/oscmd"
)

type testSuitesBuilderType string

const (
	flatBuilderType   testSuitesBuilderType = "flat"
	nestedBuilderType testSuitesBuilderType = "nested"
)

var supportedBuilderTypes = []testSuitesBuilderType{flatBuilderType, nestedBuilderType}

type testParserType string

const (
	goTestParserType testParserType = "gotest"
	osCmdParserType  testParserType = "oscmd"
)

var supportedTestParserTypes = []testParserType{goTestParserType, osCmdParserType}

type JUnitReportOptions struct {
	// BuilderType is the type of test suites builder to use
	BuilderType testSuitesBuilderType

	// RootSuiteNames is a list of root suites to be used for nested test suite output if
	// the root suite is to be more specific than the suite name without any suite delimeters
	// i.e. if `github.com/owner/repo` is to be used instead of `github.com`
	RootSuiteNames []string

	// ParserType is the parser type that will be used to parse test output
	ParserType testParserType

	// Stream determines if package result lines should be printed to the output as they are found
	Stream bool

	// Input is the reader for the test output to be parsed
	Input io.Reader

	// Output is the writer for the file to which the XML is written
	Output io.Writer
}

func (o *JUnitReportOptions) Complete(builderType, parserType string, rootSuiteNames []string) error {
	switch testSuitesBuilderType(builderType) {
	case flatBuilderType:
		o.BuilderType = flatBuilderType
	case nestedBuilderType:
		o.BuilderType = nestedBuilderType
	default:
		return fmt.Errorf("unrecognized test suites builder type: got %s, expected one of %v", builderType, supportedBuilderTypes)
	}

	switch testParserType(parserType) {
	case goTestParserType:
		o.ParserType = goTestParserType
	case osCmdParserType:
		o.ParserType = osCmdParserType
	default:
		return fmt.Errorf("unrecognized test parser type: got %s, expected one of %v", parserType, supportedTestParserTypes)
	}

	o.RootSuiteNames = rootSuiteNames

	return nil
}

func (o *JUnitReportOptions) Run() error {
	var builder builder.TestSuitesBuilder
	switch o.BuilderType {
	case flatBuilderType:
		builder = flat.NewTestSuitesBuilder()
	case nestedBuilderType:
		builder = nested.NewTestSuitesBuilder(o.RootSuiteNames)
	}

	var testParser parser.TestOutputParser
	switch o.ParserType {
	case goTestParserType:
		testParser = gotest.NewParser(builder, o.Stream)
	case osCmdParserType:
		testParser = oscmd.NewParser(builder, o.Stream)
	}

	testSuites, err := testParser.Parse(bufio.NewScanner(o.Input))
	if err != nil {
		return err
	}

	_, err = io.WriteString(o.Output, xml.Header)
	if err != nil {
		return fmt.Errorf("error writing XML header to file: %v", err)
	}

	encoder := xml.NewEncoder(o.Output)
	encoder.Indent("", "\t") // no prefix, indent with tabs

	if err := encoder.Encode(testSuites); err != nil {
		return fmt.Errorf("error encoding test suites to XML: %v", err)
	}

	_, err = io.WriteString(o.Output, "\n")
	if err != nil {
		return fmt.Errorf("error writing last newline to file: %v", err)
	}

	return nil
}
