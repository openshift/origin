package generate

import (
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/fsouza/go-dockerclient"
	"github.com/spf13/cobra"

	buildapi "github.com/openshift/origin/pkg/build/api"
	osclient "github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	genapp "github.com/openshift/origin/pkg/generate/app"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func actualExpected(actual interface{}, expected interface{}) string {
	actualJSON, e1 := json.Marshal(actual)
	expectedJSON, e2 := json.Marshal(expected)
	if e1 != nil || e2 != nil {
		return fmt.Sprintf("\nActual:\n%#v\n\nExpected:\n%#v\n", actual, expected)
	}
	return fmt.Sprintf("\nActual:\n%s\n\nExpected:\n%s\n\n", string(actualJSON), string(expectedJSON))
}

func TestGetSource(t *testing.T) {
	testURL := "https://test.repo/myproject/test.git"
	fakeGetDir := func() (string, error) {
		return "current_dir", nil
	}
	fakeGetDirErr := func() (string, error) {
		return "", fmt.Errorf("error")
	}
	tests := []struct {
		name        string
		url         string
		args        []string
		expectedDir string
		expectedURL string
		getDir      getDirFunc
		expectedErr bool
	}{
		{
			name:        "url specified",
			url:         testURL,
			args:        []string{},
			expectedDir: "",
			expectedURL: testURL,
			getDir:      fakeGetDir,
		},
		{
			name:        "argument directory",
			url:         "",
			args:        []string{"./test/dir"},
			expectedDir: "./test/dir",
			expectedURL: "",
			getDir:      fakeGetDir,
		},
		{
			name:        "argument url",
			url:         "",
			args:        []string{testURL},
			expectedDir: "",
			expectedURL: testURL,
			getDir:      fakeGetDir,
		},
		{
			name:        "no argument - cwd",
			url:         "",
			args:        []string{},
			expectedDir: "current_dir",
			expectedURL: "",
			getDir:      fakeGetDir,
		},
		{
			name:        "url specified and directory argument",
			url:         testURL,
			args:        []string{"test/dir"},
			expectedDir: "",
			expectedURL: "",
			getDir:      fakeGetDir,
			expectedErr: true,
		},
		{
			name:        "url specified and url argument",
			url:         testURL,
			args:        []string{testURL},
			expectedDir: "",
			expectedURL: "",
			getDir:      fakeGetDir,
			expectedErr: true,
		},
		{
			name:        "error from getcwd func",
			url:         "",
			args:        []string{""},
			expectedDir: "",
			expectedURL: "",
			getDir:      fakeGetDirErr,
			expectedErr: true,
		},
	}

	for _, test := range tests {
		dir, url, err := getSource(test.url, test.args, test.getDir)
		if err != nil && !test.expectedErr {
			t.Errorf("%s: unexpected error: %v", test.name, err)
			continue
		}
		if err == nil && test.expectedErr {
			t.Errorf("%s: expected error, but got none. Results: %v, %v, %v", test.name, dir, url, err)
			continue
		}
		if dir != test.expectedDir {
			t.Errorf("%s: unexpected directory. Got: %s, Expected: %s", test.name, dir, test.expectedDir)
		}
		if url != test.expectedURL {
			t.Errorf("%s: unexpected URL. Got: %s, Expected: %s", test.name, url, test.expectedURL)
		}
	}
}

func TestSetDefaultPrinter(t *testing.T) {
	testValues := []string{"yaml", ""}
	expected := []string{"yaml", "json"}
	for i := range testValues {
		var value string
		c := &cobra.Command{}
		flags := c.Flags()
		flags.StringVarP(&value, "output", "o", "", "Default output")
		flag := flags.Lookup("output")
		flag.Value.Set(testValues[i])
		setDefaultPrinter(c)
		if value != expected[i] {
			t.Errorf("Unexpected flag value after call to setDefaultPrinter. "+
				"Value: %s. Expected: %s", value, expected[i])
		}
	}
}

func TestGetResolver(t *testing.T) {
	// TODO: Assign global default namespace when available
	defaultns := "default"
	tests := []struct {
		name                  string
		namespace             string
		osClientNull          bool
		dockerClientNull      bool
		resolverCount         int
		imageStreamIndex      int
		imageStreamNamespaces []string
	}{
		{
			name:                  "all clients",
			namespace:             "myns",
			osClientNull:          false,
			dockerClientNull:      false,
			resolverCount:         3,
			imageStreamIndex:      1,
			imageStreamNamespaces: []string{"myns", defaultns},
		},
		{
			name:             "no os client",
			namespace:        "testns",
			osClientNull:     true,
			dockerClientNull: false,
			resolverCount:    2,
		},
		{
			name:                  "no docker client",
			namespace:             "testns",
			osClientNull:          false,
			dockerClientNull:      true,
			resolverCount:         2,
			imageStreamIndex:      0,
			imageStreamNamespaces: []string{"testns", defaultns},
		},
		{
			name:             "no clients",
			osClientNull:     true,
			dockerClientNull: true,
			resolverCount:    1,
		},
		{
			name:                  "no namespace",
			osClientNull:          false,
			dockerClientNull:      false,
			resolverCount:         3,
			imageStreamIndex:      1,
			imageStreamNamespaces: []string{defaultns},
		},
	}

	for _, test := range tests {
		var osClient osclient.Interface
		var dockerClient *docker.Client

		if !test.osClientNull {
			osClient = &osclient.Fake{}
		}
		if !test.dockerClientNull {
			dockerClient = &docker.Client{}
		}
		resolver := getResolver(test.namespace, osClient, dockerClient)
		weightedResolver := resolver.(genapp.PerfectMatchWeightedResolver)
		if len(weightedResolver) != test.resolverCount {
			t.Errorf("%s: unexpected number of resolvers: %d. Expected: %d", test.name, len(weightedResolver), test.resolverCount)
			continue
		}
		if test.osClientNull {
			continue
		}
		imageStreamResolver, ok := weightedResolver[test.imageStreamIndex].Resolver.(*genapp.ImageStreamResolver)
		if !ok {
			t.Errorf("%s: ImageStreamResolver not found at expected index (%d). Found: %#v",
				test.name, test.imageStreamIndex, weightedResolver[test.imageStreamIndex].Resolver)
			continue
		}
		if !reflect.DeepEqual(imageStreamResolver.Namespaces, test.imageStreamNamespaces) {
			t.Errorf("%s: did not find expected namespaces: %#v. Got: %#v",
				test.name, test.imageStreamNamespaces, imageStreamResolver.Namespaces)
		}
	}
}

