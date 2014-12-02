package util

import (
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/openshift/origin/pkg/build/api"
)

func TestGenerateBuildFromConfig(t *testing.T) {
	source := mockSource()
	strategy := mockStrategy()
	output := mockOutput()

	bc := &api.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test-build-config",
		},
		Parameters: api.BuildParameters{
			Source: source,
			Revision: &api.SourceRevision{
				Type: api.BuildSourceGit,
				Git: &api.GitSourceRevision{
					Commit: "1234",
				},
			},
			Strategy: strategy,
			Output:   output,
		},
	}
	revision := &api.SourceRevision{
		Type: api.BuildSourceGit,
		Git: &api.GitSourceRevision{
			Commit: "abcd",
		},
	}
	build := GenerateBuildFromConfig(bc, revision)
	if !reflect.DeepEqual(source, build.Parameters.Source) {
		t.Errorf("Build source does not match BuildConfig source")
	}
	if !reflect.DeepEqual(strategy, build.Parameters.Strategy) {
		t.Errorf("Build strategy does not match BuildConfig strategy")
	}
	if !reflect.DeepEqual(output, build.Parameters.Output) {
		t.Errorf("Build output does not match BuildConfig output")
	}
	if !reflect.DeepEqual(revision, build.Parameters.Revision) {
		t.Errorf("Build revision does not match passed in revision")
	}
}

func TestGenerateBuildFromBuild(t *testing.T) {
	source := mockSource()
	strategy := mockStrategy()
	output := mockOutput()

	build := &api.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test-build",
		},
		Parameters: api.BuildParameters{
			Source: source,
			Revision: &api.SourceRevision{
				Type: api.BuildSourceGit,
				Git: &api.GitSourceRevision{
					Commit: "1234",
				},
			},
			Strategy: strategy,
			Output:   output,
		},
	}
	newBuild := GenerateBuildFromBuild(build)
	if !reflect.DeepEqual(build.Parameters, newBuild.Parameters) {
		t.Errorf("Build parameters does not match the original Build parameters")
	}
	if !reflect.DeepEqual(build.ObjectMeta.Labels, newBuild.ObjectMeta.Labels) {
		t.Errorf("Build labels does not match the original Build labels")
	}
}

func mockSource() api.BuildSource {
	return api.BuildSource{
		Type: api.BuildSourceGit,
		Git: &api.GitBuildSource{
			URI: "http://test.repository/namespace/name",
			Ref: "test-tag",
		},
	}
}

func mockStrategy() api.BuildStrategy {
	return api.BuildStrategy{
		Type: api.DockerBuildStrategyType,
		DockerStrategy: &api.DockerBuildStrategy{
			ContextDir: "/test/dir",
			NoCache:    true,
		},
	}
}

func mockOutput() api.BuildOutput {
	return api.BuildOutput{
		Registry: "http://localhost:5000",
		ImageTag: "test/image-tag",
	}
}
