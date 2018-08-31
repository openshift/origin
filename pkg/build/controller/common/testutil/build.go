package test

import (
	buildv1 "github.com/openshift/api/build/v1"
)

type TestBuild buildv1.Build

func Build() *TestBuild {
	b := (*TestBuild)(&buildv1.Build{})
	b.Kind = "Build"
	b.APIVersion = "build.openshift.io/v1"
	b.Name = "TestBuild"
	b.WithDockerStrategy()
	b.Spec.Source.Git = &buildv1.GitBuildSource{
		URI: "http://test.build/source",
	}
	b.Spec.TriggeredBy = []buildv1.BuildTriggerCause{}
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
	b.Spec.Strategy.DockerStrategy = &buildv1.DockerBuildStrategy{}
	return b
}

func (b *TestBuild) WithSourceStrategy() *TestBuild {
	b.clearStrategy()
	strategy := &buildv1.SourceBuildStrategy{}
	strategy.From.Name = "builder/image"
	strategy.From.Kind = "DockerImage"
	b.Spec.Strategy.SourceStrategy = strategy
	return b
}

func (b *TestBuild) WithCustomStrategy() *TestBuild {
	b.clearStrategy()
	strategy := &buildv1.CustomBuildStrategy{}
	strategy.From.Name = "builder/image"
	strategy.From.Kind = "DockerImage"
	b.Spec.Strategy.CustomStrategy = strategy
	return b
}

func (b *TestBuild) WithImageLabels(labels []buildv1.ImageLabel) *TestBuild {
	b.Spec.Output.ImageLabels = labels
	return b
}

func (b *TestBuild) WithNodeSelector(ns map[string]string) *TestBuild {
	b.Spec.NodeSelector = ns
	return b
}

func (b *TestBuild) AsBuild() *buildv1.Build {
	return (*buildv1.Build)(b)
}