func mapsEqual(map1, map2 map[string]string) bool {
	if len(map1) != len(map2) {
		return false
	}
	for key, value := range map1 {
		value2, ok := map2[key]
		if !ok {
			return false
		}
		if value != value2 {
			return false
		}
	}
	return true
}

func TestGetEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
		err      bool
	}{
		{
			name:     "empty input",
			input:    "",
			expected: map[string]string{},
			err:      false,
		},
		{
			name:     "single var",
			input:    "env1var=testing var",
			expected: map[string]string{"env1var": "testing var"},
			err:      false,
		},
		{
			name:     "multiple vars",
			input:    "env1var=testing1,env2var=testing2,env3var=this is a test",
			expected: map[string]string{"env1var": "testing1", "env2var": "testing2", "env3var": "this is a test"},
			err:      false,
		},
		{
			name:  "sequential commas",
			input: ",,,,test1=value1,,,",
			err:   true,
		},
		{
			name:  "errors",
			input: "env1var==,=env2var=testing2",
			err:   true,
		},
	}
	for _, test := range tests {
		result, err := getEnvironment(test.input)
		if err != nil && !test.err {
			t.Errorf("%s: unexpected error: %v", test.name, err)
			continue
		}
		if err == nil && test.err {
			t.Errorf("%s: did not get an error. Expected error", test.name)
			continue
		}
		if !mapsEqual(map[string]string(result), test.expected) {
			t.Errorf("%s: unexpected result. %s", test.name, actualExpected(result, test.expected))
		}
	}
}

type fakeSourceRefGen bool

func (f fakeSourceRefGen) FromGitURL(param string) (*genapp.SourceRef, error) {
	if f {
		return nil, fmt.Errorf("Error")
	}
	u, _ := url.Parse(param)
	return &genapp.SourceRef{
		URL: u,
	}, nil
}

func (f fakeSourceRefGen) FromDirectory(dir string) (*genapp.SourceRef, error) {
	if f {
		return nil, fmt.Errorf("Error")
	}
	return &genapp.SourceRef{
		Dir: dir,
	}, nil
}

type fakeSourceRefGenWithSourceRef map[string]*genapp.SourceRef

func (f fakeSourceRefGenWithSourceRef) FromGitURL(param string) (*genapp.SourceRef, error) {
	return f["gitURL"], nil
}

func (f fakeSourceRefGenWithSourceRef) FromDirectory(dir string) (*genapp.SourceRef, error) {
	return f["directory"], nil
}

func TestGenerateSourceRef(t *testing.T) {
	sourceURLStr := "https://test.server/repository/project.git"
	sourceURL, _ := url.Parse(sourceURLStr)

	tests := []struct {
		name        string
		input       params
		expected    genapp.SourceRef
		genError    bool
		errExpected bool
	}{
		{
			name: "url specified",
			input: params{
				sourceURL: sourceURLStr,
			},
			expected: genapp.SourceRef{
				URL: sourceURL,
			},
		},
		{
			name: "directory specified",
			input: params{
				sourceDir: "test/dir",
			},
			expected: genapp.SourceRef{
				Dir: "test/dir",
			},
		},
		{
			name: "url specified, reference specified",
			input: params{
				sourceURL: sourceURLStr,
				sourceRef: "mySourceRef",
			},
			expected: genapp.SourceRef{
				URL: sourceURL,
				Ref: "mySourceRef",
			},
		},
		{
			name: "url specified, error generating",
			input: params{
				sourceURL: sourceURLStr,
			},
			genError:    true,
			errExpected: true,
		},
		{
			name: "directory specified, error generating",
			input: params{
				sourceDir: "test/dir",
			},
			genError:    true,
			errExpected: true,
		},
	}
	for _, test := range tests {
		g := appGenerator{
			input:     test.input,
			srcRefGen: fakeSourceRefGen(test.genError),
		}
		srcRef, err := g.generateSourceRef()
		if err != nil && !test.errExpected {
			t.Errorf("%s: unexpected error: %v", test.name, err)
			continue
		}
		if err == nil && test.errExpected {
			t.Errorf("%s: got no error. Error expected.", test.name)
			continue
		}
		if test.errExpected {
			continue
		}
		if !reflect.DeepEqual(*srcRef, test.expected) {
			t.Errorf("%s: did not get expected srcref. %s", test.name, actualExpected(*srcRef, test.expected))
		}
	}
}

type fakeStrategyRefGen bool

