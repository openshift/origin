package app

import (
	"os"
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
	"github.com/openshift/origin/pkg/generate"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/source-to-image/pkg/scm/git"
)

func TestBuildConfigOutput(t *testing.T) {
	url, err := git.Parse("https://github.com/openshift/origin.git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := &ImageRef{
		Reference: imageapi.DockerImageReference{
			Registry:  "myregistry",
			Namespace: "openshift",
			Name:      "origin",
		},
	}
	base := &ImageRef{
		Reference: imageapi.DockerImageReference{
			Namespace: "openshift",
			Name:      "ruby",
		},
		Info:          testImageInfo(),
		AsImageStream: true,
	}
	tests := []struct {
		asImageStream bool
		want          *kapi.ObjectReference
	}{
		{
			asImageStream: true,
			want: &kapi.ObjectReference{
				Kind: "ImageStreamTag",
				Name: "origin:latest",
			},
		},
		{
			asImageStream: false,
			want: &kapi.ObjectReference{
				Kind: "DockerImage",
				Name: "myregistry/openshift/origin",
			},
		},
	}
	for i, test := range tests {
		output.AsImageStream = test.asImageStream
		source := &SourceRef{URL: url}
		strategy := &BuildStrategyRef{Strategy: generate.StrategySource, Base: base}
		build := &BuildRef{Source: source, Output: output, Strategy: strategy}
		config, err := build.BuildConfig()
		if err != nil {
			t.Fatalf("(%d) unexpected error: %v", i, err)
		}
		if config.Name != "origin" {
			t.Errorf("(%d) unexpected name: %s", i, config.Name)
		}
		if !reflect.DeepEqual(config.Spec.Output.To, test.want) {
			t.Errorf("(%d) unexpected output image: %v; want %v", i, config.Spec.Output.To, test.want)
		}
		if len(config.Spec.Triggers) != 4 {
			t.Errorf("(%d) unexpected number of triggers %d: %#v\n", i, len(config.Spec.Triggers), config.Spec.Triggers)
		}
		imageChangeTrigger := false
		for _, trigger := range config.Spec.Triggers {
			if trigger.Type == buildapi.ImageChangeBuildTriggerType {
				imageChangeTrigger = true
				if trigger.ImageChange == nil {
					t.Errorf("(%d) invalid image change trigger found: %#v", i, trigger)
				}
			}
		}
		if !imageChangeTrigger {
			t.Errorf("expecting image change trigger in build config")
		}
	}
}

func TestSimpleDeploymentConfig(t *testing.T) {
	image := &ImageRef{
		Reference: imageapi.DockerImageReference{
			Registry:  "myregistry",
			Namespace: "openshift",
			Name:      "origin",
		},
		Info:          testImageInfo(),
		AsImageStream: true,
	}
	deploy := &DeploymentConfigRef{Images: []*ImageRef{image}}
	config, err := deploy.DeploymentConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Name != "origin" || len(config.Spec.Triggers) != 2 || config.Spec.Template.Spec.Containers[0].Image != image.Reference.String() {
		t.Errorf("unexpected value: %#v", config)
	}
	for _, trigger := range config.Spec.Triggers {
		if trigger.Type == deployapi.DeploymentTriggerOnImageChange {
			from := trigger.ImageChangeParams.From
			if from.Kind != "ImageStreamTag" {
				t.Errorf("unexpected from.kind in image change trigger: %s", from.Kind)
			}
			if from.Name != "origin:latest" && from.Namespace != "openshift" {
				t.Errorf("unexpected from.name %q and from.namespace %q in image change trigger", from.Name, from.Namespace)
			}
		}
	}
}

func TestImageRefDeployableContainerPorts(t *testing.T) {
	tests := []struct {
		name          string
		inputPorts    map[string]struct{}
		expectedPorts map[int]string
		expectError   bool
		noConfig      bool
	}{
		{
			name: "tcp implied, individual ports",
			inputPorts: map[string]struct{}{
				"123": {},
				"456": {},
			},
			expectedPorts: map[int]string{
				123: "TCP",
				456: "TCP",
			},
			expectError: false,
		},
		{
			name: "tcp implied, multiple ports",
			inputPorts: map[string]struct{}{
				"123 456":  {},
				"678 1123": {},
			},
			expectedPorts: map[int]string{
				123:  "TCP",
				678:  "TCP",
				456:  "TCP",
				1123: "TCP",
			},
			expectError: false,
		},
		{
			name: "tcp and udp, individual ports",
			inputPorts: map[string]struct{}{
				"123/tcp": {},
				"456/udp": {},
			},
			expectedPorts: map[int]string{
				123: "TCP",
				456: "UDP",
			},
			expectError: false,
		},
		{
			name: "tcp implied, multiple ports",
			inputPorts: map[string]struct{}{
				"123/tcp 456/udp":  {},
				"678/udp 1123/tcp": {},
			},
			expectedPorts: map[int]string{
				123:  "TCP",
				456:  "UDP",
				678:  "UDP",
				1123: "TCP",
			},
			expectError: false,
		},
		{
			name: "invalid format",
			inputPorts: map[string]struct{}{
				"123/tcp abc": {},
			},
			expectedPorts: map[int]string{},
			expectError:   true,
		},
		{
			name:          "no image config",
			expectedPorts: map[int]string{},
			expectError:   false,
			noConfig:      true,
		},
	}
	for _, test := range tests {
		imageRef := &ImageRef{
			Reference: imageapi.DockerImageReference{
				Namespace: "test",
				Name:      "image",
				Tag:       imageapi.DefaultImageTag,
			},
			Info: &imageapi.DockerImage{
				Config: &imageapi.DockerConfig{
					ExposedPorts: test.inputPorts,
				},
			},
		}
		if test.noConfig {
			imageRef.Info.Config = nil
		}
		container, _, err := imageRef.DeployableContainer()
		if err != nil && !test.expectError {
			t.Errorf("%s: unexpected error: %v", test.name, err)
			continue
		}
		if err == nil && test.expectError {
			t.Errorf("%s: got no error and expected an error", test.name)
			continue
		}
		if test.expectError {
			continue
		}
		remaining := test.expectedPorts
		for _, port := range container.Ports {
			proto, ok := remaining[int(port.ContainerPort)]
			if !ok {
				t.Errorf("%s: got unexpected port: %v", test.name, port)
				continue
			}
			if kapi.Protocol(proto) != port.Protocol {
				t.Errorf("%s: got unexpected protocol %s for port %v", test.name, port.Protocol, port)
			}
			delete(remaining, int(port.ContainerPort))
		}
		if len(remaining) > 0 {
			t.Errorf("%s: did not find expected ports: %#v", test.name, remaining)
		}
	}
}

