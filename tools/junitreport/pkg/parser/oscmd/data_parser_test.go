package oscmd

import (
	"testing"

	"github.com/openshift/origin/tools/junitreport/pkg/api"
)

func TestMarksTestBeginning(t *testing.T) {
	var testCases = []struct {
		name     string
		testLine string
	}{
		{
			name:     "default",
			testLine: "=== BEGIN TEST CASE ===",
		},
		{
			name:     "failed print before",
			testLine: "some other text=== BEGIN TEST CASE ===",
		},
		{
			name:     "failed print after",
			testLine: "=== BEGIN TEST CASE ===some other text after",
		},
	}

	parser := newTestDataParser()
	for _, testCase := range testCases {
		if !parser.MarksBeginning(testCase.testLine) {
			t.Errorf("%s: did not correctly determine that line %q marked test beginning", testCase.name, testCase.testLine)
		}
	}
}

func TestExtractTestName(t *testing.T) {
	var testCases = []struct {
		name         string
		testLine     string
		expectedName string
	}{
		{
			name:         "test declaration",
			testLine:     `hack/test-cmd.sh:152: executing 'openshift ex validate master-config /tmp/openshift/test-cmd//master-config-broken.yaml' expecting failure and text 'ERROR'`,
			expectedName: `hack/test-cmd.sh:152: executing 'openshift ex validate master-config /tmp/openshift/test-cmd//master-config-broken.yaml' expecting failure and text 'ERROR'`,
		},
		{
			name:         "test conclusion success",
			testLine:     `SUCCESS after 0.041s: hack/../test/cmd/basicresources.sh:21: executing 'oc create -f test/testdata/resource-builder/directory -f test/testdata/resource-builder/json-no-extension -f test/testdata/resource-builder/yml-no-extension' expecting success`,
			expectedName: `hack/../test/cmd/basicresources.sh:21: executing 'oc create -f test/testdata/resource-builder/directory -f test/testdata/resource-builder/json-no-extension -f test/testdata/resource-builder/yml-no-extension' expecting success`,
		},
		{
			name:         "test conclusion failure",
			testLine:     `FAILURE after 30.239s: hack/../test/cmd/builds.sh:68: executing 'oc new-build -D "FROM centos:7" -o json | python -m json.tool' expecting success: the command returned the wrong error code`,
			expectedName: `hack/../test/cmd/builds.sh:68: executing 'oc new-build -D "FROM centos:7" -o json | python -m json.tool' expecting success`,
		},
		{
			name:         "failed print: test conclusion success",
			testLine:     `some other textSUCCESS after 0.041s: hack/../test/cmd/basicresources.sh:21: executing 'oc create -f test/testdata/resource-builder/directory -f test/testdata/resource-builder/json-no-extension -f test/testdata/resource-builder/yml-no-extension' expecting success`,
			expectedName: `hack/../test/cmd/basicresources.sh:21: executing 'oc create -f test/testdata/resource-builder/directory -f test/testdata/resource-builder/json-no-extension -f test/testdata/resource-builder/yml-no-extension' expecting success`,
		},
	}

	parser := newTestDataParser()
	for _, testCase := range testCases {
		actual, contained := parser.ExtractName(testCase.testLine)
		if !contained {
			t.Errorf("%s: failed to extract name from line %q", testCase.name, testCase.testLine)
		}
		if testCase.expectedName != actual {
			t.Errorf("%s: did not correctly extract name from line:\n%q:\n\texpected\n\t%q,\n\tgot\n\t%q", testCase.name, testCase.testLine, testCase.expectedName, actual)
		}
	}
}

func TestExtractResult(t *testing.T) {
	var testCases = []struct {
		name           string
		testLine       string
		expectedResult api.TestResult
	}{
		{
			name:           "success",
			testLine:       `SUCCESS after 0.046s: hack/../test/cmd/basicresources.sh:35: executing 'oc delete pods hello-openshift' expecting success`,
			expectedResult: api.TestResultPass,
		},
		{
			name:           "failure",
			testLine:       `FAILURE after 30.239s: hack/../test/cmd/builds.sh:68: executing 'oc new-build -D "FROM centos:7" -o json | python -m json.tool' expecting success: the command returned the wrong error code`,
			expectedResult: api.TestResultFail,
		},
		{
			name:           "try until failure",
			testLine:       `SUCCESS after 0.044s: hack/../test/cmd/basicresources.sh:41: executing 'oc label pod/hello-openshift acustom=label' expecting success; re-trying every 0.2s until completion or 60.000s`,
			expectedResult: api.TestResultPass,
		},
	}

	parser := newTestDataParser()
	for _, testCase := range testCases {
		actual, contained := parser.ExtractResult(testCase.testLine)
		if !contained {
			t.Errorf("%s: failed to extract result from line %q", testCase.name, testCase.testLine)
		}
		if testCase.expectedResult != actual {
			t.Errorf("%s: did not correctly extract result from line %q: expected %q, got %q", testCase.name, testCase.testLine, testCase.expectedResult, actual)
		}
	}
}

func TestExtractDuration(t *testing.T) {
	var testCases = []struct {
		name             string
		testLine         string
		expectedDuration string
	}{
		{
			name:             "test conclusion success",
			testLine:         `SUCCESS after 0.041s: hack/../test/cmd/basicresources.sh:21: executing 'oc create -f test/testdata/resource-builder/directory -f test/testdata/resource-builder/json-no-extension -f test/testdata/resource-builder/yml-no-extension' expecting success`,
			expectedDuration: "0.041s",
		},
		{
			name:             "test conclusion failure",
			testLine:         `FAILURE after 30.239s: hack/../test/cmd/builds.sh:68: executing 'oc new-build -D "FROM centos:7" -o json | python -m json.tool' expecting success: the command returned the wrong error code`,
			expectedDuration: "30.239s",
		},
	}

	parser := newTestDataParser()
	for _, testCase := range testCases {
		actual, contained := parser.ExtractDuration(testCase.testLine)
		if !contained {
			t.Errorf("%s: failed to extract duration from line %q", testCase.name, testCase.testLine)
		}
		if testCase.expectedDuration != actual {
			t.Errorf("%s: did not correctly extract duration from line %q: expected %q, got %q", testCase.name, testCase.testLine, testCase.expectedDuration, actual)
		}
	}
}