func (f fakeStrategyRefGen) FromSourceRefAndDockerContext(srcRef *genapp.SourceRef, dockerContext string) (*genapp.BuildStrategyRef, error) {
	if f {
		return nil, fmt.Errorf("Error")
	}
	return &genapp.BuildStrategyRef{
		IsDockerBuild: true,
		Base: &genapp.ImageRef{
			DockerImageReference: imageapi.DockerImageReference{
				Name: "DockerContextBuild",
			},
			Info: &imageapi.DockerImage{},
		},
	}, nil
}

func (f fakeStrategyRefGen) FromSTIBuilderImage(builderRef *genapp.ImageRef) (*genapp.BuildStrategyRef, error) {
	if f {
		return nil, fmt.Errorf("Error")
	}
	return &genapp.BuildStrategyRef{
		IsDockerBuild: false,
		Base:          builderRef,
	}, nil
}

func (f fakeStrategyRefGen) FromSourceRef(srcRef *genapp.SourceRef) (*genapp.BuildStrategyRef, error) {
	if f {
		return nil, fmt.Errorf("Error")
	}
	return &genapp.BuildStrategyRef{
		IsDockerBuild: true,
		Base: &genapp.ImageRef{
			DockerImageReference: imageapi.DockerImageReference{
				Name: "SourceRefBuild",
			},
			Info: &imageapi.DockerImage{},
		},
	}, nil

}

type fakeStrategyRefGenWithStrategy map[string]*genapp.BuildStrategyRef

func (s fakeStrategyRefGenWithStrategy) FromSourceRefAndDockerContext(srcRef *genapp.SourceRef, dockerContext string) (*genapp.BuildStrategyRef, error) {
	srcRef.ContextDir = dockerContext
	return s["sourceAndDockerContext"], nil
}

func (s fakeStrategyRefGenWithStrategy) FromSTIBuilderImage(builderRef *genapp.ImageRef) (*genapp.BuildStrategyRef, error) {
	s["stiBuilderImage"].Base = builderRef
	return s["stiBuilderImage"], nil
}

func (s fakeStrategyRefGenWithStrategy) FromSourceRef(srcRef *genapp.SourceRef) (*genapp.BuildStrategyRef, error) {
	return s["sourceRef"], nil
}

type fakeImageRefGen bool

func (f fakeImageRefGen) FromNameAndResolver(builderImage string, resolver genapp.Resolver) (*genapp.ImageRef, error) {
	if f {
		return nil, fmt.Errorf("Error")
	}
	return &genapp.ImageRef{
		DockerImageReference: imageapi.DockerImageReference{
			Name: builderImage,
		},
		Info: &imageapi.DockerImage{},
	}, nil
}

type fakeImageRefGenWithImage genapp.ImageRef

func (i *fakeImageRefGenWithImage) FromNameAndResolver(builderImage string, resolver genapp.Resolver) (*genapp.ImageRef, error) {
	return (*genapp.ImageRef)(i), nil
}

