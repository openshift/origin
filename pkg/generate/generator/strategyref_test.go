package generator

import (
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/openshift/origin/pkg/generate/app"
	"github.com/openshift/origin/pkg/generate/dockerfile"
	"github.com/openshift/origin/pkg/generate/generator/test"
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
		t.Errorf("Unexpected error generating imageRef: %v", err)
	}
	strategy, err := g.FromSTIBuilderImage(imgRef)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if strategy.Base != imgRef {
		t.Errorf("Unexpected image reference: %v", strategy.Base)
	}
	if strategy.IsDockerBuild {
		t.Errorf("Expected IsDockerBuild to be false")
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
		t.Errorf("Unexpected error: %v", err)
	}
	strategy, err := g.FromDockerContextAndParent(imgRef)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if strategy.Base.Name != "parentImage" {
		t.Errorf("Unexpected base image: %#v", strategy.Base)
	}
	if !strategy.IsDockerBuild {
		t.Errorf("Expected IsDockerBuild to be true")
	}
}

func TestFromSourceRefAndDockerContext(t *testing.T) {
	g := &BuildStrategyRefGenerator{
		gitRepository:     &test.FakeGit{},
		dockerfileFinder:  &fakeFinder{},
		dockerfileParser:  &fakeParser{dfile{"FROM": []string{"test/parentImage"}}},
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
	srcRef := app.SourceRef{
		URL: url,
		Dir: tmp,
		Ref: "master",
	}
	strategy, err := g.FromSourceRefAndDockerContext(&srcRef, ".")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if strategy.Base.Name != "parentImage" {
		t.Errorf("Unexpected base image: %#v", strategy.Base)
	}
	if !strategy.IsDockerBuild {
		t.Errorf("Expected IsDockerBuild to be true")
	}
}
func TestFromSourceRefDocker(t *testing.T) {
	g := &BuildStrategyRefGenerator{
		gitRepository:     &test.FakeGit{},
		dockerfileFinder:  &fakeFinder{result: []string{"Dockerfile"}},
		dockerfileParser:  &fakeParser{dfile{"FROM": []string{"test/parentImage"}}},
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
	srcRef := app.SourceRef{
		URL: url,
		Dir: tmp,
		Ref: "master",
	}
	strategy, err := g.FromSourceRef(&srcRef)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if strategy.Base.Name != "parentImage" {
		t.Errorf("Unexpected base image: %#v", strategy.Base)
	}
	if !strategy.IsDockerBuild {
		t.Errorf("Expected IsDockerBuild to be true")
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
	srcRef := app.SourceRef{
		URL: url,
		Dir: "/tmp/dir",
		Ref: "master",
	}
	strategy, err := g.FromSourceRef(&srcRef)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if strategy.Base.Name != "wildfly-8-centos" {
		t.Errorf("Unexpected base image: %#v", strategy.Base)
	}
	if strategy.IsDockerBuild {
		t.Errorf("Expected IsDockerBuild to be false")
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