func TestExtractMessage(t *testing.T) {
	var testCases = []struct {
		name            string
		testLine        string
		expectedMessage string
	}{
		{
			name:            "fail on error code",
			testLine:        `FAILURE after 0.041s: hack/../test/cmd/help.sh:32: executing 'oc' expecting success: the command returned the wrong error code`,
			expectedMessage: "the command returned the wrong error code",
		},
		{
			name:            "fail on text",
			testLine:        `FAILURE after 0.027s: hack/../test/cmd/help.sh:39: executing 'oc' expecting success and text 'Build and Deploy Commands:': the output content test failed`,
			expectedMessage: "the output content test failed",
		},
		{
			name:            "fail on both error code and text",
			testLine:        `FAILURE after 0.024s: hack/../test/cmd/help.sh:40: executing 'oc' expecting success and text 'Other Commands:': the command returned the wrong error code; the output content test failed`,
			expectedMessage: "the command returned the wrong error code; the output content test failed",
		},
		{
			name:            "fail on timeout",
			testLine:        `FAILURE after 13.514s: hack/../test/cmd/images.sh:54: executing 'oc get imagestreamtags wildfly:latest' expecting success; re-trying every 0.2s until completion or 60.000s: the command timed out`,
			expectedMessage: "the command timed out",
		},
	}

	parser := newTestDataParser()
	for _, testCase := range testCases {
		actual, contained := parser.ExtractMessage(testCase.testLine)
		if !contained {
			t.Errorf("%s: failed to extract duration from line %q", testCase.name, testCase.testLine)
		}
		if testCase.expectedMessage != actual {
			t.Errorf("%s: did not correctly extract message from line %q: expected %q, got %q", testCase.name, testCase.testLine, testCase.expectedMessage, actual)
		}
	}
}

func TestMarksTestCompletion(t *testing.T) {
	var testCases = []struct {
		name     string
		testLine string
	}{
		{
			name:     "default",
			testLine: "=== END TEST CASE ===",
		},
		{
			name:     "failed print before",
			testLine: "some other text=== END TEST CASE ===",
		},
		{
			name:     "failed print after",
			testLine: "=== END TEST CASE ===some other text after",
		},
	}

	parser := newTestDataParser()
	for _, testCase := range testCases {
		if !parser.MarksCompletion(testCase.testLine) {
			t.Errorf("%s: did not correctly determine that line %q marked test completion", testCase.name, testCase.testLine)
		}
	}
}

func TestMarksSuiteBeginning(t *testing.T) {
	var testCases = []struct {
		name     string
		testLine string
	}{
		{
			name:     "basic",
			testLine: "=== BEGIN TEST SUITE package/name ===",
		},
		{
			name:     "numeric",
			testLine: "=== BEGIN TEST SUITE 1234 ===",
		},
		{
			name:     "url",
			testLine: "=== BEGIN TEST SUITE github.com/maintainer/repository/package/file ===",
		},
		{
			name:     "failed print",
			testLine: `some other textok=== BEGIN TEST SUITE package/name ===`,
		},
	}

	parser := newTestSuiteDataParser()
	for _, testCase := range testCases {
		if !parser.MarksBeginning(testCase.testLine) {
			t.Errorf("%s: did not correctly determine that line %q marked the start of a suite", testCase.name, testCase.testLine)
		}
	}
}

func TestExtractSuiteName(t *testing.T) {
	var testCases = []struct {
		name         string
		testLine     string
		expectedName string
	}{
		{
			name:         "basic",
			testLine:     "=== BEGIN TEST SUITE package/name ===",
			expectedName: "package/name",
		},
		{
			name:         "numeric",
			testLine:     "=== BEGIN TEST SUITE 1234 ===",
			expectedName: "1234",
		},
		{
			name:         "url",
			testLine:     "=== BEGIN TEST SUITE github.com/maintainer/repository/package/file ===",
			expectedName: "github.com/maintainer/repository/package/file",
		},
		{
			name:         "failed print",
			testLine:     `some other text=== BEGIN TEST SUITE package/name ===`,
			expectedName: "package/name",
		},
	}

	parser := newTestSuiteDataParser()
	for _, testCase := range testCases {
		actual, contained := parser.ExtractName(testCase.testLine)
		if !contained {
			t.Errorf("%s: failed to extract name from line %q", testCase.name, testCase.testLine)
		}
		if testCase.expectedName != actual {
			t.Errorf("%s: did not correctly extract suite name from line %q: expected %q, got %q", testCase.name, testCase.testLine, testCase.expectedName, actual)
		}
	}
}

func TestMarksSuiteCompletion(t *testing.T) {
	var testCases = []struct {
		name     string
		testLine string
	}{
		{
			name:     "basic",
			testLine: "=== END TEST SUITE ===",
		},
		{
			name:     "failed print",
			testLine: `some other text=== END TEST SUITE ===`,
		},
	}

	parser := newTestSuiteDataParser()
	for _, testCase := range testCases {
		if !parser.MarksCompletion(testCase.testLine) {
			t.Errorf("%s: did not correctly determine that line %q marked the end of a suite", testCase.name, testCase.testLine)
		}
	}
}
