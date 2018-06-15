package cmd

import (
	"io/ioutil"
	"strings"
	"testing"

	"github.com/MakeNowJust/heredoc"
	configcmd "github.com/openshift/origin/pkg/bulk"
	imagefake "github.com/openshift/origin/pkg/image/generated/internalclientset/fake"
	"github.com/openshift/origin/pkg/oc/generate/app"
	newcmd "github.com/openshift/origin/pkg/oc/generate/cmd"
	templatefake "github.com/openshift/origin/pkg/template/generated/internalclientset/fake"
)

// TestNewBuildRun ensures that Run command calls the right actions
// and returns the expected error.
func TestNewBuildRun(t *testing.T) {
	tests := []struct {
		name            string
		config          *newcmd.AppConfig
		expectedActions []testAction
		expectedErr     string
	}{
		{
			name:        "no input",
			config:      &newcmd.AppConfig{},
			expectedErr: usageError("oc new-build", newBuildNoInput, "oc", "new-build").Error(),
		},
		{
			name: "no matches",
			config: &newcmd.AppConfig{
				ComponentInputs: newcmd.ComponentInputs{
					Components: []string{"test"},
				},
			},
			expectedErr: heredoc.Doc(`
				The 'oc new-build' command will match arguments to the following types:

				  1. Images tagged into image streams in the current project or the 'openshift' project
				     - if you don't specify a tag, we'll add ':latest'
				  2. Images in the Docker Hub, on remote registries, or on the local Docker engine
				  3. Git repository URLs or local paths that point to Git repositories

				--allow-missing-images can be used to force the use of an image that was not matched

				See 'oc new-build -h' for examples.`),
			expectedActions: []testAction{
				{verb: "list", resource: "imagestreams"},
				{verb: "list", resource: "templates"},
			},
		},
	}

	o := &NewBuildOptions{
		ObjectGeneratorOptions: &ObjectGeneratorOptions{
			Action: configcmd.BulkAction{
				Out: ioutil.Discard,
			},
			CommandPath: "oc new-build",
			BaseName:    "oc",
			CommandName: "new-build",
		},
	}

	for _, test := range tests {
		templateClient := templatefake.NewSimpleClientset()
		imageClient := imagefake.NewSimpleClientset()

		o.Config = test.config
		o.Config.SetOpenShiftClient(imageClient.Image(), templateClient.Template(), nil, "openshift", nil)

		o.Config.DockerSearcher = MockSearcher{
			OnSearch: func(precise bool, terms ...string) (app.ComponentMatches, []error) {
				return app.ComponentMatches{}, []error{}
			},
		}
		o.Config.TemplateFileSearcher = MockSearcher{
			OnSearch: func(precise bool, terms ...string) (app.ComponentMatches, []error) {
				return app.ComponentMatches{}, []error{}
			},
		}
		if err := o.RunNewBuild(); err != nil {
			if !strings.Contains(err.Error(), test.expectedErr) {
				t.Fatalf("[%s] error not expected: %v", test.name, err)
			}
		} else if len(test.expectedErr) != 0 {
			t.Fatalf("[%s] expected error: %v, got nil", test.name, test.expectedErr)
		}

		got := imageClient.Actions()
		got = append(got, templateClient.Actions()...)
		if len(test.expectedActions) != len(got) {
			t.Fatalf("action length mismatch: expected %d, got %d", len(test.expectedActions), len(got))
		}

		for i, action := range test.expectedActions {
			if !got[i].Matches(action.verb, action.resource) {
				t.Errorf("action mismatch: expected %s %s, got %s %s", action.verb, action.resource, got[i].GetVerb(), got[i].GetResource())
			}
		}
	}

}

// MockSearcher implements Searcher.
type MockSearcher struct {
	OnSearch func(precise bool, terms ...string) (app.ComponentMatches, []error)
}

func (m MockSearcher) Type() string {
	return ""
}

// Search mocks a search.
func (m MockSearcher) Search(precise bool, terms ...string) (app.ComponentMatches, []error) {
	return m.OnSearch(precise, terms...)
}
