package app

import (
	"testing"

	"github.com/openshift/origin/pkg/generate/app/test"
	"github.com/openshift/origin/pkg/generate/source"
)

var sourceDetectors = source.Detectors{
	fakeDetector,
}

func TestFromSTIBuilderImage(t *testing.T) {
	g := &BuildStrategyRefGenerator{
		gitRepository:     &test.FakeGit{},
		dockerfileFinder:  &fakeFinder{},
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
	if strategy.Base.Reference.Name != "parentImage" {
		t.Fatalf("Unexpected base image: %#v", strategy.Base)
	}
	if !strategy.IsDockerBuild {
		t.Fatalf("Expected IsDockerBuild to be true")
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

func fakeDetector(dir string) (*source.Info, bool) {
	return &source.Info{
		Platform: "JEE",
		Version:  "1.0",
	}, true
}
