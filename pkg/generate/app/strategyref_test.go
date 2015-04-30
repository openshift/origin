package app

import (
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/openshift/origin/pkg/generate/app/test"
	"github.com/openshift/origin/pkg/generate/dockerfile"
	"github.com/openshift/origin/pkg/generate/source"
)

var sourceDetectors = source.Detectors{
	fakeDetector,
}

func TestFromSTIBuilderImage(t *testing.T) {
	g := &BuildStrategyRefGenerator{
		gitRepository:     &test.FakeGit{},
		dockerfileFinder:  &fakeFinder{},
		dockerfileParser:  &fakeParser{},
		sourceDetectors:   sourceDetectors,
		imageRefGenerator: NewImageRefGenerator(),
	}
	imgRef, err := g.imageRefGenerator.FromName("test/image")
	if err != nil {
		t.Fatalf("Unexpected error generating imageRef: %v", err)
	}
	strategy, err := g.FromSTIBuilderImage(imgRef)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if strategy.Base != imgRef {
		t.Fatalf("Unexpected image reference: %v", strategy.Base)
	}
	if strategy.IsDockerBuild {
		t.Fatalf("Expected IsDockerBuild to be false")
	}
}

func TestFromDockerContextAndParent(t *testing.T) {
	g := &BuildStrategyRefGenerator{
		gitRepository:     &test.FakeGit{},
		dockerfileFinder:  &fakeFinder{},
		dockerfileParser:  &fakeParser{},
		sourceDetectors:   sourceDetectors,
		imageRefGenerator: NewImageRefGenerator(),
	}
	imgRef, err := g.imageRefGenerator.FromName("test/parentImage")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	strategy, err := g.FromDockerContextAndParent(imgRef)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if strategy.Base.Name != "parentImage" {
		t.Fatalf("Unexpected base image: %#v", strategy.Base)
	}
	if !strategy.IsDockerBuild {
		t.Fatalf("Expected IsDockerBuild to be true")
	}
}

func TestFromSourceRefAndDockerContext(t *testing.T) {
	exposedPort := "8080"
	g := &BuildStrategyRefGenerator{
		gitRepository:     &test.FakeGit{},
		dockerfileFinder:  &fakeFinder{},
		dockerfileParser:  &fakeParser{dfile{"FROM": []string{"test/parentImage"}, "EXPOSE": []string{exposedPort}}},
		sourceDetectors:   sourceDetectors,
		imageRefGenerator: NewImageRefGenerator(),
	}
	url, _ := url.Parse("https://test.repository.com/test.git")
	tmp, err := ioutil.TempDir("", "test")
	if err != nil {
		t.Fatalf("Unable to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmp)
	f, err := os.Create(filepath.Join(tmp, "Dockerfile"))
	if err != nil {
		t.Fatalf("Unable to create temp file: %v", err)
	}
	f.Close()
	srcRef := SourceRef{
		URL: url,
		Dir: tmp,
		Ref: "master",
	}
	strategy, err := g.FromSourceRefAndDockerContext(&srcRef, ".")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if strategy.Base.Name != "parentImage" {
		t.Fatalf("Unexpected base image: %#v", strategy.Base)
	}
	if !strategy.IsDockerBuild {
		t.Fatalf("Expected IsDockerBuild to be true")
	}
	if _, ok := strategy.Base.Info.Config.ExposedPorts[exposedPort]; !ok {
		t.Fatalf("Expected port %s not found", exposedPort)
	}
}
func TestFromSourceRefDocker(t *testing.T) {
	exposedPort := "8080"
	g := &BuildStrategyRefGenerator{
		gitRepository:     &test.FakeGit{},
		dockerfileFinder:  &fakeFinder{result: []string{"Dockerfile"}},
		dockerfileParser:  &fakeParser{dfile{"FROM": []string{"test/parentImage"}, "EXPOSE": []string{exposedPort}}},
		sourceDetectors:   sourceDetectors,
		imageRefGenerator: NewImageRefGenerator(),
	}
	url, _ := url.Parse("https://test.repository.com/test.git")
	tmp, err := ioutil.TempDir("", "test")
	if err != nil {
		t.Fatalf("Unable to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmp)
	f, err := os.Create(filepath.Join(tmp, "Dockerfile"))
	if err != nil {
		t.Fatalf("Unable to create temp file: %v", err)
	}
	f.Close()
	srcRef := SourceRef{
		URL: url,
		Dir: tmp,
		Ref: "master",
	}
	strategy, err := g.FromSourceRef(&srcRef)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if strategy.Base.Name != "parentImage" {
		t.Fatalf("Unexpected base image: %#v", strategy.Base)
	}
	if !strategy.IsDockerBuild {
		t.Fatalf("Expected IsDockerBuild to be true")
	}
	if _, ok := strategy.Base.Info.Config.ExposedPorts[exposedPort]; !ok {
		t.Fatalf("Expected port %s not found", exposedPort)
	}
}

func TestFromSourceRefSTI(t *testing.T) {
	g := &BuildStrategyRefGenerator{
		gitRepository:     &test.FakeGit{},
		dockerfileFinder:  &fakeFinder{},
		dockerfileParser:  &fakeParser{},
		sourceDetectors:   sourceDetectors,
		imageRefGenerator: NewImageRefGenerator(),
	}
	url, _ := url.Parse("https://test.repository.com/test.git")
	srcRef := SourceRef{
		URL: url,
		Dir: "/tmp/dir",
		Ref: "master",
	}
	strategy, err := g.FromSourceRef(&srcRef)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if strategy.Base.Name != "wildfly-8-centos" {
		t.Fatalf("Unexpected base image: %#v", strategy.Base)
	}
	if strategy.IsDockerBuild {
		t.Fatalf("Expected IsDockerBuild to be false")
	}
}

type fakeFinder struct {
	result []string
}

func (f *fakeFinder) Find(dir string) ([]string, error) {
	return f.result, nil
}

type dfile map[string][]string

func (d dfile) GetDirective(name string) ([]string, bool) {
	result, ok := d[name]
	return result, ok
}

type fakeParser struct {
	result dfile
}

func (f *fakeParser) Parse(input io.Reader) (dockerfile.Dockerfile, error) {
	return f.result, nil
}

func fakeDetector(dir string) (*source.Info, bool) {
	return &source.Info{
		Platform: "JEE",
		Version:  "1.0",
	}, true
}
