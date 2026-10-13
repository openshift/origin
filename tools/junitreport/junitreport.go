package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/openshift/origin/tools/junitreport/pkg/cmd"
)

var (
	// parserType is a flag that holds the type of parser to use
	parserType string

	// builderType is a flag that holds the type of builder to use
	builderType string

	// rootSuites is a flag that holds the comma-delimited list of root suite names
	rootSuites string

	// testOutputFile is a flag that holds the path to the file containing test output
	testOutputFile string

	// outputFile is a flag that holds the path to the jUnit XML report to be written
	outputFile string

	// stream is a flag that determines if a streamed subset of the input stream should be printed as it is read
	stream bool
)

const (
	defaultParserType     = "gotest"
	defaultBuilderType    = "flat"
	defaultTestOutputFile = "/dev/stdin"
	defaultOutputFile     = "/dev/stdout"
	defaultFilter         = false
)

func init() {
	flag.StringVar(&parserType, "type", defaultParserType, "which type of test output to parse")
	flag.StringVar(&builderType, "suites", defaultBuilderType, "which test suite structure to use")
	flag.StringVar(&rootSuites, "roots", "", "comma-delimited list of root suite names")
	flag.StringVar(&testOutputFile, "f", defaultTestOutputFile, "the path to the file containing test output to consume")
	flag.StringVar(&outputFile, "output", defaultOutputFile, "the path to the jUnit XML output file to write")
	flag.BoolVar(&stream, "stream", defaultFilter, "print a streamed subset of the input as it is read")
}

const (
	junitReportUsageLong = `Consume test output to create jUnit XML files and summarize jUnit XML files.

%[1]s consumes test output through Stdin and creates jUnit XML files. Currently, only the output of 'go test'
and the output of 'oscmd' functions with $JUNIT_REPORT_OUTPUT set are supported. jUnit XML can be build with
nested or flat test suites. Sub-trees of test suites can be selected when using the nested test-suites represen-
tation to only build XML for some subset of the test output. This parser is greedy, so all output not directly
related to a test suite is considered test case output.
`

	junitReportUsage = `Usage:
  %[1]s [--type=TEST-OUTPUT-TYPE] [--suites=SUITE-TYPE] [-f=FILE]
  %[1]s [-f=FILE] summarize
`

	junitReportExamples = `Examples:
  # Consume 'go test' output to create a jUnit XML file
  go test -v -cover ./... | %[1]s > report.xml

  # Consume 'go test' output to create a jUnit XML file, while also printing package output as it is generated
  go test -v -cover ./... | %[1]s --stream > report.xml

  # Consume 'go test' output from a file to create a jUnit XML file
  %[1]s -f testoutput.txt > report.xml

  # Consume 'go test' output to create a specific jUnit XML file
  %[1]s --output report.xml

  # Consume 'go test' output to create a jUnit XML file with nested test suites
  go test -v -cover ./... | junitreport --suites=nested > report.xml

  # Consume 'go test' output to create a jUnit XML file with nested test suites rooted at 'github.com/maintainer'
  go test -v -cover ./... | junitreport --suites=nested --roots=github.com/maintainer > report.xml

  # Describe failures and skipped tests in an existing jUnit XML file
  cat report.xml | %[1]s summarize

  # Consume 'os::cmd' output from to create a jUnit XML file
  JUNIT_REPORT='true' hack/test-cmd.sh | junitreport --type=os::cmd > report.xml
`
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, junitReportUsageLong+"\n", os.Args[0])
		fmt.Fprintf(os.Stderr, junitReportUsage+"\n", os.Args[0])
		fmt.Fprintf(os.Stderr, junitReportExamples+"\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Options:")
		flag.PrintDefaults()
		os.Exit(2)
	}

	flag.Parse()

	var rootSuiteNames []string
	if len(rootSuites) > 0 {
		rootSuiteNames = strings.Split(rootSuites, ",")
	}

	var input io.Reader
	if testOutputFile == defaultTestOutputFile {
		input = os.Stdin
	} else {
		file, err := os.Open(testOutputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading input file: %v\n", err)
		}
		defer file.Close()
		input = file
	}

	arguments := flag.Args()
	// If we are asked to summarize an XML file, that is all we do
	if len(arguments) == 1 && arguments[0] == "summarize" {
		summary, err := cmd.Summarize(input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error summarizing jUnit XML file: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprint(os.Stdout, summary)
		os.Exit(0)
	}
	if len(arguments) > 1 {
		fmt.Fprintf(os.Stderr, "Incorrect usage of %[1]s, see '%[1]s --help' for more details.\n", os.Args[0])
		os.Exit(1)
	}

	var output io.Writer
	if outputFile == defaultOutputFile {
		output = os.Stdout
	} else {
		file, err := os.Create(outputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
		}
		defer file.Close()
		output = file
	}

	// Otherwise, we get ready to parse and generate XML output.
	options := cmd.JUnitReportOptions{
		Stream: stream,
		Input:  input,
		Output: output,
	}

	err := options.Complete(builderType, parserType, rootSuiteNames)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	err = options.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating output: %v\n", err)
		os.Exit(1)
	}
}
