package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/dockertools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	docker "github.com/fsouza/go-dockerclient"

	buildapi "github.com/openshift/origin/pkg/build/api"
	client "github.com/openshift/origin/pkg/client/testclient"
	deploy "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/dockerregistry"
	"github.com/openshift/origin/pkg/generate/app"
	"github.com/openshift/origin/pkg/generate/dockerfile"
	"github.com/openshift/origin/pkg/generate/source"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/util/namer"
)

func TestAddArguments(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "test-newapp")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testDir := filepath.Join(tmpDir, "test/one/two/three")
	err = os.MkdirAll(testDir, 0777)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	tests := map[string]struct {
		args       []string
		env        util.StringList
		parms      util.StringList
		repos      util.StringList
		components util.StringList
		unknown    []string
	}{
		"components": {
			args:       []string{"one", "two+three", "four~five"},
			components: util.StringList{"one", "two+three", "four~five"},
			unknown:    []string{},
		},
		"source": {
			args:    []string{".", testDir, "git://server/repo.git"},
			repos:   util.StringList{".", testDir, "git://server/repo.git"},
			unknown: []string{},
		},
		"env": {
			args:    []string{"first=one", "second=two", "third=three"},
			env:     util.StringList{"first=one", "second=two", "third=three"},
			unknown: []string{},
		},
		"mix 1": {
			args:       []string{"git://server/repo.git", "mysql+ruby~git@test.server/repo.git", "env1=test", "ruby-helloworld-sample"},
			repos:      util.StringList{"git://server/repo.git"},
			components: util.StringList{"mysql+ruby~git@test.server/repo.git", "ruby-helloworld-sample"},
			env:        util.StringList{"env1=test"},
			unknown:    []string{},
		},
	}

	for n, c := range tests {
		a := AppConfig{}
		unknown := a.AddArguments(c.args)
		if !reflect.DeepEqual(a.Environment, c.env) {
			t.Errorf("%s: Different env variables. Expected: %v, Actual: %v", n, c.env, a.Environment)
		}
		if !reflect.DeepEqual(a.SourceRepositories, c.repos) {
			t.Errorf("%s: Different source repos. Expected: %v, Actual: %v", n, c.repos, a.SourceRepositories)
		}
		if !reflect.DeepEqual(a.Components, c.components) {
			t.Errorf("%s: Different components. Expected: %v, Actual: %v", n, c.components, a.Components)
		}
		if !reflect.DeepEqual(unknown, c.unknown) {
			t.Errorf("%s: Different unknown result. Expected: %v, Actual: %v", n, c.unknown, unknown)
		}
	}

}

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
				Components: util.StringList{"one", "two", "three/four"},
			},
			componentValues:     []string{"one", "two", "three/four"},
			sourceRepoLocations: []string{},
			env:                 map[string]string{},
			parms:               map[string]string{},
		},
		"envs": {
			cfg: AppConfig{
				Environment: util.StringList{"one=first", "two=second", "three=third"},
			},
			componentValues:     []string{},
			sourceRepoLocations: []string{},
			env:                 map[string]string{"one": "first", "two": "second", "three": "third"},
			parms:               map[string]string{},
		},
		"component+source": {
			cfg: AppConfig{
				Components: util.StringList{"one~https://server/repo.git"},
			},
			componentValues:     []string{"one"},
			sourceRepoLocations: []string{"https://server/repo.git"},
			env:                 map[string]string{},
			parms:               map[string]string{},
		},
		"components+source": {
			cfg: AppConfig{
				Components: util.StringList{"mysql+ruby~git://github.com/namespace/repo.git"},
			},
			componentValues:     []string{"mysql", "ruby"},
			sourceRepoLocations: []string{"git://github.com/namespace/repo.git"},
			env:                 map[string]string{},
			parms:               map[string]string{},
		},
		"components+parms": {
			cfg: AppConfig{
				Components:         util.StringList{"ruby-helloworld-sample"},
				TemplateParameters: util.StringList{"one=first", "two=second"},
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
		c.cfg.refBuilder = &app.ReferenceBuilder{}
		cr, _, env, parms, err := c.cfg.validate()
		if err != nil {
			t.Errorf("%s: Unexpected error: %v", n, err)
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
		appCfg.refBuilder = &app.ReferenceBuilder{}
		appCfg.SetOpenShiftClient(&client.Fake{}, c.namespace)
		appCfg.AddArguments([]string{c.templateName})
		appCfg.TemplateParameters = util.StringList{}
		for k, v := range c.parms {
			appCfg.TemplateParameters.Set(fmt.Sprintf("%v=%v", k, v))
		}

		components, _, _, parms, err := appCfg.validate()
		if err != nil {
			t.Errorf("%s: Unexpected error: %v", n, err)
		}
		err = appCfg.resolve(components)
		if err != nil {
			t.Errorf("%s: Unexpected error: %v", n, err)
		}
		_, err = appCfg.buildTemplates(components, app.Environment(parms))
		if err != nil {
			t.Errorf("%s: Unexpected error: %v", n, err)
		}
		for _, component := range components {
			match := component.Input().Match
			if !match.IsTemplate() {
				t.Errorf("%s: Expected template match, got: %v", n, match)
			}
			if c.templateName != match.Name {
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

func TestEnsureHasSource(t *testing.T) {
	tests := []struct {
		name         string
		cfg          AppConfig
		components   app.ComponentReferences
		repositories []*app.SourceRepository
		expectedErr  string
	}{
		{
			name: "One requiresSource, multiple repositories",
			components: app.ComponentReferences{
				app.ComponentReference(&app.ComponentInput{
					ExpectToBuild: true,
				}),
			},
			repositories: app.MockSourceRepositories(),
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
			repositories: app.MockSourceRepositories(),
			expectedErr:  "Use '[image]~[repo]' to declare which code goes with which image",
		},
		{
			name: "One requiresSource, no repositories",
			components: app.ComponentReferences{
				app.ComponentReference(&app.ComponentInput{
					ExpectToBuild: true,
				}),
			},
			repositories: []*app.SourceRepository{},
			expectedErr:  "you must specify a repository via --code",
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
			repositories: []*app.SourceRepository{},
			expectedErr:  "you must provide at least one source code repository",
		},
		{
			name: "Successful - one repository",
			components: app.ComponentReferences{
				app.ComponentReference(&app.ComponentInput{
					ExpectToBuild: false,
				}),
			},
			repositories: []*app.SourceRepository{app.MockSourceRepositories()[0]},
			expectedErr:  "",
		},
		{
			name: "Successful - no requiresSource",
			components: app.ComponentReferences{
				app.ComponentReference(&app.ComponentInput{
					ExpectToBuild: false,
				}),
			},
			repositories: app.MockSourceRepositories(),
			expectedErr:  "",
		},
	}

	for _, test := range tests {
		err := test.cfg.ensureHasSource(test.components, test.repositories)
		if err != nil {
			if !strings.Contains(err.Error(), test.expectedErr) {
				t.Errorf("%s: Invalid error: Expected %s, got %v", test.name, test.expectedErr, err)
			}
		} else if len(test.expectedErr) != 0 {
			t.Errorf("%s: Expected %s error but got none", test.name, test.expectedErr)
		}
	}
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name        string
		cfg         AppConfig
		components  app.ComponentReferences
		expectedErr string
	}{
		{
			name: "Resolver error",
			components: app.ComponentReferences{
				app.ComponentReference(&app.ComponentInput{
					Value: "mysql:invalid",
					Resolver: app.DockerRegistryResolver{
						Client: dockerregistry.NewClient(),
					},
				})},
			expectedErr: `tag "invalid" has not been set`,
		},
		{
			name: "Successful mysql builder",
			components: app.ComponentReferences{
				app.ComponentReference(&app.ComponentInput{
					Value: "mysql",
					Match: &app.ComponentMatch{
						Builder: true,
					},
				})},
			expectedErr: "",
		},
		{
			name: "Unable to build source code",
			components: app.ComponentReferences{
				app.ComponentReference(&app.ComponentInput{
					Value:         "mysql",
					ExpectToBuild: true,
				})},
			expectedErr: "no resolver",
		},
		{
			name: "Successful docker build",
			cfg: AppConfig{
				Strategy: "docker",
			},
			components: app.ComponentReferences{
				app.ComponentReference(&app.ComponentInput{
					Value:         "mysql",
					ExpectToBuild: true,
				})},
			expectedErr: "",
		},
	}

	for _, test := range tests {
		err := test.cfg.resolve(test.components)
		if err != nil {
			if !strings.Contains(err.Error(), test.expectedErr) {
				t.Errorf("%s: Invalid error: Expected %s, got %v", test.name, test.expectedErr, err)
			}
		} else if len(test.expectedErr) != 0 {
			t.Errorf("%s: Expected %s error but got none", test.name, test.expectedErr)
		}
	}
}

func TestDetectSource(t *testing.T) {
	dockerResolver := app.DockerRegistryResolver{
		Client: dockerregistry.NewClient(),
	}
	mocks := app.MockSourceRepositories()
	tests := []struct {
		name         string
		cfg          *AppConfig
		repositories []*app.SourceRepository
		expectedLang string
		expectedErr  string
	}{
		{
			name: "detect source - ruby",
			cfg: &AppConfig{
				detector: app.SourceRepositoryEnumerator{
					Detectors: source.DefaultDetectors,
					Tester:    dockerfile.NewTester(),
				},
				dockerResolver: dockerResolver,
			},
			repositories: []*app.SourceRepository{mocks[1]},
			expectedLang: "ruby",
			expectedErr:  "",
		},
	}

	for _, test := range tests {
		err := test.cfg.detectSource(test.repositories)
		if err != nil {
			if !strings.Contains(err.Error(), test.expectedErr) {
				t.Errorf("%s: Invalid error: Expected %s, got %v", test.name, test.expectedErr, err)
			}
		} else if len(test.expectedErr) != 0 {
			t.Errorf("%s: Expected %s error but got none", test.name, test.expectedErr)
		}

		for _, repo := range test.repositories {
			info := repo.Info()
			if info == nil {
				t.Errorf("%s: expected repository info to be populated; it is nil", test.name)
				continue
			}
			if term := strings.Join(info.Terms(), ","); term != test.expectedLang {
				t.Errorf("%s: expected repository info term to be %s; got %s\n", test.name, test.expectedLang, term)
			}
		}
	}
}

func TestRunAll(t *testing.T) {
	dockerResolver := app.DockerRegistryResolver{
		Client: dockerregistry.NewClient(),
	}
	tests := []struct {
		name            string
		config          *AppConfig
		expected        map[string][]string
		expectedErr     error
		expectInsecure  util.StringSet
		expectedVolumes map[string]string
		checkPort       string
	}{
		{
			name: "successful ruby app generation",
			config: &AppConfig{
				SourceRepositories: util.StringList{"https://github.com/openshift/ruby-hello-world"},

				dockerResolver: fakeDockerResolver(),
				imageStreamResolver: app.ImageStreamResolver{
					Client:            &client.Fake{},
					ImageStreamImages: &client.Fake{},
					Namespaces:        []string{"default"},
				},
				Strategy:                        "source",
				imageStreamByAnnotationResolver: app.NewImageStreamByAnnotationResolver(&client.Fake{}, &client.Fake{}, []string{"default"}),
				templateResolver: app.TemplateResolver{
					Client: &client.Fake{},
					TemplateConfigsNamespacer: &client.Fake{},
					Namespaces:                []string{"openshift", "default"},
				},
				detector: app.SourceRepositoryEnumerator{
					Detectors: source.DefaultDetectors,
					Tester:    dockerfile.NewTester(),
				},
				typer:           kapi.Scheme,
				osclient:        &client.Fake{},
				originNamespace: "default",
			},
			expected: map[string][]string{
				"imageStream":      {"ruby-hello-world", "ruby"},
				"buildConfig":      {"ruby-hello-world"},
				"deploymentConfig": {"ruby-hello-world"},
				"service":          {"ruby-hello-world"},
			},
			expectedVolumes: nil,
			expectedErr:     nil,
		},
		{
			name: "successful docker app generation",
			config: &AppConfig{
				SourceRepositories: util.StringList{"https://github.com/openshift/ruby-hello-world"},

				dockerResolver: fakeSimpleDockerResolver(),
				imageStreamResolver: app.ImageStreamResolver{
					Client:            &client.Fake{},
					ImageStreamImages: &client.Fake{},
					Namespaces:        []string{"default"},
				},
				Strategy:                        "docker",
				imageStreamByAnnotationResolver: app.NewImageStreamByAnnotationResolver(&client.Fake{}, &client.Fake{}, []string{"default"}),
				templateResolver: app.TemplateResolver{
					Client: &client.Fake{},
					TemplateConfigsNamespacer: &client.Fake{},
					Namespaces:                []string{"openshift", "default"},
				},
				detector: app.SourceRepositoryEnumerator{
					Detectors: source.DefaultDetectors,
					Tester:    dockerfile.NewTester(),
				},
				typer:           kapi.Scheme,
				osclient:        &client.Fake{},
				originNamespace: "default",
			},
			checkPort: "8080",
			expected: map[string][]string{
				"imageStream":      {"ruby-hello-world", "ruby-20-centos7"},
				"buildConfig":      {"ruby-hello-world"},
				"deploymentConfig": {"ruby-hello-world"},
				"service":          {"ruby-hello-world"},
			},
			expectedErr: nil,
		},
		{
			name: "app generation using context dir",
			config: &AppConfig{
				SourceRepositories:              util.StringList{"https://github.com/openshift/sti-ruby"},
				ContextDir:                      "2.0/test/rack-test-app",
				dockerResolver:                  dockerResolver,
				imageStreamResolver:             fakeImageStreamResolver(),
				imageStreamByAnnotationResolver: app.NewImageStreamByAnnotationResolver(&client.Fake{}, &client.Fake{}, []string{"default"}),
				templateResolver: app.TemplateResolver{
					Client: &client.Fake{},
					TemplateConfigsNamespacer: &client.Fake{},
					Namespaces:                []string{"openshift", "default"},
				},

				detector: app.SourceRepositoryEnumerator{
					Detectors: source.DefaultDetectors,
					Tester:    dockerfile.NewTester(),
				},
				typer:           kapi.Scheme,
				osclient:        &client.Fake{},
				originNamespace: "default",
			},
			expected: map[string][]string{
				"imageStream":      {"sti-ruby"},
				"buildConfig":      {"sti-ruby"},
				"deploymentConfig": {"sti-ruby"},
				"service":          {"sti-ruby"},
			},
			expectedVolumes: nil,
			expectedErr:     nil,
		},
		{
			name: "insecure registry generation",
			config: &AppConfig{
				Components:         util.StringList{"myrepo:5000/myco/example"},
				SourceRepositories: util.StringList{"https://github.com/openshift/ruby-hello-world"},
				Strategy:           "source",
				dockerResolver: app.DockerClientResolver{
					Client: &dockertools.FakeDockerClient{
						Images: []docker.APIImages{{RepoTags: []string{"myrepo:5000/myco/example"}}},
						Image:  dockerBuilderImage(),
					},
					Insecure: true,
				},
				imageStreamResolver: app.ImageStreamResolver{
					Client:            &client.Fake{},
					ImageStreamImages: &client.Fake{},
					Namespaces:        []string{"default"},
				},
				templateResolver: app.TemplateResolver{
					Client: &client.Fake{},
					TemplateConfigsNamespacer: &client.Fake{},
					Namespaces:                []string{},
				},
				templateFileResolver: &app.TemplateFileResolver{},
				detector: app.SourceRepositoryEnumerator{
					Detectors: source.DefaultDetectors,
					Tester:    dockerfile.NewTester(),
				},
				typer:            kapi.Scheme,
				osclient:         &client.Fake{},
				originNamespace:  "default",
				InsecureRegistry: true,
			},
			expected: map[string][]string{
				"imageStream":      {"example", "ruby-hello-world"},
				"buildConfig":      {"ruby-hello-world"},
				"deploymentConfig": {"ruby-hello-world"},
				"service":          {"ruby-hello-world"},
			},
			expectedErr:     nil,
			expectedVolumes: nil,
			expectInsecure:  util.NewStringSet("example"),
		},
		{
			name: "emptyDir volumes",
			config: &AppConfig{
				DockerImages: util.StringList{"mysql"},

				dockerResolver: dockerResolver,
				imageStreamResolver: app.ImageStreamResolver{
					Client:            &client.Fake{},
					ImageStreamImages: &client.Fake{},
					Namespaces:        []string{"default"},
				},
				templateResolver: app.TemplateResolver{
					Client: &client.Fake{},
					TemplateConfigsNamespacer: &client.Fake{},
					Namespaces:                []string{"openshift", "default"},
				},

				detector: app.SourceRepositoryEnumerator{
					Detectors: source.DefaultDetectors,
					Tester:    dockerfile.NewTester(),
				},
				typer:           kapi.Scheme,
				osclient:        &client.Fake{},
				originNamespace: "default",
			},

			expected: map[string][]string{
				"imageStream":      {"mysql"},
				"deploymentConfig": {"mysql"},
				"service":          {"mysql"},
				"volumeMounts":     {"mysql-volume-1"},
			},
			expectedVolumes: map[string]string{
				"mysql-volume-1": "EmptyDir",
			},
			expectedErr: nil,
		},
		{
			name: "Docker build",
			config: &AppConfig{
				SourceRepositories: util.StringList{"https://github.com/openshift/ruby-hello-world"},

				dockerResolver: app.DockerClientResolver{
					Client: &dockertools.FakeDockerClient{
						Images: []docker.APIImages{{RepoTags: []string{"openshift/ruby-20-centos7"}}},
						Image:  dockerBuilderImage(),
					},
					Insecure: true,
				},
				imageStreamResolver: app.ImageStreamResolver{
					Client:            &client.Fake{},
					ImageStreamImages: &client.Fake{},
					Namespaces:        []string{"default"},
				},
				imageStreamByAnnotationResolver: app.NewImageStreamByAnnotationResolver(&client.Fake{}, &client.Fake{}, []string{"default"}),
				templateResolver: app.TemplateResolver{
					Client: &client.Fake{},
					TemplateConfigsNamespacer: &client.Fake{},
					Namespaces:                []string{"openshift", "default"},
				},
				detector: app.SourceRepositoryEnumerator{
					Detectors: source.DefaultDetectors,
					Tester:    dockerfile.NewTester(),
				},
				typer:           kapi.Scheme,
				osclient:        &client.Fake{},
				originNamespace: "default",
			},
			expected: map[string][]string{
				"imageStream":      {"ruby-hello-world", "ruby-20-centos7"},
				"buildConfig":      {"ruby-hello-world"},
				"deploymentConfig": {"ruby-hello-world"},
				"service":          {"ruby-hello-world"},
			},
			expectedErr: nil,
		},
	}

	for _, test := range tests {
		test.config.refBuilder = &app.ReferenceBuilder{}
		res, err := test.config.RunAll(os.Stdout, os.Stderr)
		if err != test.expectedErr {
			t.Errorf("%s: Error mismatch! Expected %v, got %v", test.name, test.expectedErr, err)
			continue
		}
		imageStreams := []*imageapi.ImageStream{}
		got := map[string][]string{}
		gotVolumes := map[string]string{}
		for _, obj := range res.List.Items {
			switch tp := obj.(type) {
			case *buildapi.BuildConfig:
				got["buildConfig"] = append(got["buildConfig"], tp.Name)
			case *kapi.Service:
				if test.checkPort != "" {
					if len(tp.Spec.Ports) == 0 {
						t.Errorf("%s: did not get any ports in service")
						break
					}
					expectedPort, _ := strconv.Atoi(test.checkPort)
					if tp.Spec.Ports[0].Port != expectedPort {
						t.Errorf("%s: did not get expected port in service. Expected: %d. Got %d\n", expectedPort, tp.Spec.Ports[0].Port)
					}
				}
				got["service"] = append(got["service"], tp.Name)
			case *imageapi.ImageStream:
				got["imageStream"] = append(got["imageStream"], tp.Name)
				imageStreams = append(imageStreams, tp)
			case *deploy.DeploymentConfig:
				got["deploymentConfig"] = append(got["deploymentConfig"], tp.Name)
				if podTemplate := tp.Template.ControllerTemplate.Template; podTemplate != nil {
					for _, volume := range podTemplate.Spec.Volumes {
						if volume.VolumeSource.EmptyDir != nil {
							gotVolumes[volume.Name] = "EmptyDir"
						} else {
							gotVolumes[volume.Name] = "UNKNOWN"
						}
					}
					for _, container := range podTemplate.Spec.Containers {
						for _, volumeMount := range container.VolumeMounts {
							got["volumeMounts"] = append(got["volumeMounts"], volumeMount.Name)
						}
					}
				}
			}
		}

		if len(test.expected) != len(got) {
			t.Errorf("%s: Resource kind size mismatch! Expected %d, got %d", test.name, len(test.expected), len(got))
			continue
		}
		for k, exp := range test.expected {
			g, ok := got[k]
			if !ok {
				t.Errorf("%s: Didn't find expected kind %s", test.name, k)
			}

			sort.Strings(g)
			sort.Strings(exp)

			if !reflect.DeepEqual(g, exp) {
				t.Errorf("%s: %s resource names mismatch! Expected %v, got %v", test.name, k, exp, g)
				continue
			}
		}

		if len(test.expectedVolumes) != len(gotVolumes) {
			t.Errorf("%s: Volume count mismatch! Expected %d, got %d", test.name, len(test.expectedVolumes), len(gotVolumes))
			continue
		}
		for k, exp := range test.expectedVolumes {
			g, ok := gotVolumes[k]
			if !ok {
				t.Errorf("%s: Didn't find expected volume %s", test.name, k)
			}

			if g != exp {
				t.Errorf("%s: Expected volume of type %s, got %s", test.name, g, exp)
			}
		}

		if test.expectInsecure == nil {
			continue
		}
		for _, stream := range imageStreams {
			_, hasAnnotation := stream.Annotations[imageapi.InsecureRepositoryAnnotation]
			if test.expectInsecure.Has(stream.Name) && !hasAnnotation {
				t.Errorf("%s: Expected insecure annotation for stream: %s, but did not get one.", test.name, stream.Name)
			}
			if !test.expectInsecure.Has(stream.Name) && hasAnnotation {
				t.Errorf("%s: Got insecure annotation for stream: %s, and was not expecting one.", test.name, stream.Name)
			}
		}

	}
}

func TestRunBuild(t *testing.T) {
	dockerResolver := app.DockerRegistryResolver{
		Client: dockerregistry.NewClient(),
	}

	tests := []struct {
		name        string
		config      *AppConfig
		expected    map[string][]string
		expectedErr error
	}{
		{
			name: "successful ruby app generation",
			config: &AppConfig{
				SourceRepositories: util.StringList{"https://github.com/openshift/ruby-hello-world"},
				DockerImages:       util.StringList{"openshift/ruby-20-centos7", "openshift/mongodb-24-centos7"},
				OutputDocker:       true,

				dockerResolver: dockerResolver,
				imageStreamResolver: app.ImageStreamResolver{
					Client:            &client.Fake{},
					ImageStreamImages: &client.Fake{},
					Namespaces:        []string{"default"},
				},
				imageStreamByAnnotationResolver: &app.ImageStreamByAnnotationResolver{
					Client:            &client.Fake{},
					ImageStreamImages: &client.Fake{},
					Namespaces:        []string{"default"},
				},
				templateResolver: app.TemplateResolver{
					Client: &client.Fake{},
					TemplateConfigsNamespacer: &client.Fake{},
					Namespaces:                []string{"openshift", "default"},
				},

				detector: app.SourceRepositoryEnumerator{
					Detectors: source.DefaultDetectors,
					Tester:    dockerfile.NewTester(),
				},
				typer:           kapi.Scheme,
				osclient:        &client.Fake{},
				originNamespace: "default",
			},
			expected: map[string][]string{
				"buildConfig": {"ruby-hello-world"},
				"imageStream": {"ruby-20-centos7"},
			},
			expectedErr: nil,
		},
	}

	for _, test := range tests {
		test.config.refBuilder = &app.ReferenceBuilder{}
		res, err := test.config.RunBuilds(os.Stdout, os.Stderr)
		if err != test.expectedErr {
			t.Errorf("%s: Error mismatch! Expected %v, got %v", test.name, test.expectedErr, err)
			continue
		}
		got := map[string][]string{}
		for _, obj := range res.List.Items {
			switch tp := obj.(type) {
			case *buildapi.BuildConfig:
				got["buildConfig"] = append(got["buildConfig"], tp.Name)
			case *imageapi.ImageStream:
				got["imageStream"] = append(got["imageStream"], tp.Name)
			}
		}

		if len(test.expected) != len(got) {
			t.Errorf("%s: Resource kind size mismatch! Expected %d, got %d", test.name, len(test.expected), len(got))
			continue
		}

		for k, exp := range test.expected {
			g, ok := got[k]
			if !ok {
				t.Errorf("%s: Didn't find expected kind %s", test.name, k)
			}

			sort.Strings(g)
			sort.Strings(exp)

			if !reflect.DeepEqual(g, exp) {
				t.Errorf("%s: Resource names mismatch! Expected %v, got %v", test.name, exp, g)
				continue
			}
		}
	}
}

func TestEnsureValidUniqueName(t *testing.T) {
	chars := []byte("abcdefghijk")
	longBytes := []byte{}
	for i := 0; i < (util.DNS1123SubdomainMaxLength + 20); i++ {
		longBytes = append(longBytes, chars[i%len(chars)])
	}
	longName := string(longBytes)
	tests := []struct {
		name        string
		input       []string
		expected    []string
		expectError bool
	}{
		{
			name:     "duplicate names",
			input:    []string{"one", "two", "three", "one", "one", "two"},
			expected: []string{"one", "two", "three", "one-1", "one-2", "two-1"},
		},
		{
			name:     "mixed case names",
			input:    []string{"One", "ONE", "tWo"},
			expected: []string{"one", "one-1", "two"},
		},
		{
			name:        "short name",
			input:       []string{"t"},
			expectError: true,
		},
		{
			name:  "long name",
			input: []string{longName, longName, longName},
			expected: []string{longName[:util.DNS1123SubdomainMaxLength],
				namer.GetName(longName[:util.DNS1123SubdomainMaxLength], "1", util.DNS1123SubdomainMaxLength),
				namer.GetName(longName[:util.DNS1123SubdomainMaxLength], "2", util.DNS1123SubdomainMaxLength),
			},
		},
	}

tests:
	for _, test := range tests {
		result := []string{}
		names := make(map[string]int)
		for _, i := range test.input {
			name, err := ensureValidUniqueName(names, i)
			if err != nil && !test.expectError {
				t.Errorf("%s: unexpected error: %v", test.name, err)
			}
			if err == nil && test.expectError {
				t.Errorf("%s: did not get an error.", test.name)
			}
			if err != nil {
				continue tests
			}
			result = append(result, name)
		}
		if !reflect.DeepEqual(result, test.expected) {
			t.Errorf("%s: unexpected output. Expected: %#v, Got: %#v", test.name, test.expected, result)
		}
	}
}

// Make sure that buildPipelines defaults DockerImage.Config if needed to
// avoid a nil panic.
func TestBuildPipelinesWithUnresolvedImage(t *testing.T) {
	dockerParser := dockerfile.NewParser()

	dockerFileInput := strings.NewReader("EXPOSE 1234\nEXPOSE 4567")
	dockerFile, err := dockerParser.Parse(dockerFileInput)
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
			Match: &app.ComponentMatch{
				Value: "mysql",
			},
		}),
	}

	a := AppConfig{}
	group, err := a.buildPipelines(refs, app.Environment{})
	if err != nil {
		t.Error(err)
	}

	expectedPorts := util.NewStringSet("1234", "4567")
	actualPorts := util.NewStringSet()
	for port := range group[0].InputImage.Info.Config.ExposedPorts {
		actualPorts.Insert(port)
	}
	if e, a := expectedPorts.List(), actualPorts.List(); !reflect.DeepEqual(e, a) {
		t.Errorf("Expected ports=%v, got %v", e, a)
	}
}

func builderImageStream() *imageapi.ImageStream {
	return &imageapi.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Name:            "ruby-builder",
			ResourceVersion: "1",
		},
		Status: imageapi.ImageStreamStatus{
			Tags: map[string]imageapi.TagEventList{
				"latest": {
					Items: []imageapi.TagEvent{
						{
							Image: "the-image-id",
						},
					},
				},
			},
			DockerImageRepository: "example/ruby:latest",
		},
	}

}

