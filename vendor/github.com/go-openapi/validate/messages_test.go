// Copyright 2015 go-swagger maintainers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validate

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/loads/fmts"
	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

var (
	// This debug environment variable allows to report and capture actual validation messages
	// during testing. It should be disabled (undefined) during CI tests.
	DebugTest = os.Getenv("SWAGGER_DEBUG_TEST") != ""
)

func init() {
	loads.AddLoader(fmts.YAMLMatcher, fmts.YAMLDoc)
}

type ExpectedMessage struct {
	Message              string `yaml:"message"`
	WithContinueOnErrors bool   `yaml:"withContinueOnErrors"` // should be expected only when SetContinueOnErrors(true)
	IsRegexp             bool   `yaml:"isRegexp"`             // expected message is interpreted as regexp (with regexp.MatchString())
}

type ExpectedFixture struct {
	Comment           string            `yaml:"comment,omitempty"`
	Todo              string            `yaml:"todo,omitempty"`
	ExpectedLoadError bool              `yaml:"expectedLoadError"` // expect error on load: skip validate step
	ExpectedValid     bool              `yaml:"expectedValid"`     // expect valid spec
	ExpectedMessages  []ExpectedMessage `yaml:"expectedMessages"`
	ExpectedWarnings  []ExpectedMessage `yaml:"expectedWarnings"`
	Tested            bool              `yaml:"-"`
	Failed            bool              `yaml:"-"`
}

type ExpectedMap map[string]*ExpectedFixture

// Test message improvements, issue #44 and some more
// ContinueOnErrors mode on
// WARNING: this test is very demanding and constructed with varied scenarios,
// which are not necessarily "unitary". Expect multiple changes in messages whenever
// altering the validator.
func Test_MessageQualityContinueOnErrors_Issue44(t *testing.T) {
	if !enableLongTests {
		skipNotify(t)
		t.SkipNow()
	}
	errs := testMessageQuality(t, true, true) /* set haltOnErrors=true to iterate spec by spec */
	assert.Zero(t, errs, "Message testing didn't match expectations")
}

// ContinueOnErrors mode off
func Test_MessageQualityStopOnErrors_Issue44(t *testing.T) {
	if !enableLongTests {
		skipNotify(t)
		t.SkipNow()
	}
	errs := testMessageQuality(t, true, false) /* set haltOnErrors=true to iterate spec by spec */
	assert.Zero(t, errs, "Message testing didn't match expectations")
}

func loadTestConfig(t *testing.T, fp string) ExpectedMap {
	expectedConfig, err := ioutil.ReadFile(fp)
	require.NoErrorf(t, err, "cannot read expected messages config file: %v", err)

	tested := make(ExpectedMap, 200)

	err = yaml.Unmarshal(expectedConfig, &tested)
	require.NoErrorf(t, err, "cannot unmarshall expected messages from config file : %v", err)

	// Check config
	for fixture, expected := range tested {
		require.Nil(t, UniqueItems("", "", expected.ExpectedMessages), "duplicate error messages configured for %s", fixture)
		require.Nil(t, UniqueItems("", "", expected.ExpectedWarnings), "duplicate warning messages configured for %s", fixture)
	}
	return tested
}

func testMessageQuality(t *testing.T, haltOnErrors bool, continueOnErrors bool) int {
	// Verifies the production of validation error messages in multiple
	// spec scenarios.
	//
	// The objective is to demonstrate that:
	//   - messages are stable
	//   - validation continues as much as possible, even in presence of many errors
	//
	// haltOnErrors is used in dev mode to study and fix testcases step by step (output is pretty verbose)
	//
	// set SWAGGER_DEBUG_TEST=1 env to get a report of messages at the end of each test.
	// expectedMessage{"", false, false},
	//
	// expected messages and warnings are configured in ./fixtures/validation/expected_messages.yaml
	//
	var errs int // error count

	tested := loadTestConfig(t, filepath.Join("fixtures", "validation", "expected_messages.yaml"))

	if err := filepath.Walk(filepath.Join("fixtures", "validation"), testWalkSpecs(t, tested, haltOnErrors, continueOnErrors)); err != nil {
		t.Logf("%v", err)
		errs++
	}
	recapTest(t, tested)
	return errs
}

func testDebugLog(t *testing.T, thisTest *ExpectedFixture) {
	if DebugTest {
		if thisTest.Comment != "" {
			t.Logf("\tDEVMODE: Comment: %s", thisTest.Comment)
		}
		if thisTest.Todo != "" {
			t.Logf("\tDEVMODE: Todo: %s", thisTest.Todo)
		}
	}
}

