package util

import (
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/openshift/origin/pkg/build/api"
)

func TestGenerateBuild(t *testing.T) {
	source := api.BuildSource{
		Type: api.BuildSourceGit,
		Git: &api.GitBuildSource{
			URI: "http://test.repository/namespace/name",
			Ref: "test-tag",
		},
	}
	strategy := api.BuildStrategy{
		Type: api.DockerBuildStrategyType,
		DockerStrategy: &api.DockerBuildStrategy{
			ContextDir: "/test/dir",
			NoCache:    true,
		},
	}
	output := api.BuildOutput{
		Registry: "http://localhost:5000",
		ImageTag: "test/image-tag",
	}
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
	build := GenerateBuild(bc, revision)
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
