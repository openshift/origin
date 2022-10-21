package ginkgo

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

// Define types, these are subsets of the sippy APIs of the same name, copied here to eliminate a lot of the cruft.
// ProwJobRunTest defines a join table linking tests to the job runs they execute in, along with the status for
// that execution.
// We're getting dangerously close to being able to live push results after a job run.

type ProwJobRun struct {
	ProwJob ProwJob
	URL     string
	Tests   []ProwJobRunTest
}

type ProwJob struct {
	Name string
}

type Test struct {
	Name string
}

type Suite struct {
	Name string
}

type ProwJobRunTest struct {
	Test   Test
	Suite  Suite
	Status int // would like to use smallint here, but gorm auto-migrate breaks trying to change the type every start
}

// WriteJobRunTestFailureSummary writes a more minimal json file summarizing a little info about the
// job run, and what tests flaked and failed. (successful tests are omitted)
// This is intended to be later submitted to sippy for a risk analysis of how unusual the
// test failures were, but that final step is handled elsewhere.
func WriteJobRunTestFailureSummary(artifactDir, timeSuffix string, finalSuiteResults *junitapi.JUnitTestSuite) error {

	tests := map[string]*passFail{}

	for _, testCase := range finalSuiteResults.TestCases {
		if _, ok := tests[testCase.Name]; !ok {
			tests[testCase.Name] = &passFail{}
		}
		if testCase.SkipMessage != nil {
			continue
		}

		if testCase.FailureOutput != nil {
			tests[testCase.Name].Failed = true
		} else {
			tests[testCase.Name].Passed = true
		}
	}

	jr := ProwJobRun{
		ProwJob: ProwJob{Name: os.Getenv("JOB_NAME")},
		URL:     os.Getenv("JOB_URL"), // just a guess, may not exist
		Tests:   []ProwJobRunTest{},
	}

	for k, v := range tests {
		if !v.Failed {
			// if no failures, it is neither a fail nor a flake:
			continue
		}
		jr.Tests = append(jr.Tests, ProwJobRunTest{
			Test:   Test{Name: k},
			Suite:  Suite{Name: finalSuiteResults.Name},
			Status: getSippyStatusCode(v),
		})
	}

	jsonContent, err := json.MarshalIndent(jr, "", "    ")
	if err != nil {
		return err
	}
	outputFile := filepath.Join(artifactDir, fmt.Sprintf("test-flakes-and-failures-%s.json", timeSuffix))
	return ioutil.WriteFile(outputFile, jsonContent, 0644)
}

// passFail is a simple struct to track test names which can appear more than once.
// If both passed and failed are true, it was a flake.
type passFail struct {
	Passed bool
	Failed bool
}

// getSippyStatusCode returns the code sippy uses internally for each type of failure.
func getSippyStatusCode(pf *passFail) int {
	switch {
	case pf.Failed && pf.Passed:
		return 13 // flake
	case pf.Failed && !pf.Passed:
		return 12 // fail
	}
	// we should not hit this given the above filtering
	return 0
}
