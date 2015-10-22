package app

import (
	"os"
	"testing"

	image "github.com/openshift/origin/pkg/image/api"
)

func TestFromName(t *testing.T) {
	g := NewImageRefGenerator()
	imageRef, err := g.FromName("some.registry.com:1234/namespace1/name:tag1234")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if imageRef.Registry != "some.registry.com:1234" {
		t.Fatalf("Unexpected registry: %s", imageRef.Registry)
	}
	if imageRef.Namespace != "namespace1" {
		t.Fatalf("Unexpected namespace: %s", imageRef.Namespace)
	}
	if imageRef.Name != "name" {
		t.Fatalf("Unexpected name: %s", imageRef.Name)
	}
	if imageRef.Tag != "tag1234" {
		t.Fatalf("Unexpected tag: %s", imageRef.Tag)
	}
}

func TestFromStream(t *testing.T) {
	g := NewImageRefGenerator()
	repo := image.ImageStream{
		Status: image.ImageStreamStatus{
			DockerImageRepository: "my.registry:5000/test/image",
		},
	}
	imageRef, err := g.FromStream(&repo, "tag1234")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if imageRef.Registry != "my.registry:5000" {
		t.Fatalf("Unexpected registry: %s", imageRef.Registry)
	}
	if imageRef.Namespace != "test" {
		t.Fatalf("Unexpected namespace: %s", imageRef.Namespace)
	}
	if imageRef.Name != "image" {
		t.Fatalf("Unexpected name: %s", imageRef.Name)
	}
}

func TestFromNameAndPorts(t *testing.T) {
	g := NewImageRefGenerator()
	ports := []string{"8080"}
	imageRef, err := g.FromNameAndPorts("some.registry.com:1234/namespace1/name:tag1234", ports)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if imageRef.Registry != "some.registry.com:1234" {
		t.Fatalf("Unexpected registry: %s", imageRef.Registry)
	}
	if imageRef.Namespace != "namespace1" {
		t.Fatalf("Unexpected namespace: %s", imageRef.Namespace)
	}
	if imageRef.Name != "name" {
		t.Fatalf("Unexpected name: %s", imageRef.Name)
	}
	if imageRef.Tag != "tag1234" {
		t.Fatalf("Unexpected tag: %s", imageRef.Tag)
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
	if _, writeErr := df.Write([]byte(dockerFile)); writeErr != nil {
		t.Fatalf("Unexpected error: %v", writeErr)
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
	if imageRef.Registry != "some.registry.com:1234" {
		t.Fatalf("Unexpected registry: %s", imageRef.Registry)
	}
	if imageRef.Namespace != "namespace1" {
		t.Fatalf("Unexpected namespace: %s", imageRef.Namespace)
	}
	if imageRef.Name != "name" {
		t.Fatalf("Unexpected name: %s", imageRef.Name)
	}
	if imageRef.Tag != "tag1234" {
		t.Fatalf("Unexpected tag: %s", imageRef.Tag)
	}
	expectedPort := "443"
	if _, ok := imageRef.Info.Config.ExposedPorts[expectedPort]; !ok {
		t.Fatalf("Expected port %s not found", expectedPort)
	}
}

const dockerFile = `FROM openshift/ruby-20-centos7
USER default
EXPOSE 443
ENV RACK_ENV production
ENV RAILS_ENV production
COPY . /opt/openshift/src/
RUN scl enable ror40 "bundle install"
CMD ["scl", "enable", "ror40", "./run.sh"]`
