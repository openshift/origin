package test

import (
	buildapi "github.com/openshift/origin/pkg/build/api"
)

type TestBuild buildapi.Build

func Build() *TestBuild {
	b := (*TestBuild)(&buildapi.Build{})
	b.Name = "TestBuild"
	b.Spec.Strategy.DockerStrategy = &buildapi.DockerBuildStrategy{}
	b.Spec.Source.Git = &buildapi.GitBuildSource{
		URI: "http://test.build/source",
	}
	return b
}

func (b *TestBuild) WithDockerStrategy() *TestBuild {
	b.Spec.Strategy.DockerStrategy = &buildapi.DockerBuildStrategy{}
	return b
}

func (b *TestBuild) WithSourceStrategy() *TestBuild {
	strategy := &buildapi.SourceBuildStrategy{}
	strategy.From.Name = "builder/image"
	strategy.From.Kind = "DockerImage"
	b.Spec.Strategy.SourceStrategy = strategy
	return b
}

func (b *TestBuild) WithCustomStrategy() *TestBuild {
	strategy := &buildapi.CustomBuildStrategy{}
	strategy.From.Name = "builder/image"
	strategy.From.Kind = "DockerImage"
	b.Spec.Strategy.CustomStrategy = strategy
	return b
}

func (b *TestBuild) AsBuild() *buildapi.Build {
	return (*buildapi.Build)(b)
}
