package app

import (
	"log"
	"net/url"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/api/latest"
	build "github.com/openshift/origin/pkg/build/api"
	config "github.com/openshift/origin/pkg/config/api"
)

func TestWithType(t *testing.T) {
	out := &Generated{
		Items: []runtime.Object{
			&build.BuildConfig{
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

	builds := []build.BuildConfig{}
	if !out.WithType(&builds) {
		t.Errorf("expected true")
	}
	if len(builds) != 1 {
		t.Errorf("unexpected slice: %#v", builds)
	}

	buildPtrs := []*build.BuildConfig{}
	if out.WithType(&buildPtrs) {
		t.Errorf("expected false")
	}
	if len(buildPtrs) != 0 {
		t.Errorf("unexpected slice: %#v", buildPtrs)
	}
}

func TestSimpleBuildConfig(t *testing.T) {
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

	output := &ImageRef{Registry: "myregistry", Namespace: "openshift", Name: "origin"}
	build = &BuildRef{Source: source, Output: output}
	config, err = build.BuildConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Name != "origin" || config.Parameters.Output.ImageTag != "myregistry/openshift/origin" || config.Parameters.Output.Registry != "myregistry" {
		t.Errorf("unexpected name: %#v", config)
	}
}

func TestSimpleDeploymentConfig(t *testing.T) {
	image := &ImageRef{Registry: "myregistry", Namespace: "openshift", Name: "origin"}
	deploy := &DeploymentConfigRef{Images: []*ImageInfo{{ImageRef: image}}}
	config, err := deploy.DeploymentConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Name != "origin" || len(config.Triggers) != 2 || config.Template.ControllerTemplate.Template.Spec.Containers[0].Image != image.pullSpec() {
		t.Errorf("unexpected value: %#v", config)
	}
}

func ExampleGenerateSimpleDockerApp() {
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
	output := &ImageRef{Name: name, AsImageRepository: true}
	// create our build based on source and input
	// TODO: we might need to pick a base image if this is STI
	build := &BuildRef{Source: source, Output: output}
	// take the output image and wire it into a deployment config
	deploy := &DeploymentConfigRef{[]*ImageInfo{{ImageRef: output}}}

	outputRepo, _ := output.ImageRepository()
	buildConfig, _ := build.BuildConfig()
	deployConfig, _ := deploy.DeploymentConfig()
	items := []runtime.Object{
		outputRepo,
		buildConfig,
		deployConfig,
	}
	rawItems := []runtime.RawExtension{}
	kapi.Scheme.Convert(items, &rawItems)

	out := &config.Config{
		Items: rawItems,
	}

	data, err := latest.Codec.Encode(out)
	if err != nil {
		log.Fatalf("Unable to generate output: %v", err)
	}
	log.Print(string(data))
	// output:
}
