package test

import (
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
)

type TestBuild buildapi.Build

func Build() *TestBuild {
	b := (*TestBuild)(&buildapi.Build{})
	b.Name = "TestBuild"
	b.WithDockerStrategy()
	b.Spec.Source.Git = &buildapi.GitBuildSource{
		URI: "http://test.build/source",
	}
	b.Spec.TriggeredBy = []buildapi.BuildTriggerCause{}
	return b
}

// clearStrategy nil all strategies in the Spec since it is a
// common pattern to detect strategy by testing for non-nil.
func (b *TestBuild) clearStrategy() {
	b.Spec.Strategy.DockerStrategy = nil
	b.Spec.Strategy.SourceStrategy = nil
	b.Spec.Strategy.CustomStrategy = nil
	b.Spec.Strategy.JenkinsPipelineStrategy = nil
}

func (b *TestBuild) WithDockerStrategy() *TestBuild {
	b.clearStrategy()
	b.Spec.Strategy.DockerStrategy = &buildapi.DockerBuildStrategy{}
	return b
}

func (b *TestBuild) WithSourceStrategy() *TestBuild {
	b.clearStrategy()
	strategy := &buildapi.SourceBuildStrategy{}
	strategy.From.Name = "builder/image"
	strategy.From.Kind = "DockerImage"
	b.Spec.Strategy.SourceStrategy = strategy
	return b
}

func (b *TestBuild) WithCustomStrategy() *TestBuild {
	b.clearStrategy()
	strategy := &buildapi.CustomBuildStrategy{}
	strategy.From.Name = "builder/image"
	strategy.From.Kind = "DockerImage"
	b.Spec.Strategy.CustomStrategy = strategy
	return b
}

func (b *TestBuild) WithImageLabels(labels []buildapi.ImageLabel) *TestBuild {
	b.Spec.Output.ImageLabels = labels
	return b
}

func (b *TestBuild) WithNodeSelector(ns map[string]string) *TestBuild {
	b.Spec.NodeSelector = ns
	return b
}

func (b *TestBuild) AsBuild() *buildapi.Build {
	return (*buildapi.Build)(b)
}
