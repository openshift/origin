package app

import (
	"log"
	"reflect"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/source-to-image/pkg/scm/git"

	_ "github.com/openshift/origin/pkg/api/install"
)

func testImageInfo() *imageapi.DockerImage {
	return &imageapi.DockerImage{
		Config: &imageapi.DockerConfig{},
	}
}

func TestWithType(t *testing.T) {
	out := &Generated{
		Items: []runtime.Object{
			&buildapi.BuildConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
			},
			&kapi.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
			},
		},
	}

	builds := []buildapi.BuildConfig{}
	if !out.WithType(&builds) {
		t.Errorf("expected true")
	}
	if len(builds) != 1 {
		t.Errorf("unexpected slice: %#v", builds)
	}

	buildPtrs := []*buildapi.BuildConfig{}
	if out.WithType(&buildPtrs) {
		t.Errorf("expected false")
	}
	if len(buildPtrs) != 0 {
		t.Errorf("unexpected slice: %#v", buildPtrs)
	}
}

func TestBuildConfigNoOutput(t *testing.T) {
	url, err := git.Parse("https://github.com/openshift/origin.git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	source := &SourceRef{URL: url}
	build := &BuildRef{Source: source}
	config, err := build.BuildConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Name != "origin" {
		t.Errorf("unexpected name: %#v", config)
	}
	if !reflect.DeepEqual(config.Spec.Output, buildapi.BuildOutput{}) {
		t.Errorf("unexpected build output: %#v", config.Spec.Output)
	}
}

func TestBuildConfigWithSecrets(t *testing.T) {
	url, err := git.Parse("https://github.com/openshift/origin.git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	source := &SourceRef{URL: url, Secrets: []buildapi.SecretBuildSource{
		{Secret: kapi.LocalObjectReference{Name: "foo"}, DestinationDir: "/var"},
		{Secret: kapi.LocalObjectReference{Name: "bar"}},
	}}
	build := &BuildRef{Source: source}
	config, err := build.BuildConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	secrets := config.Spec.Source.Secrets
	if got := len(secrets); got != 2 {
		t.Errorf("expected 2 source secrets in build config, got %d", got)
	}
}

func TestBuildConfigBinaryWithImageSource(t *testing.T) {
	source := &SourceRef{
		Name: "binarybuild",
		SourceImage: &ImageRef{
			Reference: imageapi.DockerImageReference{
				Name:     "foo",
				Registry: "bar",
			},
		},
	}
	build := &BuildRef{Source: source, Binary: true}
	config, err := build.BuildConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, trigger := range config.Spec.Triggers {
		if trigger.Type == buildapi.ImageChangeBuildTriggerType {
			t.Fatalf("binary build should not have any imagechangetriggers")
		}
		if trigger.Type == buildapi.ConfigChangeBuildTriggerType {
			t.Fatalf("binary build should not have a buildconfig change trigger")
		}

	}
}

func TestBuildConfigWithImageSource(t *testing.T) {
	source := &SourceRef{
		Name: "binarybuild",
		SourceImage: &ImageRef{
			Reference: imageapi.DockerImageReference{
				Name:     "foo",
				Registry: "bar",
			},
		},
	}
	build := &BuildRef{Source: source, Binary: false}
	config, err := build.BuildConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	foundICT := false
	foundCCT := false
	for _, trigger := range config.Spec.Triggers {
		if trigger.Type == buildapi.ImageChangeBuildTriggerType {
			foundICT = true
		}
		if trigger.Type == buildapi.ConfigChangeBuildTriggerType {
			foundCCT = true
		}
	}
	if !foundICT {
		t.Fatalf("expected to find an imagechangetrigger on the build")
	}
	if !foundCCT {
		t.Fatalf("expected to find a configchangetrigger on the build")
	}
}

func TestSourceRefBuildSourceURI(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "URL without hash",
			input:    "https://github.com/openshift/ruby-hello-world.git",
			expected: "https://github.com/openshift/ruby-hello-world.git",
		},
		{
			name:     "URL with hash",
			input:    "https://github.com/openshift/ruby-hello-world.git#testref",
			expected: "https://github.com/openshift/ruby-hello-world.git",
		},
	}
	for _, tst := range tests {
		u, _ := git.Parse(tst.input)
		s := SourceRef{
			URL: u,
		}
		buildSource, _ := s.BuildSource()
		if buildSource.Git.URI != tst.expected {
			t.Errorf("%s: unexpected build source URI: %s. Expected: %s", tst.name, buildSource.Git.URI, tst.expected)
		}
	}
}

