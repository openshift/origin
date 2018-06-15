package cmd

import (
	"io/ioutil"
	"strconv"
	"strings"
	"testing"

	"github.com/openshift/origin/pkg/oc/generate/app"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configcmd "github.com/openshift/origin/pkg/bulk"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imagefake "github.com/openshift/origin/pkg/image/generated/internalclientset/fake"
	newcmd "github.com/openshift/origin/pkg/oc/generate/cmd"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	templatefake "github.com/openshift/origin/pkg/template/generated/internalclientset/fake"
)

// TestNewAppDefaultFlags ensures that flags default values are set.
func TestNewAppDefaultFlags(t *testing.T) {
	config := newcmd.NewAppConfig()
	config.Deploy = true

	tests := map[string]struct {
		flagName   string
		defaultVal string
	}{
		"as test": {
			flagName:   "as-test",
			defaultVal: strconv.FormatBool(config.AsTestDeployment),
		},
		"code": {
			flagName:   "code",
			defaultVal: "[" + strings.Join(config.SourceRepositories, ",") + "]",
		},
		"context dir": {
			flagName:   "context-dir",
			defaultVal: "",
		},

		"image-stream": {
			flagName:   "image-stream",
			defaultVal: "[" + strings.Join(config.ImageStreams, ",") + "]",
		},
		"docker-image": {
			flagName:   "docker-image",
			defaultVal: "[" + strings.Join(config.DockerImages, ",") + "]",
		},
		"template": {
			flagName:   "template",
			defaultVal: "[" + strings.Join(config.Templates, ",") + "]",
		},
		"file": {
			flagName:   "file",
			defaultVal: "[" + strings.Join(config.TemplateFiles, ",") + "]",
		},
		"param": {
			flagName:   "param",
			defaultVal: "[" + strings.Join(config.TemplateParameters, ",") + "]",
		},
		"group": {
			flagName:   "group",
			defaultVal: "[" + strings.Join(config.Groups, ",") + "]",
		},
		"env": {
			flagName:   "env",
			defaultVal: "[" + strings.Join(config.Environment, ",") + "]",
		},
		"build-env": {
			flagName:   "build-env",
			defaultVal: "[" + strings.Join(config.BuildEnvironment, ",") + "]",
		},
		"name": {
			flagName:   "name",
			defaultVal: config.Name,
		},
		"strategy": {
			flagName:   "strategy",
			defaultVal: "",
		},
		"labels": {
			flagName:   "labels",
			defaultVal: "",
		},
		"insecure-registry": {
			flagName:   "insecure-registry",
			defaultVal: strconv.FormatBool(false),
		},
		"list": {
			flagName:   "list",
			defaultVal: strconv.FormatBool(false),
		},
		"search": {
			flagName:   "search",
			defaultVal: strconv.FormatBool(false),
		},
		"allow-missing-images": {
			flagName:   "allow-missing-images",
			defaultVal: strconv.FormatBool(false),
		},
		"allow-missing-imagestream-tags": {
			flagName:   "allow-missing-imagestream-tags",
			defaultVal: strconv.FormatBool(false),
		},
		"grant-install-rights": {
			flagName:   "grant-install-rights",
			defaultVal: strconv.FormatBool(false),
		},
		"no-install": {
			flagName:   "no-install",
			defaultVal: strconv.FormatBool(false),
		},
		"output-version": {
			flagName:   "output-version",
			defaultVal: "",
		},
	}

	cmd := NewCmdNewApplication("oc", NewAppRecommendedCommandName, nil, nil, nil, nil)

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

// TestNewAppRunFailure test failures.
func TestNewAppRunFailure(t *testing.T) {
	tests := map[string]struct {
		config      *newcmd.AppConfig
		expectedErr string
	}{
		"list_and_search": {
			config: &newcmd.AppConfig{
				AsList:   true,
				AsSearch: true,
			},
			expectedErr: "--list and --search can't be used together",
		},
		"list_with_arguments": {
			config: &newcmd.AppConfig{
				AsList: true,
				ComponentInputs: newcmd.ComponentInputs{
					Templates: []string{"test"},
				},
			},
			expectedErr: "--list can't be used with arguments",
		},
		"list_no_matches": {
			config: &newcmd.AppConfig{
				AsList: true,
			},
			expectedErr: "no matches found",
		},
		"search_with_source_code": {
			config: &newcmd.AppConfig{
				AsSearch: true,
				ComponentInputs: newcmd.ComponentInputs{
					Components:         []string{"mysql"},
					SourceRepositories: []string{"https://github.com/openshift/ruby-hello-world"},
				},
			},
			expectedErr: "--search can't be used with source code",
		},
		"search_with_env": {
			config: &newcmd.AppConfig{
				AsSearch: true,
				ComponentInputs: newcmd.ComponentInputs{
					Components: []string{"mysql"},
				},
				GenerationInputs: newcmd.GenerationInputs{
					Environment: []string{"FOO=BAR"},
				},
			},
			expectedErr: "--search can't be used with --env",
		},
		"search_with_build_env": {
			config: &newcmd.AppConfig{
				AsSearch: true,
				ComponentInputs: newcmd.ComponentInputs{
					Components: []string{"mysql"},
				},
				GenerationInputs: newcmd.GenerationInputs{
					BuildEnvironment: []string{"FOO=BAR"},
				},
			},
			expectedErr: "--search can't be used with --build-env",
		},
		"search_with_param": {
			config: &newcmd.AppConfig{
				AsSearch: true,
				ComponentInputs: newcmd.ComponentInputs{
					Components: []string{"mysql"},
				},
				GenerationInputs: newcmd.GenerationInputs{
					TemplateParameters: []string{"FOO=BAR"},
				},
			},
			expectedErr: "--search can't be used with --param",
		},
		"search_without_argument": {
			config: &newcmd.AppConfig{
				AsSearch: true,
				ComponentInputs: newcmd.ComponentInputs{
					DockerImages: []string{""},
				},
			},
			expectedErr: "no matches found",
		},
		"without_args": {
			config:      &newcmd.AppConfig{},
			expectedErr: "You must specify one or more images, image streams, templates, or source code locations to create an application.",
		},
	}

	opts := &NewAppOptions{
		ObjectGeneratorOptions: &ObjectGeneratorOptions{
			BaseName:    "oc",
			CommandName: NewAppRecommendedCommandName,
		},
	}

	for testName, test := range tests {
		test.config.Resolvers = newcmd.NewAppConfig().Resolvers
		test.config.Deploy = true

		opts.Config = test.config

		if err := opts.RunNewApp(); err != nil {
			if !strings.Contains(err.Error(), test.expectedErr) {
				t.Fatalf("[%s] error not expected: %+v", testName, err)
			}
		} else if len(test.expectedErr) != 0 {
			t.Fatalf("[%s] expected error: %v, got nil", testName, test.expectedErr)
		}
	}
}

// TestNewAppRunQueryActions ensures that NewApp Query commands calls the right actions.
func TestNewAppRunQueryActions(t *testing.T) {
	type testAction struct {
		namespace, verb, resource string
	}

	tests := []struct {
		name                         string
		config                       *newcmd.AppConfig
		expectedActions              []testAction
		expectedErr                  string
		expectedDockerVisited        bool
		expectedTemplateFilesVisited bool
	}{
		{
			name: "list",
			config: &newcmd.AppConfig{
				AsList: true,
			},
			expectedActions: []testAction{
				{namespace: "openshift", verb: "list", resource: "imagestreams"},
				{namespace: "openshift", verb: "list", resource: "templates"},
			},
			expectedDockerVisited: true,
		},
		{
			name: "search dockerimage",
			config: &newcmd.AppConfig{
				AsSearch: true,
				ComponentInputs: newcmd.ComponentInputs{
					DockerImages: []string{"repo/test"},
				},
			},
			expectedActions:       []testAction{},
			expectedDockerVisited: true,
		},
		{
			name: "search template",
			config: &newcmd.AppConfig{
				AsSearch: true,
				ComponentInputs: newcmd.ComponentInputs{
					Templates: []string{"test"},
				},
			},
			expectedActions: []testAction{
				{namespace: "openshift", verb: "list", resource: "templates"},
			},
		},
		{
			name: "search imagestream",
			config: &newcmd.AppConfig{
				AsSearch: true,
				ComponentInputs: newcmd.ComponentInputs{
					ImageStreams: []string{"testimage"},
				},
			},
			expectedActions: []testAction{
				{namespace: "openshift", verb: "list", resource: "imagestreams"},
			},
		},
		{
			name: "search templatefiles",
			config: &newcmd.AppConfig{
				AsSearch: true,
				ComponentInputs: newcmd.ComponentInputs{
					TemplateFiles: []string{"testfile"},
				},
			},
			expectedTemplateFilesVisited: true,
		},
		{
			name: "search all",
			config: &newcmd.AppConfig{
				AsSearch: true,
				ComponentInputs: newcmd.ComponentInputs{
					TemplateFiles: []string{"testfile"},
					DockerImages:  []string{"repo/test"},
					Templates:     []string{"test"},
					ImageStreams:  []string{"testimage"},
				},
			},
			expectedActions: []testAction{
				{namespace: "openshift", verb: "list", resource: "imagestreams"},
				{namespace: "openshift", verb: "list", resource: "templates"},
			},
			expectedDockerVisited:        true,
			expectedTemplateFilesVisited: true,
		},
		{
			name: "search template failure",
			config: &newcmd.AppConfig{
				AsSearch: true,
				ComponentInputs: newcmd.ComponentInputs{
					Templates: []string{"non-exist-template"},
				},
			},
			expectedActions: []testAction{
				{namespace: "openshift", verb: "list", resource: "templates"},
			},

			expectedErr: "no matches found",
		},
		{
			name: "search imagestream failure",
			config: &newcmd.AppConfig{
				AsSearch: true,
				ComponentInputs: newcmd.ComponentInputs{
					ImageStreams: []string{"#@@#%*"},
				},
			},
			expectedErr: "no matches found",
		},
		{
			name: "search dockerimage failure",
			config: &newcmd.AppConfig{
				AsSearch: true,
				ComponentInputs: newcmd.ComponentInputs{
					DockerImages: []string{"fakerepo/non-exist-image"},
				},
			},
			expectedDockerVisited: true,
			expectedActions:       []testAction{},
			expectedErr:           "no matches found",
		},
	}

	o := &NewAppOptions{
		ObjectGeneratorOptions: &ObjectGeneratorOptions{
			Action: configcmd.BulkAction{
				Out: ioutil.Discard,
			},
			BaseName:    "oc",
			CommandName: NewAppRecommendedCommandName,
		},
	}

	for _, test := range tests {
		// Prepare structure for test.
		templateClient := templatefake.NewSimpleClientset(fakeTemplateList())
		imageClient := imagefake.NewSimpleClientset(fakeImagestreamList())

		o.Config = test.config
		o.Config.Deploy = true

		o.Config.SetOpenShiftClient(imageClient.Image(), templateClient.Template(), nil, "openshift", nil)

		var dockerVisited, tfVisited bool
		o.Config.DockerSearcher = MockSearcher{
			OnSearch: func(precise bool, terms ...string) (app.ComponentMatches, []error) {
				dockerVisited = true
				match := &app.ComponentMatch{
					Name:  "repo/test",
					Image: &imageapi.DockerImage{},
				}
				return app.ComponentMatches{match}, []error{}
			},
		}
		o.Config.DockerSearcher = MockSearcher{
			OnSearch: func(precise bool, terms ...string) (app.ComponentMatches, []error) {
				dockerVisited = true
				return app.ComponentMatches{}, []error{}
			},
		}

		o.Config.TemplateFileSearcher = MockSearcher{
			OnSearch: func(precise bool, terms ...string) (app.ComponentMatches, []error) {
				tfVisited = true
				match := &app.ComponentMatch{
					Template: &templateapi.Template{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "testfile",
							Namespace: "openshift",
						},
					},
				}
				return app.ComponentMatches{match}, []error{}
			},
		}

		// Call RunNewApp and check expected behavior.
		if err := o.RunNewApp(); err != nil {
			if !strings.Contains(err.Error(), test.expectedErr) {
				t.Fatalf("[%s] error not expected: %v", test.name, err)
			}
		} else if len(test.expectedErr) != 0 {
			t.Fatalf("[%s] expected error: %v, got nil", test.name, test.expectedErr)
		}

		if dockerVisited != test.expectedDockerVisited {
			t.Errorf("[%s] error mismatch: expected %v, got %v", test.name, test.expectedDockerVisited, dockerVisited)
		}

		if tfVisited != test.expectedTemplateFilesVisited {
			t.Errorf("[%s] error mismatch: expected %v, got %v", test.name, test.expectedTemplateFilesVisited, tfVisited)
		}

		got := imageClient.Actions()
		got = append(got, templateClient.Actions()...)

		if len(test.expectedActions) != len(got) {
			t.Fatalf("[%s] action length mismatch: expected %d, got %d", test.name, len(test.expectedActions), len(got))
		}

		for i, action := range test.expectedActions {
			if !got[i].Matches(action.verb, action.resource) {
				t.Errorf("[%s] action mismatch: expected %s %s %s, got %s %s %s", test.name, action.namespace, action.verb, action.resource, got[i].GetNamespace(), got[i].GetVerb(), got[i].GetResource())
			}
		}
	}

}

func fakeTemplateList() *templateapi.TemplateList {
	return &templateapi.TemplateList{
		Items: []templateapi.Template{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "openshift",
				},
			},
		},
	}
}

func fakeImagestreamList() *imageapi.ImageStreamList {
	return &imageapi.ImageStreamList{
		Items: []imageapi.ImageStream{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testimage",
					Namespace: "openshift",
				},
			},
		},
	}
}