func TestFromName(t *testing.T) {
	g := NewImageRefGenerator()
	imageRef, err := g.FromName("some.registry.com:1234/namespace1/name:tag1234")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	ref := imageRef.Reference
	if ref.Registry != "some.registry.com:1234" {
		t.Fatalf("Unexpected registry: %s", ref.Registry)
	}
	if ref.Namespace != "namespace1" {
		t.Fatalf("Unexpected namespace: %s", ref.Namespace)
	}
	if ref.Name != "name" {
		t.Fatalf("Unexpected name: %s", ref.Name)
	}
	if ref.Tag != "tag1234" {
		t.Fatalf("Unexpected tag: %s", ref.Tag)
	}
}

func TestFromStream(t *testing.T) {
	g := NewImageRefGenerator()
	repo := imageapi.ImageStream{
		Status: imageapi.ImageStreamStatus{
			DockerImageRepository: "my.registry:5000/test/image",
		},
	}
	imageRef, err := g.FromStream(&repo, "tag1234")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	ref := imageRef.Reference
	if ref.Registry != "my.registry:5000" {
		t.Fatalf("Unexpected registry: %s", ref.Registry)
	}
	if ref.Namespace != "test" {
		t.Fatalf("Unexpected namespace: %s", ref.Namespace)
	}
	if ref.Name != "image" {
		t.Fatalf("Unexpected name: %s", ref.Name)
	}
}

func TestFromNameAndPorts(t *testing.T) {
	g := NewImageRefGenerator()
	ports := []string{"8080"}
	imageRef, err := g.FromNameAndPorts("some.registry.com:1234/namespace1/name:tag1234", ports)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	ref := imageRef.Reference
	if ref.Registry != "some.registry.com:1234" {
		t.Fatalf("Unexpected registry: %s", ref.Registry)
	}
	if ref.Namespace != "namespace1" {
		t.Fatalf("Unexpected namespace: %s", ref.Namespace)
	}
	if ref.Name != "name" {
		t.Fatalf("Unexpected name: %s", ref.Name)
	}
	if ref.Tag != "tag1234" {
		t.Fatalf("Unexpected tag: %s", ref.Tag)
	}
	if _, ok := imageRef.Info.Config.ExposedPorts[ports[0]]; !ok {
		t.Fatalf("Expected port %s not found", ports[0])
	}
}

func TestFromDockerfile(t *testing.T) {
	// Setup a Dockerfile
	df, err := os.Create("Dockerfile")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer os.Remove(df.Name())
	if _, err := df.Write([]byte(dockerFile)); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	df.Close()
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Actual test
	g := NewImageRefGenerator()
	name := "some.registry.com:1234/namespace1/name:tag1234"
	imageRef, err := g.FromDockerfile(name, pwd, ".")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	ref := imageRef.Reference
	if ref.Registry != "some.registry.com:1234" {
		t.Fatalf("Unexpected registry: %s", ref.Registry)
	}
	if ref.Namespace != "namespace1" {
		t.Fatalf("Unexpected namespace: %s", ref.Namespace)
	}
	if ref.Name != "name" {
		t.Fatalf("Unexpected name: %s", ref.Name)
	}
	if ref.Tag != "tag1234" {
		t.Fatalf("Unexpected tag: %s", ref.Tag)
	}
	expectedPort := "443"
	if _, ok := imageRef.Info.Config.ExposedPorts[expectedPort]; !ok {
		t.Fatalf("Expected port %s not found", expectedPort)
	}
}

const dockerFile = `FROM centos/ruby-22-centos7
USER default
EXPOSE 443
ENV RACK_ENV production
ENV RAILS_ENV production
COPY . /opt/openshift/src/
RUN scl enable ror40 "bundle install"
CMD ["scl", "enable", "ror40", "./run.sh"]`
