package cmd

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/sets"

	buildapi "github.com/openshift/origin/pkg/build/api"
	client "github.com/openshift/origin/pkg/client/testclient"
	"github.com/openshift/origin/pkg/generate/app"
	image "github.com/openshift/origin/pkg/image/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
	"github.com/openshift/source-to-image/pkg/test"

	_ "github.com/openshift/origin/pkg/api/install"
)

func TestValidate(t *testing.T) {
	tests := map[string]struct {
		cfg                 AppConfig
		componentValues     []string
		sourceRepoLocations []string
		env                 map[string]string
		parms               map[string]string
	}{
		"components": {
			cfg: AppConfig{
				ComponentInputs: ComponentInputs{
					Components: []string{"one", "two", "three/four"},
				},
			},
			componentValues:     []string{"one", "two", "three/four"},
			sourceRepoLocations: []string{},
			env:                 map[string]string{},
			parms:               map[string]string{},
		},
		"envs": {
			cfg: AppConfig{
				GenerationInputs: GenerationInputs{
					Environment: []string{"one=first", "two=second", "three=third"},
				},
			},
			componentValues:     []string{},
			sourceRepoLocations: []string{},
			env:                 map[string]string{"one": "first", "two": "second", "three": "third"},
			parms:               map[string]string{},
		},
		"component+source": {
			cfg: AppConfig{
				ComponentInputs: ComponentInputs{
					Components: []string{"one~https://server/repo.git"},
				},
			},
			componentValues:     []string{"one"},
			sourceRepoLocations: []string{"https://server/repo.git"},
			env:                 map[string]string{},
			parms:               map[string]string{},
		},
		"components+source": {
			cfg: AppConfig{
				ComponentInputs: ComponentInputs{
					Components: []string{"mysql+ruby~git://github.com/namespace/repo.git"},
				},
			},
			componentValues:     []string{"mysql", "ruby"},
			sourceRepoLocations: []string{"git://github.com/namespace/repo.git"},
			env:                 map[string]string{},
			parms:               map[string]string{},
		},
		"components+parms": {
			cfg: AppConfig{
				ComponentInputs: ComponentInputs{
					Components: []string{"ruby-helloworld-sample"},
				},
				GenerationInputs: GenerationInputs{
					TemplateParameters: []string{"one=first", "two=second"},
				},
			},
			componentValues:     []string{"ruby-helloworld-sample"},
			sourceRepoLocations: []string{},
			env:                 map[string]string{},
			parms: map[string]string{
				"one": "first",
				"two": "second",
			},
		},
	}
	for n, c := range tests {
		b := &app.ReferenceBuilder{}
		env, parms, err := c.cfg.validate()
		if err != nil {
			t.Errorf("%s: Unexpected error: %v", n, err)
			continue
		}

		if err := AddComponentInputsToRefBuilder(b, &c.cfg.Resolvers, &c.cfg.ComponentInputs, &c.cfg.GenerationInputs); err != nil {
			t.Errorf("%s: Unexpected error: %v", n, err)
			continue
		}
		cr, _, errs := b.Result()
		if len(errs) > 0 {
			t.Errorf("%s: Unexpected error: %v", n, errs)
			continue
		}

		compValues := []string{}
		for _, r := range cr {
			compValues = append(compValues, r.Input().Value)
		}
		if !reflect.DeepEqual(c.componentValues, compValues) {
			t.Errorf("%s: Component values don't match. Expected: %v, Got: %v", n, c.componentValues, compValues)
		}
		if len(env) != len(c.env) {
			t.Errorf("%s: Environment variables don't match. Expected: %v, Got: %v", n, c.env, env)
		}
		for e, v := range env {
			if c.env[e] != v {
				t.Errorf("%s: Environment variables don't match. Expected: %v, Got: %v", n, c.env, env)
				break
			}
		}
		if len(parms) != len(c.parms) {
			t.Errorf("%s: Template parameters don't match. Expected: %v, Got: %v", n, c.parms, parms)
		}
		for p, v := range parms {
			if c.parms[p] != v {
				t.Errorf("%s: Template parameters don't match. Expected: %v, Got: %v", n, c.parms, parms)
				break
			}
		}
	}
}