func TestGenerateSimpleDockerApp(t *testing.T) {
	// TODO: determine if the repo is secured prior to fetching
	// TODO: determine whether we want to clone this repo, or use it directly. Using it directly would require setting hooks
	// if we have source, assume we are going to go into a build flow.
	// TODO: get info about git url: does this need STI?
	url, _ := git.Parse("https://github.com/openshift/origin.git")
	source := &SourceRef{URL: url}
	// generate a local name for the repo
	name, _ := source.SuggestName()
	// BUG: an image repo (if we want to create one) needs to tell other objects its pullspec, but we don't know what that will be
	// until the object is placed into a namespace and we lookup what registry (registries?) serve the object.
	// QUESTION: Is it ok for generation to require a namespace?  Do we want to be able to create apps with builds, image repos, and
	// deployment configs in templates (hint: yes).
	// SOLUTION? Make deployment config accept unqualified image repo names (foo) and then prior to creating the RC resolve those.
	output := &ImageRef{
		Reference: imageapi.DockerImageReference{
			Name: name,
		},
		AsImageStream: true,
	}
	// create our build based on source and input
	// TODO: we might need to pick a base image if this is STI
	build := &BuildRef{Source: source, Output: output}
	// take the output image and wire it into a deployment config
	deploy := &DeploymentConfigRef{Images: []*ImageRef{output}}

	outputRepo, _ := output.ImageStream()
	buildConfig, _ := build.BuildConfig()
	deployConfig, _ := deploy.DeploymentConfig()
	items := []runtime.Object{
		outputRepo,
		buildConfig,
		deployConfig,
	}
	out := &kapi.List{
		Items: items,
	}

	data, err := runtime.Encode(kapi.Codecs.LegacyCodec(kapi.Registry.GroupOrDie(kapi.GroupName).GroupVersions[0]), out)
	if err != nil {
		log.Fatalf("Unable to generate output: %v", err)
	}
	log.Print(string(data))
	// output:
}

func TestImageStream(t *testing.T) {
	tests := []struct {
		name        string
		r           *ImageRef
		expectedIs  *imageapi.ImageStream
		expectedErr error
	}{
		{
			name: "existing image stream",
			r: &ImageRef{
				Stream: &imageapi.ImageStream{
					ObjectMeta: metav1.ObjectMeta{
						Name: "some-stream",
					},
				},
			},
			expectedIs: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name: "some-stream",
				},
			},
		},
		{
			name: "input stream",
			r: &ImageRef{
				Reference: imageapi.DockerImageReference{
					Namespace: "test",
					Name:      "input",
				},
			},
			expectedIs: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name: "input",
				},
				Spec: imageapi.ImageStreamSpec{
					DockerImageRepository: "test/input",
				},
			},
		},
		{
			name: "insecure input stream",
			r: &ImageRef{
				Reference: imageapi.DockerImageReference{
					Namespace: "test",
					Name:      "insecure",
				},
				Insecure: true,
			},
			expectedIs: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name: "insecure",
					Annotations: map[string]string{
						imageapi.InsecureRepositoryAnnotation: "true",
					},
				},
				Spec: imageapi.ImageStreamSpec{
					DockerImageRepository: "test/insecure",
				},
			},
		},
		{
			name: "output stream",
			r: &ImageRef{
				Reference: imageapi.DockerImageReference{
					Namespace: "test",
					Name:      "output",
				},
				OutputImage: true,
			},
			expectedIs: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name: "output",
				},
			},
		},
	}

	for _, test := range tests {
		is, err := test.r.ImageStream()
		if err != test.expectedErr {
			t.Errorf("%s: error mismatch, expected %v, got %v", test.name, test.expectedErr, err)
			continue
		}
		if !reflect.DeepEqual(is, test.expectedIs) {
			t.Errorf("%s: image stream mismatch, expected %+v, got %+v", test.name, test.expectedIs, is)
		}
	}
}