func TestGenerateBuildStrategyRef(t *testing.T) {
	tests := []struct {
		name              string
		input             params
		expected          genapp.BuildStrategyRef
		errorExpected     bool
		strategyRefGenErr bool
		imageRefGenErr    bool
	}{
		{
			name: "docker context specified",
			input: params{
				dockerContext: "context",
			},
			expected: genapp.BuildStrategyRef{
				IsDockerBuild: true,
				Base: &genapp.ImageRef{
					DockerImageReference: imageapi.DockerImageReference{
						Name: "DockerContextBuild",
					},
				},
			},
		},
		{
			name: "docker context specified error",
			input: params{
				dockerContext: "context",
			},
			errorExpected:     true,
			strategyRefGenErr: true,
		},
		{
			name: "builder image specified",
			input: params{
				builderImage: "mybuilder",
			},
			expected: genapp.BuildStrategyRef{
				IsDockerBuild: false,
				Base: &genapp.ImageRef{
					DockerImageReference: imageapi.DockerImageReference{
						Name: "mybuilder",
					},
				},
			},
		},
		{
			name: "builder image specified strategyref error",
			input: params{
				builderImage: "mybuilder",
			},
			strategyRefGenErr: true,
			errorExpected:     true,
		},
		{
			name: "builder image specified imagerefgen error",
			input: params{
				builderImage: "mybuilder",
			},
			imageRefGenErr: true,
			errorExpected:  true,
		},
		{
			name:  "source detection",
			input: params{},
			expected: genapp.BuildStrategyRef{
				IsDockerBuild: true,
				Base: &genapp.ImageRef{
					DockerImageReference: imageapi.DockerImageReference{
						Name: "SourceRefBuild",
					},
				},
			},
		},
		{
			name:              "source detection error",
			input:             params{},
			strategyRefGenErr: true,
			errorExpected:     true,
		},
		{
			name:  "port specified",
			input: params{port: "80"},
			expected: genapp.BuildStrategyRef{
				IsDockerBuild: true,
				Base: &genapp.ImageRef{
					DockerImageReference: imageapi.DockerImageReference{
						Name: "SourceRefBuild",
					},
					Info: &imageapi.DockerImage{
						Config: imageapi.DockerConfig{
							ExposedPorts: map[string]struct{}{"80": {}},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		g := appGenerator{
			input:          test.input,
			strategyRefGen: fakeStrategyRefGen(test.strategyRefGenErr),
			imageRefGen:    fakeImageRefGen(test.imageRefGenErr),
		}
		output, err := g.generateBuildStrategyRef(&genapp.SourceRef{})
		if err != nil && !test.errorExpected {
			t.Errorf("%s: unexpected error: %v", test.name, err)
			continue
		}
		if err == nil && test.errorExpected {
			t.Errorf("%s: got no error. Error expected", test.name)
			continue
		}
		if test.errorExpected {
			continue
		}
		if output.IsDockerBuild != test.expected.IsDockerBuild {
			t.Errorf("%s: did not get expected build type. %s",
				test.name, actualExpected(output.IsDockerBuild, test.expected.IsDockerBuild))
		}
		if output.Base.Name != test.expected.Base.Name {
			t.Errorf("%s: did not get expected base image name. %s",
				test.name, actualExpected(output.Base.Name, test.expected.Base.Name))
		}
		if test.expected.Base.Info != nil {
			if !reflect.DeepEqual(output.Base.Info, test.expected.Base.Info) {
				t.Errorf("%s: did not get expected image info. %s",
					test.name, actualExpected(output.Base.Info, test.expected.Base.Info))
			}
		}
	}
}

func TestGeneratePipeline(t *testing.T) {
	sourceURL, _ := url.Parse("https://test.server/repository/code.git")
	srcRef := genapp.SourceRef{
		Name: "source",
		URL:  sourceURL,
		Ref:  "ref",
	}
	strategyRef := genapp.BuildStrategyRef{
		IsDockerBuild: false,
		Base: &genapp.ImageRef{
			DockerImageReference: imageapi.DockerImageReference{
				Name: "Builder",
			},
			Info: &imageapi.DockerImage{
				Config: imageapi.DockerConfig{
					ExposedPorts: map[string]struct{}{"80": {}},
				},
			},
		},
	}

	g := appGenerator{
		input: params{
			env: genapp.Environment{"key1": "value1"},
		},
	}

	pipeline, err := g.generatePipeline(&srcRef, &strategyRef)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if pipeline.From != "source" {
		t.Errorf("Unexpected pipeline from: %v", pipeline.From)
	}
	if pipeline.InputImage.Name != "Builder" {
		t.Errorf("Unexpected pipeline input image: %#v", pipeline.InputImage)
	}
	if pipeline.Image.Name != "source" {
		t.Errorf("Unexpected pipeline output image: %#v", pipeline.Image)
	}
	if _, ok := pipeline.Image.Info.Config.ExposedPorts["80"]; !ok {
		t.Errorf("Pipeline output image does not contain expected port: %#v", pipeline.Image)
	}
	if _, ok := pipeline.Deployment.Env["key1"]; !ok {
		t.Errorf("Pipeline deployment does not contain expected environment variable: %#v", pipeline.Deployment)
	}
}

type expectedObjects struct {
	services          map[string]kapi.Service
	buildConfig       buildapi.BuildConfig
	deploymentConfig  deployapi.DeploymentConfig
	imageRepositories map[string]imageapi.ImageRepository
}

func expectedBuildTriggers() []buildapi.BuildTriggerPolicy {
	return []buildapi.BuildTriggerPolicy{
		{
			Type: buildapi.GithubWebHookBuildTriggerType,
			GithubWebHook: &buildapi.WebHookTrigger{
				Secret: "asecret",
			},
		},
		{
			Type: buildapi.GenericWebHookBuildTriggerType,
			GenericWebHook: &buildapi.WebHookTrigger{
				Secret: "asecret",
			},
		},
	}
}

func expectedDeploymentTriggers(name string) []deployapi.DeploymentTriggerPolicy {
	return []deployapi.DeploymentTriggerPolicy{
		{
			Type: deployapi.DeploymentTriggerOnConfigChange,
		},
		{
			Type: deployapi.DeploymentTriggerOnImageChange,
			ImageChangeParams: &deployapi.DeploymentTriggerImageChangeParams{
				Automatic:      true,
				ContainerNames: []string{name},
				From:           kapi.ObjectReference{Name: name},
				Tag:            "latest",
			},
		},
	}
}

func expectedService(name string, port int, proto kapi.Protocol) map[string]kapi.Service {
	return map[string]kapi.Service{
		name: {
			ObjectMeta: kapi.ObjectMeta{
				Name: name,
			},
			Spec: kapi.ServiceSpec{
				Port:          port,
				Protocol:      proto,
				ContainerPort: util.NewIntOrStringFromInt(port),
				Selector: map[string]string{
					"deploymentconfig": name,
				},
			},
		},
	}
}

func expectedBuildConfig(name, uri, ref, contextDir, stiBuilder string) buildapi.BuildConfig {
	var strategy buildapi.BuildStrategy
	if len(stiBuilder) > 0 {
		strategy = buildapi.BuildStrategy{
			Type: buildapi.STIBuildStrategyType,
			STIStrategy: &buildapi.STIBuildStrategy{
				Image: stiBuilder,
			},
		}
	} else {
		strategy = buildapi.BuildStrategy{
			Type: buildapi.DockerBuildStrategyType,
		}
	}
	return buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
		Triggers: expectedBuildTriggers(),
		Parameters: buildapi.BuildParameters{
			Source: buildapi.BuildSource{
				Type: buildapi.BuildSourceGit,
				Git: &buildapi.GitBuildSource{
					URI: uri,
					Ref: ref,
				},
				ContextDir: contextDir,
			},
			Strategy: strategy,
			Output: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Name: name,
				},
			},
		},
	}

}

func portName(name string, port int, proto kapi.Protocol) string {
	protoString := strings.ToLower(string(proto))
	return fmt.Sprintf("%s-%s-%d", name, protoString, port)
}

func expectedDeploymentConfig(name string, port int, proto kapi.Protocol, environment genapp.Environment) deployapi.DeploymentConfig {
	var env []kapi.EnvVar
	if environment != nil {
		for k, v := range environment {
			env = append(env, kapi.EnvVar{Name: k, Value: v})
		}
	}
	return deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
		Triggers: expectedDeploymentTriggers(name),
		Template: deployapi.DeploymentTemplate{
			Strategy: deployapi.DeploymentStrategy{
				Type: deployapi.DeploymentStrategyTypeRecreate,
			},
			ControllerTemplate: kapi.ReplicationControllerSpec{
				Replicas: 1,
				Selector: map[string]string{"deploymentconfig": name},
				Template: &kapi.PodTemplateSpec{
					Spec: kapi.PodSpec{
						Containers: []kapi.Container{
							{
								Name:  name,
								Image: "library/" + name + ":latest",
								Ports: []kapi.Port{
									{
										Name:          portName(name, port, proto),
										ContainerPort: port,
										Protocol:      proto,
									},
								},
								Env: env,
							},
						},
					},
				},
			},
		},
	}
}

func expectedImageRepos(names []string) map[string]imageapi.ImageRepository {
	result := make(map[string]imageapi.ImageRepository)
	for _, name := range names {
		result[name] = imageapi.ImageRepository{
			ObjectMeta: kapi.ObjectMeta{
				Name: name,
			},
		}
	}
	return result

}

func mustParse(urlString string) *url.URL {
	u, err := url.Parse(urlString)
	if err != nil {
		panic(err)
	}
	return u
}

func TestRun(t *testing.T) {
	tests := []struct {
		name     string
		g        appGenerator
		expected expectedObjects
	}{

		{
			name: "source repo with dockerfile",
			g: appGenerator{
				input: params{
					sourceDir: "/path/to/source",
				},
				srcRefGen: fakeSourceRefGenWithSourceRef{
					"directory": &genapp.SourceRef{
						URL: mustParse("https://git.server/namespace/myproject.git"),
					},
				},
				strategyRefGen: fakeStrategyRefGenWithStrategy{
					"sourceRef": &genapp.BuildStrategyRef{
						IsDockerBuild: true,
						Base: &genapp.ImageRef{
							DockerImageReference: imageapi.DockerImageReference{
								Name: "parent",
							},
							Info: &imageapi.DockerImage{
								Config: imageapi.DockerConfig{
									ExposedPorts: map[string]struct{}{"8080/tcp": {}},
								},
							},
						},
					},
				},
			},
			expected: expectedObjects{
				services:          expectedService("myproject", 8080, kapi.ProtocolTCP),
				buildConfig:       expectedBuildConfig("myproject", "https://git.server/namespace/myproject.git", "", "", ""),
				deploymentConfig:  expectedDeploymentConfig("myproject", 8080, kapi.ProtocolTCP, nil),
				imageRepositories: expectedImageRepos([]string{"myproject"}),
			},
		},
		{
			name: "ruby source repo",
			g: appGenerator{
				input: params{
					sourceDir: "/path/to/source",
				},
				srcRefGen: fakeSourceRefGenWithSourceRef{
					"directory": &genapp.SourceRef{
						URL: mustParse("https://git.server/namespace/rubyproject.git"),
					},
				},
				strategyRefGen: fakeStrategyRefGenWithStrategy{
					"sourceRef": &genapp.BuildStrategyRef{
						IsDockerBuild: false,
						Base: &genapp.ImageRef{
							DockerImageReference: imageapi.DockerImageReference{
								Namespace: "openshift",
								Name:      "ruby-20-centos7",
							},
							Info: &imageapi.DockerImage{
								Config: imageapi.DockerConfig{
									ExposedPorts: map[string]struct{}{"9292": {}},
								},
							},
						},
					},
				},
			},
			expected: expectedObjects{
				services:          expectedService("rubyproject", 9292, kapi.ProtocolTCP),
				buildConfig:       expectedBuildConfig("rubyproject", "https://git.server/namespace/rubyproject.git", "", "", "openshift/ruby-20-centos7"),
				deploymentConfig:  expectedDeploymentConfig("rubyproject", 9292, kapi.ProtocolTCP, nil),
				imageRepositories: expectedImageRepos([]string{"rubyproject"}),
			},
		},
		{
			name: "url specified ruby source",
			g: appGenerator{
				input: params{
					sourceURL: "https://git.server/namespace/rubydev.git",
				},
				srcRefGen: fakeSourceRefGenWithSourceRef{
					"gitURL": &genapp.SourceRef{
						URL: mustParse("https://git.server/namespace/rubydev.git"),
					},
				},
				strategyRefGen: fakeStrategyRefGenWithStrategy{
					"sourceRef": &genapp.BuildStrategyRef{
						IsDockerBuild: false,
						Base: &genapp.ImageRef{
							DockerImageReference: imageapi.DockerImageReference{
								Namespace: "openshift",
								Name:      "ruby-20-centos7",
							},
							Info: &imageapi.DockerImage{
								Config: imageapi.DockerConfig{
									ExposedPorts: map[string]struct{}{"8080": {}},
								},
							},
						},
					},
				},
			},
			expected: expectedObjects{
				services:          expectedService("rubydev", 8080, kapi.ProtocolTCP),
				buildConfig:       expectedBuildConfig("rubydev", "https://git.server/namespace/rubydev.git", "", "", "openshift/ruby-20-centos7"),
				deploymentConfig:  expectedDeploymentConfig("rubydev", 8080, kapi.ProtocolTCP, nil),
				imageRepositories: expectedImageRepos([]string{"rubydev"}),
			},
		},
		{
			name: "docker build with port specified",
			g: appGenerator{
				input: params{
					sourceURL: "https://git.server/namespace/dockerproject.git",
					port:      "4444",
				},
				srcRefGen: fakeSourceRefGenWithSourceRef{
					"gitURL": &genapp.SourceRef{
						URL: mustParse("https://git.server/namespace/dockerproject.git"),
					},
				},
				strategyRefGen: fakeStrategyRefGenWithStrategy{
					"sourceRef": &genapp.BuildStrategyRef{
						IsDockerBuild: true,
						Base: &genapp.ImageRef{
							DockerImageReference: imageapi.DockerImageReference{
								Name: "parentimage",
							},
							Info: &imageapi.DockerImage{
								Config: imageapi.DockerConfig{
									ExposedPorts: map[string]struct{}{"8080": {}},
								},
							},
						},
					},
				},
			},
			expected: expectedObjects{
				services:          expectedService("dockerproject", 4444, kapi.ProtocolTCP),
				buildConfig:       expectedBuildConfig("dockerproject", "https://git.server/namespace/dockerproject.git", "", "", ""),
				deploymentConfig:  expectedDeploymentConfig("dockerproject", 4444, kapi.ProtocolTCP, nil),
				imageRepositories: expectedImageRepos([]string{"dockerproject"}),
			},
		},
		{
			name: "docker build with port and name specified",
			g: appGenerator{
				input: params{
					sourceURL: "https://git.server/namespace/dockerproject.git",
					port:      "4444",
					name:      "test001",
				},
				srcRefGen: fakeSourceRefGenWithSourceRef{
					"gitURL": &genapp.SourceRef{
						URL: mustParse("https://git.server/namespace/dockerproject.git"),
					},
				},
				strategyRefGen: fakeStrategyRefGenWithStrategy{
					"sourceRef": &genapp.BuildStrategyRef{
						IsDockerBuild: true,
						Base: &genapp.ImageRef{
							DockerImageReference: imageapi.DockerImageReference{
								Name: "parentimage",
							},
							Info: &imageapi.DockerImage{
								Config: imageapi.DockerConfig{
									ExposedPorts: map[string]struct{}{"8080": {}},
								},
							},
						},
					},
				},
			},
			expected: expectedObjects{
				services:          expectedService("test001", 4444, kapi.ProtocolTCP),
				buildConfig:       expectedBuildConfig("test001", "https://git.server/namespace/dockerproject.git", "", "", ""),
				deploymentConfig:  expectedDeploymentConfig("test001", 4444, kapi.ProtocolTCP, nil),
				imageRepositories: expectedImageRepos([]string{"test001"}),
			},
		},
		{
			name: "source repo with dockerfile and context",
			g: appGenerator{
				input: params{
					sourceDir:     "/path/to/source",
					dockerContext: "v1/test",
				},
				srcRefGen: fakeSourceRefGenWithSourceRef{
					"directory": &genapp.SourceRef{
						URL: mustParse("https://git.server/namespace/myproject.git"),
					},
				},
				strategyRefGen: fakeStrategyRefGenWithStrategy{
					"sourceAndDockerContext": &genapp.BuildStrategyRef{
						IsDockerBuild: true,
						Base: &genapp.ImageRef{
							DockerImageReference: imageapi.DockerImageReference{
								Name: "parent",
							},
							Info: &imageapi.DockerImage{
								Config: imageapi.DockerConfig{
									ExposedPorts: map[string]struct{}{"8080/tcp": {}},
								},
							},
						},
					},
				},
			},
			expected: expectedObjects{
				services:          expectedService("myproject", 8080, kapi.ProtocolTCP),
				buildConfig:       expectedBuildConfig("myproject", "https://git.server/namespace/myproject.git", "", "v1/test", ""),
				deploymentConfig:  expectedDeploymentConfig("myproject", 8080, kapi.ProtocolTCP, nil),
				imageRepositories: expectedImageRepos([]string{"myproject"}),
			},
		},
		{
			name: "source repo with builder image",
			g: appGenerator{
				input: params{
					sourceDir:    "/path/to/source",
					builderImage: "test/mybuilder",
				},
				srcRefGen: fakeSourceRefGenWithSourceRef{
					"directory": &genapp.SourceRef{
						URL: mustParse("https://git.server/namespace/project01.git"),
					},
				},
				strategyRefGen: fakeStrategyRefGenWithStrategy{
					"stiBuilderImage": &genapp.BuildStrategyRef{
						IsDockerBuild: false,
					},
				},
				imageRefGen: &fakeImageRefGenWithImage{
					DockerImageReference: imageapi.DockerImageReference{
						Name:      "mybuilder",
						Namespace: "test",
					},
					Info: &imageapi.DockerImage{
						Config: imageapi.DockerConfig{
							ExposedPorts: map[string]struct{}{"1111/udp": {}},
						},
					},
				},
			},
			expected: expectedObjects{
				services:          expectedService("project01", 1111, kapi.ProtocolUDP),
				buildConfig:       expectedBuildConfig("project01", "https://git.server/namespace/project01.git", "", "", "test/mybuilder"),
				deploymentConfig:  expectedDeploymentConfig("project01", 1111, kapi.ProtocolUDP, nil),
				imageRepositories: expectedImageRepos([]string{"project01"}),
			},
		},
		{
			name: "docker build with env vars",
			g: appGenerator{
				input: params{
					sourceURL: "https://git.server/namespace/envproject.git",
					env:       genapp.Environment{"key1": "value1"},
				},
				srcRefGen: fakeSourceRefGenWithSourceRef{
					"gitURL": &genapp.SourceRef{
						URL: mustParse("https://git.server/namespace/envproject.git"),
					},
				},
				strategyRefGen: fakeStrategyRefGenWithStrategy{
					"sourceRef": &genapp.BuildStrategyRef{
						IsDockerBuild: true,
						Base: &genapp.ImageRef{
							DockerImageReference: imageapi.DockerImageReference{
								Name: "parentimage",
							},
							Info: &imageapi.DockerImage{
								Config: imageapi.DockerConfig{
									ExposedPorts: map[string]struct{}{"2222": {}},
								},
							},
						},
					},
				},
			},
			expected: expectedObjects{
				services:          expectedService("envproject", 2222, kapi.ProtocolTCP),
				buildConfig:       expectedBuildConfig("envproject", "https://git.server/namespace/envproject.git", "", "", ""),
				deploymentConfig:  expectedDeploymentConfig("envproject", 2222, kapi.ProtocolTCP, genapp.Environment{"key1": "value1"}),
				imageRepositories: expectedImageRepos([]string{"envproject"}),
			},
		},
		{
			name: "docker build with env vars",
			g: appGenerator{
				input: params{
					sourceURL: "https://git.server/namespace/envproject.git",
					env:       genapp.Environment{"key1": "value1"},
				},
				srcRefGen: fakeSourceRefGenWithSourceRef{
					"gitURL": &genapp.SourceRef{
						URL: mustParse("https://git.server/namespace/envproject.git"),
					},
				},
				strategyRefGen: fakeStrategyRefGenWithStrategy{
					"sourceRef": &genapp.BuildStrategyRef{
						IsDockerBuild: true,
						Base: &genapp.ImageRef{
							DockerImageReference: imageapi.DockerImageReference{
								Name: "parentimage",
							},
							Info: &imageapi.DockerImage{
								Config: imageapi.DockerConfig{
									ExposedPorts: map[string]struct{}{"2222": {}},
								},
							},
						},
					},
				},
			},
			expected: expectedObjects{
				services:          expectedService("envproject", 2222, kapi.ProtocolTCP),
				buildConfig:       expectedBuildConfig("envproject", "https://git.server/namespace/envproject.git", "", "", ""),
				deploymentConfig:  expectedDeploymentConfig("envproject", 2222, kapi.ProtocolTCP, genapp.Environment{"key1": "value1"}),
				imageRepositories: expectedImageRepos([]string{"envproject"}),
			},
		},
		{
			name: "docker build with git reference",
			g: appGenerator{
				input: params{
					sourceURL: "https://git.server/namespace/envproject.git",
					sourceRef: "mybranch",
				},
				srcRefGen: fakeSourceRefGenWithSourceRef{
					"gitURL": &genapp.SourceRef{
						URL: mustParse("https://git.server/namespace/envproject.git"),
						Ref: "mybranch",
					},
				},
				strategyRefGen: fakeStrategyRefGenWithStrategy{
					"sourceRef": &genapp.BuildStrategyRef{
						IsDockerBuild: true,
						Base: &genapp.ImageRef{
							DockerImageReference: imageapi.DockerImageReference{
								Name: "parentimage",
							},
							Info: &imageapi.DockerImage{
								Config: imageapi.DockerConfig{
									ExposedPorts: map[string]struct{}{"2222": {}},
								},
							},
						},
					},
				},
			},
			expected: expectedObjects{
				services:          expectedService("envproject", 2222, kapi.ProtocolTCP),
				buildConfig:       expectedBuildConfig("envproject", "https://git.server/namespace/envproject.git", "mybranch", "", ""),
				deploymentConfig:  expectedDeploymentConfig("envproject", 2222, kapi.ProtocolTCP, nil),
				imageRepositories: expectedImageRepos([]string{"envproject"}),
			},
		},
	}

	for _, test := range tests {
		list, err := test.g.run()
		if err != nil {
			t.Errorf("%s: unexpected error returned from run: %v", test.name, err)
			continue
		}
		validateObjectList(t, test.name, list, test.expected)
	}
}

func validateServices(t *testing.T, name string, expected map[string]kapi.Service, actual []kapi.Service) {
	for _, s := range actual {
		e, ok := expected[s.Name]
		if !ok {
			t.Errorf("%s: got unexpected service: %#v", name, s)
			continue
		}
		if !reflect.DeepEqual(s.ObjectMeta, e.ObjectMeta) {
			t.Errorf("%s: service meta does not match. %s", name, actualExpected(s.ObjectMeta, e.ObjectMeta))
			continue
		}
		if !reflect.DeepEqual(s.Spec, e.Spec) {
			t.Errorf("%s: service spec does not match. %s", name, actualExpected(s.Spec, e.Spec))
		}
	}
}

func validateBuildTriggers(t *testing.T, name string, expected []buildapi.BuildTriggerPolicy, actual []buildapi.BuildTriggerPolicy) {
	if len(expected) != len(actual) {
		t.Errorf("%s: did not get the same number of build triggers. %s", name, actualExpected(actual, expected))
		return
	}
	for i := range expected {
		if expected[i].Type != actual[i].Type {
			t.Errorf("%s: got unexpected build trigger type. %s", name, actualExpected(actual, expected))
			continue
		}
		if expected[i].Type == buildapi.GithubWebHookBuildTriggerType {
			if actual[i].GithubWebHook == nil || len(actual[i].GithubWebHook.Secret) == 0 {
				t.Errorf("%s: invalid trigger: %#v. Does not have a secret for github", name, actual[i])
			}
		}
		if expected[i].Type == buildapi.GenericWebHookBuildTriggerType {
			if actual[i].GenericWebHook == nil || len(actual[i].GenericWebHook.Secret) == 0 {
				t.Errorf("%s: invalid trigger: %#v. Does not have a secret for generic", name, actual[i])
			}
		}
	}
}

func validateDeploymentTriggers(t *testing.T, name string, expected []deployapi.DeploymentTriggerPolicy, actual []deployapi.DeploymentTriggerPolicy) {
	if len(expected) != len(actual) {
		t.Errorf("%s: did not get the same number of deployment triggers. %s", name, actualExpected(actual, expected))
		return
	}
	for i := range expected {
		if expected[i].Type != actual[i].Type {
			t.Errorf("%s: got unexpected deployment trigger type. %s", name, actualExpected(actual[i], expected[i]))
			continue
		}
		if expected[i].Type == deployapi.DeploymentTriggerOnImageChange {
			if !reflect.DeepEqual(expected[i].ImageChangeParams, actual[i].ImageChangeParams) {
				t.Errorf("%s: image change trigger does not match. %s", name, actualExpected(actual[i], expected[i]))
			}
		}
	}
}

func validateBuildConfig(t *testing.T, name string, expected buildapi.BuildConfig, actual []buildapi.BuildConfig) {
	if len(actual) != 1 {
		t.Errorf("%s: did not get single build config. actual: %#v", name, actual)
		return
	}
	bc := actual[0]
	if !reflect.DeepEqual(expected.ObjectMeta, bc.ObjectMeta) {
		t.Errorf("%s: build config meta does not match. %s", name, actualExpected(bc, expected))
		return
	}
	validateBuildTriggers(t, name, expected.Triggers, bc.Triggers)
	if !reflect.DeepEqual(expected.Parameters, bc.Parameters) {
		t.Errorf("%s: build parameters don't match. %s", name, actualExpected(bc.Parameters, expected.Parameters))
	}
}

func validateDeploymentConfig(t *testing.T, name string, expected deployapi.DeploymentConfig, actual []deployapi.DeploymentConfig) {
	if len(actual) != 1 {
		t.Errorf("%s: did not get single deployment config. actual: %#v", name, actual)
		return
	}
	dc := actual[0]
	if !reflect.DeepEqual(expected.ObjectMeta, dc.ObjectMeta) {
		t.Errorf("%s: deployment config meta does not match. %s", name, actualExpected(dc, expected))
		return
	}
	validateDeploymentTriggers(t, name, expected.Triggers, dc.Triggers)
	if !reflect.DeepEqual(expected.Template.Strategy, dc.Template.Strategy) {
		t.Errorf("%s: deployment strategy does not match. %s", name, actualExpected(dc.Template.Strategy, expected.Template.Strategy))
	}
	if expected.Template.ControllerTemplate.Replicas != dc.Template.ControllerTemplate.Replicas {
		t.Errorf("%s: deployment replicas does not match. %s",
			name, actualExpected(dc.Template.ControllerTemplate.Replicas, expected.Template.ControllerTemplate.Replicas))
	}
	if !reflect.DeepEqual(expected.Template.ControllerTemplate.Selector, dc.Template.ControllerTemplate.Selector) {
		t.Errorf("%s: deployment selector does not match. %s",
			name, actualExpected(dc.Template.ControllerTemplate.Selector, expected.Template.ControllerTemplate.Selector))
	}
	if !reflect.DeepEqual(expected.Template.ControllerTemplate.Template.Spec, dc.Template.ControllerTemplate.Template.Spec) {
		t.Errorf("%s: deployment pod template does not match. %s",
			name, actualExpected(dc.Template.ControllerTemplate.Template.Spec, expected.Template.ControllerTemplate.Template.Spec))
	}
}

func validateImageRepos(t *testing.T, name string, expected map[string]imageapi.ImageRepository, actual []imageapi.ImageRepository) {
	if len(expected) != len(actual) {
		t.Errorf("%s: did not get the same number of image repositories. %s", name, actualExpected(actual, expected))
		return
	}
	for _, actualRepo := range actual {
		expectedRepo, ok := expected[actualRepo.Name]
		if !ok {
			t.Errorf("%s: image repo not expected: %#v", name, actualRepo)
			continue
		}
		if !reflect.DeepEqual(actualRepo, expectedRepo) {
			t.Errorf("%s: repositories don't match: %s", name, actualExpected(actualRepo, expectedRepo))
		}
	}
}

func validateObjectList(t *testing.T, name string, list *kapi.List, expected expectedObjects) {
	generated := &genapp.Generated{Items: list.Items}

	//Validate services
	services := []kapi.Service{}
	generated.WithType(&services)
	validateServices(t, name, expected.services, services)

	// Validate buildConfig
	buildConfigs := []buildapi.BuildConfig{}
	generated.WithType(&buildConfigs)
	validateBuildConfig(t, name, expected.buildConfig, buildConfigs)

	// Validate deploymentConfig
	deploymentConfigs := []deployapi.DeploymentConfig{}
	generated.WithType(&deploymentConfigs)
	validateDeploymentConfig(t, name, expected.deploymentConfig, deploymentConfigs)

	// Validate image repositories
	imageRepos := []imageapi.ImageRepository{}
	generated.WithType(&imageRepos)
	validateImageRepos(t, name, expected.imageRepositories, imageRepos)
}