func builderImage() *imageapi.ImageStreamImage {
	return &imageapi.ImageStreamImage{
		Image: imageapi.Image{
			DockerImageReference: "example/ruby:latest",
			DockerImageMetadata: imageapi.DockerImage{
				Config: &imageapi.DockerConfig{
					Env: []string{
						"STI_SCRIPTS_URL=http://repo/git/ruby",
					},
					ExposedPorts: map[string]struct{}{
						"8080/tcp": {},
					},
				},
			},
		},
	}
}
func dockerBuilderImage() *docker.Image {
	return &docker.Image{
		ID: "ruby",
		Config: &docker.Config{
			Env: []string{
				"STI_SCRIPTS_URL=http://repo/git/ruby",
			},
			ExposedPorts: map[docker.Port]struct{}{
				"8080/tcp": {},
			},
		},
	}
}

func fakeImageStreamResolver() app.Resolver {
	client := &client.Fake{
		ReactFn: func(action testclient.FakeAction) (runtime.Object, error) {
			switch action.Action {
			case "get-imagestream":
				return builderImageStream(), nil
			case "get-imagestream-image":
				return builderImage(), nil
			}
			return nil, nil
		},
	}
	return app.ImageStreamResolver{
		Client:            client,
		ImageStreamImages: client,
		Namespaces:        []string{"default"},
	}
}

func fakeDockerResolver() app.Resolver {
	return app.DockerClientResolver{
		Client: &dockertools.FakeDockerClient{
			Images: []docker.APIImages{{RepoTags: []string{"library/ruby:latest"}}},
			Image:  dockerBuilderImage(),
		},
		Insecure: true,
	}
}

func fakeSimpleDockerResolver() app.Resolver {
	return app.DockerClientResolver{
		Client: &dockertools.FakeDockerClient{
			Images: []docker.APIImages{{RepoTags: []string{"openshift/ruby-20-centos7"}}},
			Image: &docker.Image{
				ID: "ruby",
				Config: &docker.Config{
					Env: []string{},
				},
			},
		},
	}
}