func expectInvalid(t *testing.T, path string, thisTest *ExpectedFixture, continueOnErrors bool) {
	// Checking invalid specs
	t.Logf("Testing messages for invalid spec: %s", path)
	testDebugLog(t, thisTest)

	doc, err := loads.Spec(path)

	// Check specs with load errors (error is located in pkg loads or spec)
	if thisTest.ExpectedLoadError {
		// Expect a load error: no further validation may possibly be conducted.
		require.Error(t, err, "expected this spec to return a load error")
		assert.Equal(t, 0, verifyLoadErrors(t, err, thisTest.ExpectedMessages))
		return
	}

	require.NoError(t, err, "expected this spec to load properly")

	// Validate the spec document
	validator := NewSpecValidator(doc.Schema(), strfmt.Default)
	validator.SetContinueOnErrors(continueOnErrors)
	res, warn := validator.Validate(doc)

	// Check specs with load errors (error is located in pkg loads or spec)
	require.False(t, res.IsValid(), "expected this spec to be invalid")

	errs := verifyErrorsVsWarnings(t, res, warn)
	errs += verifyErrors(t, res, thisTest.ExpectedMessages, "error", continueOnErrors)
	errs += verifyErrors(t, warn, thisTest.ExpectedWarnings, "warning", continueOnErrors)
	assert.Equal(t, 0, errs)

	if errs > 0 {
		t.Logf("Message qualification on spec validation failed for %s", path)
		// DEVMODE allows developers to experiment and tune expected results
		if DebugTest {
			reportTest(t, path, res, thisTest.ExpectedMessages, "error", continueOnErrors)
			reportTest(t, path, warn, thisTest.ExpectedWarnings, "warning", continueOnErrors)
		}
	}
}

func expectValid(t *testing.T, path string, thisTest *ExpectedFixture, continueOnErrors bool) {
	// Expecting no message (e.g.valid spec): 0 message expected
	t.Logf("Testing valid spec: %s", path)
	testDebugLog(t, thisTest)

	doc, err := loads.Spec(path)
	require.NoError(t, err, "expected this spec to load without error")

	validator := NewSpecValidator(doc.Schema(), strfmt.Default)
	validator.SetContinueOnErrors(continueOnErrors)
	res, warn := validator.Validate(doc)
	assert.True(t, res.IsValid(), "expected this spec to be valid")
	assert.Lenf(t, res.Errors, 0, "expected no returned errors")

	// check warnings
	errs := verifyErrors(t, warn, thisTest.ExpectedWarnings, "warning", continueOnErrors)
	assert.Equal(t, 0, errs)

	if DebugTest && errs > 0 {
		reportTest(t, path, res, thisTest.ExpectedMessages, "error", continueOnErrors)
		reportTest(t, path, warn, thisTest.ExpectedWarnings, "warning", continueOnErrors)
	}
}

func checkMustHalt(t *testing.T, haltOnErrors bool) {
	if t.Failed() && haltOnErrors {
		assert.FailNow(t, "test halted: stop testing on message checking error mode")
		return
	}
}

func testWalkSpecs(t *testing.T, tested ExpectedMap, haltOnErrors, continueOnErrors bool) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		thisTest, found := tested[info.Name()]

		if info.IsDir() || !found { // skip
			return nil
		}

		t.Run(path, func(t *testing.T) {
			if !DebugTest { // when running in dev mode, run serially
				t.Parallel()
			}
			defer func() {
				thisTest.Tested = true
				thisTest.Failed = t.Failed()
			}()

			if !thisTest.ExpectedValid {
				expectInvalid(t, path, thisTest, continueOnErrors)
				checkMustHalt(t, haltOnErrors)
			} else {
				expectValid(t, path, thisTest, continueOnErrors)
				checkMustHalt(t, haltOnErrors)
			}
		})
		return nil
	}
}

func recapTest(t *testing.T, config ExpectedMap) {
	recapFailed := false
	for k, v := range config {
		if !v.Tested {
			t.Logf("WARNING: %s configured but not tested (fixture not found)", k)
			recapFailed = true
		} else if v.Failed {
			t.Logf("ERROR: %s failed passing messages verification", k)
			recapFailed = true
		}
	}
	if !recapFailed {
		t.Log("INFO:We are good")
	}
}
func reportTest(t *testing.T, path string, res *Result, expectedMessages []ExpectedMessage, msgtype string, continueOnErrors bool) {
	// Prints out a recap of error messages. To be enabled during development / test iterations
	verifiedErrors := make([]string, 0, 50)
	lines := make([]string, 0, 50)
	for _, e := range res.Errors {
		verifiedErrors = append(verifiedErrors, e.Error())
	}
	t.Logf("DEVMODE:Recap of returned %s messages while validating %s ", msgtype, path)
	for _, v := range verifiedErrors {
		status := fmt.Sprintf("Unexpected %s", msgtype)
		for _, s := range expectedMessages {
			if (s.WithContinueOnErrors && continueOnErrors) || !s.WithContinueOnErrors {
				if s.IsRegexp {
					if matched, _ := regexp.MatchString(s.Message, v); matched {
						status = fmt.Sprintf("Expected %s", msgtype)
						break
					}
				} else {
					if strings.Contains(v, s.Message) {
						status = fmt.Sprintf("Expected %s", msgtype)
						break
					}
				}
			}
		}
		lines = append(lines, fmt.Sprintf("[%s]%s", status, v))
	}

	for _, s := range expectedMessages {
		if (s.WithContinueOnErrors && continueOnErrors) || !s.WithContinueOnErrors {
			status := fmt.Sprintf("Missing %s", msgtype)
			for _, v := range verifiedErrors {
				if s.IsRegexp {
					if matched, _ := regexp.MatchString(s.Message, v); matched {
						status = fmt.Sprintf("Expected %s", msgtype)
						break
					}
				} else {
					if strings.Contains(v, s.Message) {
						status = fmt.Sprintf("Expected %s", msgtype)
						break
					}
				}
			}
			if status != fmt.Sprintf("Expected %s", msgtype) {
				lines = append(lines, fmt.Sprintf("[%s]%s", status, s.Message))
			}
		}
	}
	if len(lines) > 0 {
		sort.Strings(lines)
		for _, line := range lines {
			t.Logf(line)
		}
	}
}

