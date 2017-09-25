package cmd

import (
	"bytes"
	"fmt"
	"strconv"
	"testing"

	projectfake "github.com/openshift/origin/pkg/project/generated/internalclientset/fake"
)

// TestRequestProjectDefaultFlags ensures that flags default values are set.
func TestRequestProjectDefaultFlags(t *testing.T) {

	tests := map[string]struct {
		flagName   string
		defaultVal string
	}{
		"display name": {
			flagName:   "display-name",
			defaultVal: "",
		},
		"description": {
			flagName:   "description",
			defaultVal: "",
		},
		"skip config write": {
			flagName:   "skip-config-write",
			defaultVal: strconv.FormatBool(false),
		},
	}

	cmd := NewCmdRequestProject("oc", RequestProjectRecommendedCommandName, nil, nil, nil)

	for _, v := range tests {
		f := cmd.Flag(v.flagName)
		if f == nil {
			t.Fatalf("expected flag %s to be registered but found none", v.flagName)
		}

		if f.DefValue != v.defaultVal {
			t.Errorf("expected default value of %s for %s but found %s", v.defaultVal, v.flagName, f.DefValue)
		}
	}
}

// DISABLE_TestRequestProjectRun ensures that Run command calls the right actions.
func DISABLE_TestRequestProjectRun(t *testing.T) {
	client := projectfake.NewSimpleClientset()
	buf := &bytes.Buffer{}

	test := struct {
		opts            *NewProjectOptions
		expectedActions []testAction
		expectedErr     error
	}{
		opts: &NewProjectOptions{
			Out:         buf,
			Server:      "127.0.0.1",
			Client:      client.Project(),
			Name:        "oc",
			ProjectName: "yourproject",
		},
		expectedActions: []testAction{
			{verb: "list", resource: "newprojects"},
			{verb: "create", resource: "newprojects"},
		},
		expectedErr: nil,
	}

	expectedOutput := fmt.Sprintf(requestProjectSwitchProjectOutput, test.opts.Name, test.opts.ProjectName, test.opts.Server)

	if err := test.opts.Run(); err != test.expectedErr {
		t.Fatalf("error mismatch: expected %v, got %v", test.expectedErr, err)
	}

	if buf.String() != expectedOutput {
		t.Fatalf("error mismatch output: expected %v, got %v", expectedOutput, buf)
	}

	got := client.Actions()
	if len(test.expectedActions) != len(got) {
		t.Fatalf("action length mismatch: expected %d, got %d", len(test.expectedActions), len(got))
	}

	for i, action := range test.expectedActions {
		if !got[i].Matches(action.verb, action.resource) {
			t.Errorf("action mismatch: expected %s %s, got %s %s", action.verb, action.resource, got[i].GetVerb(), got[i].GetResource())
		}
	}

}