func TestNameSuggestions_SuggestName(t *testing.T) {
	tests := map[string]struct {
		nameSuggestions NameSuggestions
		expectedName    string
		expectedSuccess bool
	}{
		"good suggestion from first": {
			nameSuggestions: []NameSuggester{&suggestWith{"foo", true}, &suggestWith{"", false}},
			expectedName:    "foo",
			expectedSuccess: true,
		},
		"good suggestion from second": {
			nameSuggestions: []NameSuggester{&suggestWith{"foo", false}, &suggestWith{"bar", true}},
			expectedName:    "bar",
			expectedSuccess: true,
		},
		"no good suggestions": {
			nameSuggestions: []NameSuggester{&suggestWith{"foo", false}, &suggestWith{"bar", false}},
			expectedName:    "",
			expectedSuccess: false,
		},
		"nil suggestion": {
			nameSuggestions: []NameSuggester{nil, &suggestWith{"bar", true}},
			expectedName:    "bar",
			expectedSuccess: true,
		},
	}

	for name, test := range tests {
		suggestedName, success := test.nameSuggestions.SuggestName()
		if suggestedName != test.expectedName {
			t.Errorf("%s expected name %s but recieved %s", name, test.expectedName, suggestedName)
		}
		if success != test.expectedSuccess {
			t.Errorf("%s expected success condition %t but recieved %t", name, test.expectedSuccess, success)
		}
	}
}

type suggestWith struct {
	name    string
	success bool
}

func (s *suggestWith) SuggestName() (string, bool) {
	return s.name, s.success
}

func TestIsParameterizableValue(t *testing.T) {
	tests := []struct {
		val            string
		expectedReturn bool
	}{
		// Note, parens are also allowable here.  These tests
		// are set up with braces and parens are substituted in the loop
		// to test both cases since they are handled the same.
		{"foo", false},
		{"{foo}", false},
		{"$foo}", false},
		{"foo}", false},
		{"{foo", false},
		{"${foo", true},
		{"${foo}", true},
	}

	for _, test := range tests {
		if retVal := IsParameterizableValue(test.val); retVal != test.expectedReturn {
			t.Errorf("IsParameterizableValue with %s expected %t", test.val, test.expectedReturn)
		}

		// sub in parens and run again
		replaced := strings.Replace(test.val, "{", "(", -1)
		replaced = strings.Replace(replaced, "}", ")", -1)

		if retVal := IsParameterizableValue(replaced); retVal != test.expectedReturn {
			t.Errorf("IsParameterizableValue with %s expected %t", replaced, test.expectedReturn)
		}
	}

}

func TestNameFromGitURL(t *testing.T) {
	gitURL, err := git.Parse("https://github.com/openshift/origin.git")
	if err != nil {
		t.Fatalf("failed parsing git url: %v", err)
	}

	emptyHostURL, err := git.Parse("https://")
	if err != nil {
		t.Fatalf("failed parsing empty host url: %v", err)
	}

	hostPortURL, err := git.Parse("https://www.example.com:80")
	if err != nil {
		t.Fatalf("failed parsing host port url: %v", err)
	}

	nonStandardHostPortURL, err := git.Parse("https://www.example.com:8888")
	if err != nil {
		t.Fatalf("failed parsing host port url: %v", err)
	}

	hostURL, err := git.Parse("https://www.example.com")
	if err != nil {
		t.Fatalf("failed parsing host url: %v", err)
	}

	tests := map[string]struct {
		url             *git.URL
		expectedName    string
		expectedSuccess bool
	}{
		"nil url":                {url: nil, expectedName: "", expectedSuccess: false},
		"git url":                {url: gitURL, expectedName: "origin", expectedSuccess: true},
		"empty host":             {url: emptyHostURL, expectedName: "", expectedSuccess: false},
		"host port":              {url: hostPortURL, expectedName: "www.example.com", expectedSuccess: true},
		"non standard host port": {url: nonStandardHostPortURL, expectedName: "www.example.com", expectedSuccess: true},
		"host": {url: hostURL, expectedName: "www.example.com", expectedSuccess: true},
	}

	for name, test := range tests {
		parsedName, success := nameFromGitURL(test.url)
		if parsedName != test.expectedName {
			t.Errorf("%s expected name to be %s but got %s", name, test.expectedName, parsedName)
		}
		if success != test.expectedSuccess {
			t.Errorf("%s expected success to be %t", name, test.expectedSuccess)
		}
	}
}

