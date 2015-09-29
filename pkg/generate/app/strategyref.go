package app

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/openshift/origin/pkg/generate/dockerfile"
	"github.com/openshift/origin/pkg/generate/errors"
	"github.com/openshift/origin/pkg/generate/git"
	"github.com/openshift/origin/pkg/generate/source"
)

// BuildStrategyRefGenerator generates BuildStrategyRef
//
// Flows for BuildStrategyRef
// SourceRef -> BuildStrategyRef
// SourceRef + Docker Context -> BuildStrategyRef
// Docker Context + Parent Image -> BuildStrategyRef
// STI Builder Image -> BuildStrategyRef
type BuildStrategyRefGenerator struct {
	gitRepository     git.Repository
	dockerfileFinder  dockerfile.Finder
	sourceDetectors   source.Detectors
	imageRefGenerator ImageRefGenerator
}

// NewBuildStrategyRefGenerator creates a BuildStrategyRefGenerator
func NewBuildStrategyRefGenerator(sourceDetectors source.Detectors) *BuildStrategyRefGenerator {
	return &BuildStrategyRefGenerator{
		gitRepository:     git.NewRepository(),
		dockerfileFinder:  dockerfile.NewFinder(),
		sourceDetectors:   sourceDetectors,
		imageRefGenerator: NewImageRefGenerator(),
	}
}

// FromDockerContextAndParent generates a build strategy ref from a context path and parent image name
func (g *BuildStrategyRefGenerator) FromDockerContextAndParent(parentRef *ImageRef) (*BuildStrategyRef, error) {
	return &BuildStrategyRef{
		IsDockerBuild: true,
		Base:          parentRef,
	}, nil
}

// FromSTIBuilderImage generates a build strategy from a builder image ref
func (g *BuildStrategyRefGenerator) FromSTIBuilderImage(image *ImageRef) (*BuildStrategyRef, error) {
	return &BuildStrategyRef{
		IsDockerBuild: false,
		Base:          image,
	}, nil
}

func (g *BuildStrategyRefGenerator) detectDockerFile(dir string) (contextDir string, found bool, err error) {
	dockerFiles, err := g.dockerfileFinder.Find(dir)
	if err != nil {
		return "", false, err
	}
	if len(dockerFiles) > 1 {
		return "", true, errors.NewMultipleDockerfilesErr(dockerFiles)
	}
	if len(dockerFiles) == 1 {
		return filepath.Dir(dockerFiles[0]), true, nil
	}
	return "", false, nil
}

func (g *BuildStrategyRefGenerator) getSource(srcRef *SourceRef) error {
	var err error
	// Clone git repository into a local directory
	if srcRef.Dir, err = ioutil.TempDir("", "gen"); err != nil {
		return err
	}
	if err = g.gitRepository.Clone(srcRef.Dir, srcRef.URL.String()); err != nil {
		return fmt.Errorf("unable to clone repository at %s", srcRef.URL.String())
	}
	if len(srcRef.Ref) != 0 {
		if err = g.gitRepository.Checkout(srcRef.Dir, srcRef.Ref); err != nil {
			return fmt.Errorf("unable to checkout reference %s from repository at %s", srcRef.Ref, srcRef.URL.String())
		}
	}
	return nil
}