func TestBuildTemplates(t *testing.T) {
	tests := map[string]struct {
		templateName string
		namespace    string
		parms        map[string]string
	}{
		"simple": {
			templateName: "first-stored-template",
			namespace:    "default",
			parms:        map[string]string{},
		},
	}
	for n, c := range tests {
		appCfg := AppConfig{}
		appCfg.Out = &bytes.Buffer{}
		appCfg.SetOpenShiftClient(&client.Fake{}, c.namespace, nil)
		appCfg.KubeClient = ktestclient.NewSimpleFake()
		appCfg.TemplateSearcher = fakeTemplateSearcher()
		appCfg.AddArguments([]string{c.templateName})
		appCfg.TemplateParameters = []string{}
		for k, v := range c.parms {
			appCfg.TemplateParameters = append(appCfg.TemplateParameters, fmt.Sprintf("%v=%v", k, v))
		}

		_, parms, err := appCfg.validate()
		if err != nil {
			t.Errorf("%s: Unexpected error: %v", n, err)
			continue
		}

		resolved, err := Resolve(&appCfg.Resolvers, &appCfg.ComponentInputs, &appCfg.GenerationInputs)
		if err != nil {
			t.Errorf("%s: Unexpected error: %v", n, err)
			continue
		}
		components := resolved.Components

		err = components.Resolve()
		if err != nil {
			t.Errorf("%s: Unexpected error: %v", n, err)
			continue
		}
		_, _, err = appCfg.buildTemplates(components, app.Environment(parms))
		if err != nil {
			t.Errorf("%s: Unexpected error: %v", n, err)
		}
		for _, component := range components {
			match := component.Input().ResolvedMatch
			if !match.IsTemplate() {
				t.Errorf("%s: Expected template match, got: %v", n, match)
			}
			if fmt.Sprintf("%s/%s", c.namespace, c.templateName) != match.Name {
				t.Errorf("%s: Expected template name %q, got: %q", n, c.templateName, match.Name)
			}
			if len(parms) != len(c.parms) {
				t.Errorf("%s: Template parameters don't match. Expected: %v, Got: %v", n, c.parms, parms)
			}
			for p, v := range parms {
				if c.parms[p] != v {
					t.Errorf("%s: Template parameters don't match. Expected: %v, Got: %v", n, c.parms, parms)
					break
				}
			}
		}
	}
}

func fakeTemplateSearcher() app.Searcher {
	client := &client.Fake{}
	client.AddReactor("list", "templates", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, templateList(), nil
	})
	return app.TemplateSearcher{
		Client:     client,
		Namespaces: []string{"default"},
	}
}

func templateList() *templateapi.TemplateList {
	return &templateapi.TemplateList{
		Items: []templateapi.Template{
			{
				Objects: []runtime.Object{},
				ObjectMeta: kapi.ObjectMeta{
					Name:      "first-stored-template",
					Namespace: "default",
				},
			},
		},
	}
}

