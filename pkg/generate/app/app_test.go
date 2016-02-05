package app

import (
	"log"
	"net/url"
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	"k8s.io/kubernetes/pkg/runtime"

	buildapi "github.com/openshift/origin/pkg/build/api"
	imageapi "github.com/openshift/origin/pkg/image/api"

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
				ObjectMeta: kapi.ObjectMeta{
					Name: "foo",
				},
			},
			&kapi.Service{
				ObjectMeta: kapi.ObjectMeta{
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
	url, err := url.Parse("https://github.com/openshift/origin.git")
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
	url, err := url.Parse("https://github.com/openshift/origin.git")
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
		u, _ := url.Parse(tst.input)
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
	url, _ := url.Parse("https://github.com/openshift/origin.git")
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

	data, err := runtime.Encode(kapi.Codecs.LegacyCodec(registered.GroupOrDie(kapi.GroupName).GroupVersions[0]), out)
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
					ObjectMeta: kapi.ObjectMeta{
						Name: "some-stream",
					},
				},
			},
			expectedIs: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
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
				ObjectMeta: kapi.ObjectMeta{
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
				ObjectMeta: kapi.ObjectMeta{
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
				ObjectMeta: kapi.ObjectMeta{
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
