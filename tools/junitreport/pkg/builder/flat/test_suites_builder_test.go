package flat

import (
	"reflect"
	"testing"

	"github.com/openshift/origin/tools/junitreport/pkg/api"
)

func TestAddSuite(t *testing.T) {
	var testCases = []struct {
		name           string
		seedSuites     *api.TestSuites
		suitesToAdd    []*api.TestSuite
		expectedSuites *api.TestSuites
	}{
		{
			name: "empty",
			suitesToAdd: []*api.TestSuite{
				{
					Name: "testSuite",
				},
			},
			expectedSuites: &api.TestSuites{
				Suites: []*api.TestSuite{
					{
						Name: "testSuite",
					},
				},
			},
		},
		{
			name: "populated",
			seedSuites: &api.TestSuites{
				Suites: []*api.TestSuite{
					{
						Name: "testSuite",
					},
				},
			},
			suitesToAdd: []*api.TestSuite{
				{
					Name: "testSuite2",
				},
			},
			expectedSuites: &api.TestSuites{
				Suites: []*api.TestSuite{
					{
						Name: "testSuite",
					},
					{
						Name: "testSuite2",
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		builder := NewTestSuitesBuilder()
		if testCase.seedSuites != nil {
			builder.(*flatTestSuitesBuilder).testSuites = testCase.seedSuites
		}

		for _, suite := range testCase.suitesToAdd {
			builder.AddSuite(suite)
		}

		if expected, actual := testCase.expectedSuites, builder.Build(); !reflect.DeepEqual(expected, actual) {
			t.Errorf("%s: did not correctly add suites:\n\texpected:\n\t%v,\n\tgot\n\t%v", testCase.name, expected, actual)
		}
	}
}