func TestEnsureHasSource(t *testing.T) {
	gitLocalDir := test.CreateLocalGitDirectory(t)
	defer os.RemoveAll(gitLocalDir)

	tests := []struct {
		name              string
		cfg               AppConfig
		components        app.ComponentReferences
		repositories      []*app.SourceRepository
		expectedErr       string
		dontExpectToBuild bool
	}{
		{
			name: "One requiresSource, multiple repositories",
			components: app.ComponentReferences{
				app.ComponentReference(&app.ComponentInput{
					ExpectToBuild: true,
				}),
			},
			repositories: mockSourceRepositories(t, gitLocalDir),
			expectedErr:  "there are multiple code locations provided - use one of the following suggestions",
		},
		{
			name: "Multiple requiresSource, multiple repositories",
			components: app.ComponentReferences{
				app.ComponentReference(&app.ComponentInput{
					ExpectToBuild: true,
				}),
				app.ComponentReference(&app.ComponentInput{
					ExpectToBuild: true,
				}),
			},
			repositories: mockSourceRepositories(t, gitLocalDir),
			expectedErr:  "Use '[image]~[repo]' to declare which code goes with which image",
		},
		{
			name: "One requiresSource, no repositories",
			components: app.ComponentReferences{
				app.ComponentReference(&app.ComponentInput{
					ExpectToBuild: true,
				}),
			},
			repositories:      []*app.SourceRepository{},
			expectedErr:       "",
			dontExpectToBuild: true,
		},
		{
			name: "Multiple requiresSource, no repositories",
			components: app.ComponentReferences{
				app.ComponentReference(&app.ComponentInput{
					ExpectToBuild: true,
				}),
				app.ComponentReference(&app.ComponentInput{
					ExpectToBuild: true,
				}),
			},
			repositories:      []*app.SourceRepository{},
			expectedErr:       "",
			dontExpectToBuild: true,
		},
		{
			name: "Successful - one repository",
			components: app.ComponentReferences{
				app.ComponentReference(&app.ComponentInput{
					ExpectToBuild: false,
				}),
			},
			repositories: mockSourceRepositories(t, gitLocalDir)[:1],
			expectedErr:  "",
		},
		{
			name: "Successful - no requiresSource",
			components: app.ComponentReferences{
				app.ComponentReference(&app.ComponentInput{
					ExpectToBuild: false,
				}),
			},
			repositories: mockSourceRepositories(t, gitLocalDir),
			expectedErr:  "",
		},
	}
	for _, test := range tests {
		err := EnsureHasSource(test.components, test.repositories, &test.cfg.GenerationInputs)
		if err != nil {
			if !strings.Contains(err.Error(), test.expectedErr) {
				t.Errorf("%s: Invalid error: Expected %s, got %v", test.name, test.expectedErr, err)
			}
		} else if len(test.expectedErr) != 0 {
			t.Errorf("%s: Expected %s error but got none", test.name, test.expectedErr)
		}
		if test.dontExpectToBuild {
			for _, comp := range test.components {
				if comp.NeedsSource() {
					t.Errorf("%s: expected component reference to not require source.", test.name)
				}
			}
		}
	}
}

// mockSourceRepositories is a set of mocked source repositories used for
// testing.
func mockSourceRepositories(t *testing.T, file string) []*app.SourceRepository {
	var b []*app.SourceRepository
	for _, location := range []string{
		"https://github.com/openshift/ruby-hello-world.git",
		file,
	} {
		s, err := app.NewSourceRepository(location)
		if err != nil {
			t.Fatal(err)
		}
		b = append(b, s)
	}
	return b
}

// Make sure that buildPipelines defaults DockerImage.Config if needed to
// avoid a nil panic.
func TestBuildPipelinesWithUnresolvedImage(t *testing.T) {
	dockerFile, err := app.NewDockerfile("FROM centos\nEXPOSE 1234\nEXPOSE 4567")
	if err != nil {
		t.Fatal(err)
	}

	sourceRepo, err := app.NewSourceRepository("https://github.com/foo/bar.git")
	if err != nil {
		t.Fatal(err)
	}
	sourceRepo.BuildWithDocker()
	sourceRepo.SetInfo(&app.SourceRepositoryInfo{
		Dockerfile: dockerFile,
	})

	refs := app.ComponentReferences{
		app.ComponentReference(&app.ComponentInput{
			Value:         "mysql",
			Uses:          sourceRepo,
			ExpectToBuild: true,
			ResolvedMatch: &app.ComponentMatch{
				Value: "mysql",
			},
		}),
	}

	a := AppConfig{}
	a.Out = &bytes.Buffer{}
	group, err := a.buildPipelines(refs, app.Environment{})
	if err != nil {
		t.Error(err)
	}

	expectedPorts := sets.NewString("1234", "4567")
	actualPorts := sets.NewString()
	for port := range group[0].InputImage.Info.Config.ExposedPorts {
		actualPorts.Insert(port)
	}
	if e, a := expectedPorts.List(), actualPorts.List(); !reflect.DeepEqual(e, a) {
		t.Errorf("Expected ports=%v, got %v", e, a)
	}
}