func TestContainerPortsFromString(t *testing.T) {
	tests := map[string]struct {
		portString    string
		expectedPorts []kapi.ContainerPort
		expectedError string
	}{
		"single port": {
			portString: "80",
			expectedPorts: []kapi.ContainerPort{
				{ContainerPort: 80, HostPort: 0},
			},
		},
		"single port with separator and no host port": {
			portString:    "80:",
			expectedPorts: nil,
			expectedError: "is not valid: you must specify one (container) or two (container:host) port numbers",
		},
		"single port with multiple separators": {
			portString:    "80:81:82",
			expectedPorts: nil,
			expectedError: "is not valid: you must specify one (container) or two (container:host) port numbers",
		},
		"single port with host port": {
			portString: "80:80",
			expectedPorts: []kapi.ContainerPort{
				{ContainerPort: 80, HostPort: 80},
			},
		},
		"multiple port": {
			portString: "80:80,443:443",
			expectedPorts: []kapi.ContainerPort{
				{ContainerPort: 80, HostPort: 80},
				{ContainerPort: 443, HostPort: 443},
			},
		},
		"not a number container": {
			portString:    "abc:80",
			expectedPorts: nil,
			expectedError: "is not valid: you must specify one (container) or two (container:host) port numbers",
		},
		"not a number host": {
			portString:    "80:abc",
			expectedPorts: nil,
			expectedError: "is not valid: you must specify one (container) or two (container:host) port numbers",
		},
		"empty string": {
			portString:    "",
			expectedPorts: nil,
			expectedError: "is not valid: you must specify one (container) or two (container:host) port numbers",
		},
	}

	for name, test := range tests {
		ports, err := ContainerPortsFromString(test.portString)
		if !reflect.DeepEqual(ports, test.expectedPorts) {
			t.Errorf("%s expected ports to be %#v but got %#v", name, test.expectedPorts, ports)
		}
		checkError(err, test.expectedError, name, t)
	}
}

func TestLabelsFromSpec(t *testing.T) {
	tests := map[string]struct {
		spec                 []string
		expectedLabels       map[string]string
		expectedRemoveLabels []string
		expectedError        string
	}{
		"empty spec": {
			expectedLabels: map[string]string{},
		},
		"spec with =": {
			spec:           []string{"foo=bar"},
			expectedLabels: map[string]string{"foo": "bar"},
		},
		"invalid label spec": {
			spec:          []string{"foo=bar=foobar"},
			expectedError: "invalid label spec",
		},
		"spec with -": {
			spec:                 []string{"foo-"},
			expectedLabels:       map[string]string{},
			expectedRemoveLabels: []string{"foo"},
		},
		"unknown label spec": {
			spec:          []string{"foo:bar"},
			expectedError: "unknown label spec",
		},
		"modify and remove": {
			spec:          []string{"foo=bar", "foo-"},
			expectedError: "can not both modify and remove a label in the same command",
		},
	}
	for name, test := range tests {
		labels, removeLabels, err := LabelsFromSpec(test.spec)
		checkError(err, test.expectedError, name, t)

		if !reflect.DeepEqual(labels, test.expectedLabels) {
			t.Errorf("%s expected labels %#v but got %#v", name, test.expectedLabels, labels)
		}
		if !reflect.DeepEqual(removeLabels, test.expectedRemoveLabels) {
			t.Errorf("%s expected to remove labels %#v but got %#v", name, test.expectedRemoveLabels, removeLabels)
		}

	}
}

func checkError(err error, expectedError string, name string, t *testing.T) {
	if err != nil && expectedError == "" {
		t.Errorf("%s expected no error but got %v", name, err)
	}
	if err == nil && expectedError != "" {
		t.Errorf("%s expected error %s but got none", name, expectedError)
	}
	if err != nil && expectedError != "" && !strings.Contains(err.Error(), expectedError) {
		t.Errorf("%s expected error to contain %s but got %s", name, expectedError, err.Error())
	}
}