func verifyErrorsVsWarnings(t *testing.T, res, warn *Result) int {
	// First verification of result conventions: results are redundant, just a matter of presentation
	w := len(warn.Errors)
	if !assert.Len(t, res.Warnings, w) ||
		!assert.Len(t, warn.Warnings, 0) ||
		!assert.Subset(t, res.Warnings, warn.Errors) ||
		!assert.Subset(t, warn.Errors, res.Warnings) {
		t.Log("Result equivalence errors vs warnings not verified")
		return 1
	}
	return 0
}

func verifyErrors(t *testing.T, res *Result, expectedMessages []ExpectedMessage, msgtype string, continueOnErrors bool) int {
	var numExpected, errs int
	verifiedErrors := make([]string, 0, 50)

	for _, e := range res.Errors {
		verifiedErrors = append(verifiedErrors, e.Error())
	}
	for _, s := range expectedMessages {
		if (s.WithContinueOnErrors == true && continueOnErrors == true) || s.WithContinueOnErrors == false {
			numExpected++
		}
	}

	// We got the expected number of messages (e.g. no duplicates, no uncontrolled side-effect, ...)
	if !assert.Len(t, verifiedErrors, numExpected, "unexpected number of %s messages returned. Wanted %d, got %d", msgtype, numExpected, len(verifiedErrors)) {
		errs++
	}

	// Check that all expected messages are here
	for _, s := range expectedMessages {
		found := false
		if (s.WithContinueOnErrors == true && continueOnErrors == true) || s.WithContinueOnErrors == false {
			for _, v := range verifiedErrors {
				if s.IsRegexp {
					if matched, _ := regexp.MatchString(s.Message, v); matched {
						found = true
						break
					}
				} else {
					if strings.Contains(v, s.Message) {
						found = true
						break
					}
				}
			}
			if !assert.True(t, found, "Missing expected %s message: %s", msgtype, s.Message) {
				errs++
			}
		}
	}

	// Check for no unexpected message
	for _, v := range verifiedErrors {
		found := false
		for _, s := range expectedMessages {
			if (s.WithContinueOnErrors == true && continueOnErrors == true) || s.WithContinueOnErrors == false {
				if s.IsRegexp {
					if matched, _ := regexp.MatchString(s.Message, v); matched {
						found = true
						break
					}
				} else {
					if strings.Contains(v, s.Message) {
						found = true
						break
					}
				}
			}
		}
		if !assert.True(t, found, "unexpected %s message: %s", msgtype, v) {
			errs++
		}
	}
	return errs
}

func verifyLoadErrors(t *testing.T, err error, expectedMessages []ExpectedMessage) int {
	var errs int

	// Perform several matches on single error message
	// Process here error messages from loads (normally unit tested in the load package:
	// we just want to figure out how all this is captured at the validate package level.
	v := err.Error()
	for _, s := range expectedMessages {
		var found bool
		if s.IsRegexp {
			if found, _ = regexp.MatchString(s.Message, v); found {
				break
			}
		} else {
			if found = strings.Contains(v, s.Message); found {
				break
			}
		}
		if !assert.True(t, found, "unexpected load error: %s", v) {
			t.Logf("Expecting one of the following:")
			for _, s := range expectedMessages {
				smode := "Contains"
				if s.IsRegexp {
					smode = "MatchString"
				}
				t.Logf("[%s]:%s", smode, s.Message)
			}
			errs++
		}
	}
	return errs
}

func testIssue(t *testing.T, path string, expectedNumErrors, expectedNumWarnings int) {
	res, _ := loadAndValidate(t, path)
	if expectedNumErrors > -1 && !assert.Len(t, res.Errors, expectedNumErrors) {
		t.Log("Returned errors:")
		for _, e := range res.Errors {
			t.Logf("%v", e)
		}
	}
	if expectedNumWarnings > -1 && !assert.Len(t, res.Warnings, expectedNumWarnings) {
		t.Log("Returned warnings:")
		for _, e := range res.Warnings {
			t.Logf("%v", e)
		}
	}
}

// Test unitary fixture for dev and bug fixing
func Test_SingleFixture(t *testing.T) {
	t.SkipNow()
	path := filepath.Join("fixtures", "validation", "fixture-1231.yaml")
	testIssue(t, path, -1, -1)
}