func TestBuildOutputCycleResilience(t *testing.T) {

	config := &AppConfig{}

	mockIS := &image.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Name: "mockimagestream",
		},
		Spec: image.ImageStreamSpec{
			Tags: make(map[string]image.TagReference),
		},
	}
	mockIS.Spec.Tags["latest"] = image.TagReference{
		From: &kapi.ObjectReference{
			Kind: "DockerImage",
			Name: "mockimage:latest",
		},
	}

	dfn := "mockdockerfilename"
	malOutputBC := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "buildCfgWithWeirdOutputObjectRef",
		},
		Spec: buildapi.BuildConfigSpec{
			CommonSpec: buildapi.CommonSpec{
				Source: buildapi.BuildSource{
					Dockerfile: &dfn,
				},
				Strategy: buildapi.BuildStrategy{
					DockerStrategy: &buildapi.DockerBuildStrategy{
						From: &kapi.ObjectReference{
							Kind: "ImageStreamTag",
							Name: "mockimagestream:latest",
						},
					},
				},
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Kind: "NewTypeOfRef",
						Name: "Yet-to-be-implemented",
					},
				},
			},
		},
	}

	_, err := config.followRefToDockerImage(malOutputBC.Spec.Output.To, nil, []runtime.Object{malOutputBC, mockIS})
	expected := "Unable to follow reference type: \"NewTypeOfRef\""
	if err == nil || err.Error() != expected {
		t.Errorf("Expected error from followRefToDockerImage: got \"%v\" versus expected %q", err, expected)
	}
}

func TestBuildOutputCycleWithFollowingTag(t *testing.T) {

	config := &AppConfig{}

	mockIS := &image.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Name: "mockimagestream",
		},
		Spec: image.ImageStreamSpec{
			Tags: make(map[string]image.TagReference),
		},
	}
	mockIS.Spec.Tags["latest"] = image.TagReference{
		From: &kapi.ObjectReference{
			Kind: "ImageStreamTag",
			Name: "10.0",
		},
	}
	mockIS.Spec.Tags["10.0"] = image.TagReference{
		From: &kapi.ObjectReference{
			Kind: "DockerImage",
			Name: "mockimage:latest",
		},
	}

	dfn := "mockdockerfilename"
	followingTagCycleBC := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "buildCfgWithWeirdOutputObjectRef",
		},
		Spec: buildapi.BuildConfigSpec{
			CommonSpec: buildapi.CommonSpec{
				Source: buildapi.BuildSource{
					Dockerfile: &dfn,
				},
				Strategy: buildapi.BuildStrategy{
					DockerStrategy: &buildapi.DockerBuildStrategy{
						From: &kapi.ObjectReference{
							Kind: "ImageStreamTag",
							Name: "mockimagestream:latest",
						},
					},
				},
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Kind: "ImageStreamTag",
						Name: "mockimagestream:10.0",
					},
				},
			},
		},
	}

	expected := "output image of \"mockimage:latest\" should be different than input"
	err := config.checkCircularReferences([]runtime.Object{followingTagCycleBC, mockIS})
	if err == nil || err.Error() != expected {
		t.Errorf("Expected error from followRefToDockerImage: got \"%v\" versus expected %q", err, expected)
	}
}

func TestAllowedNonNumericExposedPorts(t *testing.T) {
	tests := []struct {
		strategy             string
		allowNonNumericPorts bool
	}{
		{
			strategy:             "",
			allowNonNumericPorts: true,
		},
		{
			strategy:             "source",
			allowNonNumericPorts: false,
		},
	}

	for _, test := range tests {
		config := &AppConfig{}
		config.Strategy = test.strategy
		config.AllowNonNumericExposedPorts = test.allowNonNumericPorts

		repo, err := app.NewSourceRepositoryForDockerfile("FROM centos\nARG PORT=80\nEXPOSE $PORT")
		if err != nil {
			t.Errorf("Unexpected error during setup: %v", err)
			continue
		}
		repos := app.SourceRepositories{repo}

		err = optionallyValidateExposedPorts(config, repos)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func TestDisallowedNonNumericExposedPorts(t *testing.T) {
	tests := []struct {
		strategy             string
		allowNonNumericPorts bool
	}{
		{
			strategy:             "",
			allowNonNumericPorts: false,
		},
		{
			strategy:             "docker",
			allowNonNumericPorts: false,
		},
	}

	for _, test := range tests {
		config := &AppConfig{}
		config.Strategy = test.strategy
		config.AllowNonNumericExposedPorts = test.allowNonNumericPorts

		repo, err := app.NewSourceRepositoryForDockerfile("FROM centos\nARG PORT=80\nEXPOSE 8080 $PORT")
		if err != nil {
			t.Fatalf("Unexpected error during setup: %v", err)
		}
		repos := app.SourceRepositories{repo}

		err = optionallyValidateExposedPorts(config, repos)
		if err == nil {
			t.Error("Expected error wasn't returned")

		} else if !strings.Contains(err.Error(), "invalid EXPOSE") || !strings.Contains(err.Error(), "must be numeric") {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}
