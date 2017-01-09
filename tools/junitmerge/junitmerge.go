package main

import (
	"encoding/xml"
	"log"
	"os"

	"fmt"
	"github.com/openshift/origin/tools/junitreport/pkg/api"
	"sort"
)

type uniqueSuites map[string]*suiteRuns

func (s uniqueSuites) Merge(namePrefix string, suite *api.TestSuite) {
	name := suite.Name
	if len(namePrefix) > 0 {
		name = namePrefix + "/"
	}
	existing, ok := s[name]
	if !ok {
		existing = newSuiteRuns(suite)
		s[name] = existing
	}

	existing.Merge(suite.TestCases)

	for _, suite := range suite.Children {
		s.Merge(name, suite)
	}
}

type suiteRuns struct {
	suite *api.TestSuite
	runs  map[string]*api.TestCase
}

func newSuiteRuns(suite *api.TestSuite) *suiteRuns {
	return &suiteRuns{
		suite: suite,
		runs:  make(map[string]*api.TestCase),
	}
}

func (r *suiteRuns) Merge(testCases []*api.TestCase) {
	for _, testCase := range testCases {
		existing, ok := r.runs[testCase.Name]
		if !ok {
			r.runs[testCase.Name] = testCase
			continue
		}
		switch {
		case testCase.SkipMessage != nil:
			// if the new test is a skip, ignore it
		case existing.SkipMessage != nil && testCase.SkipMessage == nil:
			// always replace a skip with a non-skip
			r.runs[testCase.Name] = testCase
		case existing.FailureOutput == nil && testCase.FailureOutput != nil:
			// replace a passing test with a failing test
			r.runs[testCase.Name] = testCase
		}
	}
}

func main() {
	log.SetFlags(0)
	suites := make(uniqueSuites)

	for _, arg := range os.Args[1:] {
		f, err := os.Open(arg)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		d := xml.NewDecoder(f)

		for {
			t, err := d.Token()
			if err != nil {
				log.Fatal(err)
			}
			if t == nil {
				log.Fatalf("input file %s does not appear to be a JUnit XML file", arg)
			}
			// Inspect the top level DOM element and perform the appropriate action
			switch se := t.(type) {
			case xml.StartElement:
				switch se.Name.Local {
				case "testsuites":
					input := &api.TestSuites{}
					if err := d.DecodeElement(input, &se); err != nil {
						log.Fatal(err)
					}
					for _, suite := range input.Suites {
						suites.Merge("", suite)
					}
				case "testsuite":
					input := &api.TestSuite{}
					if err := d.DecodeElement(input, &se); err != nil {
						log.Fatal(err)
					}
					suites.Merge("", input)
				default:
					log.Fatal(fmt.Errorf("unexpected top level element in %s: %s", arg, se.Name.Local))
				}
			default:
				continue
			}
			break
		}
	}

	var suiteNames []string
	for k := range suites {
		suiteNames = append(suiteNames, k)
	}
	sort.Sort(sort.StringSlice(suiteNames))
	output := &api.TestSuites{}

	for _, name := range suiteNames {
		suite := suites[name]

		out := &api.TestSuite{
			Name:     name,
			NumTests: uint(len(suite.runs)),
		}

		var keys []string
		for k := range suite.runs {
			keys = append(keys, k)
		}
		sort.Sort(sort.StringSlice(keys))

		for _, k := range keys {
			testCase := suite.runs[k]
			out.TestCases = append(out.TestCases, testCase)
			switch {
			case testCase.SkipMessage != nil:
				out.NumSkipped++
			case testCase.FailureOutput != nil:
				out.NumFailed++
			}
			out.Duration += testCase.Duration
		}
		output.Suites = append(output.Suites, out)
	}

	e := xml.NewEncoder(os.Stdout)
	e.Indent("", "\t")
	if err := e.Encode(output); err != nil {
		log.Fatal(err)
	}
	e.Flush()
	fmt.Fprintln(os.Stdout)
}
