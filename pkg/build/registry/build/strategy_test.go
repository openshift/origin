package build

import (
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

func TestBuildStrategy(t *testing.T) {
	ctx := kapi.NewDefaultContext()
	if !Strategy.NamespaceScoped() {
		t.Errorf("Build is namespace scoped")
	}
	if Strategy.AllowCreateOnUpdate() {
		t.Errorf("Build should not allow create on update")
	}
	build := &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{Name: "buildid", Namespace: "default"},
		Parameters: buildapi.BuildParameters{
			Source: buildapi.BuildSource{
				Type: buildapi.BuildSourceGit,
				Git: &buildapi.GitBuildSource{
					URI: "http://github.com/my/repository",
				},
				ContextDir: "context",
			},
			Strategy: buildapi.BuildStrategy{
				Type:           buildapi.DockerBuildStrategyType,
				DockerStrategy: &buildapi.DockerBuildStrategy{},
			},
			Output: buildapi.BuildOutput{
				DockerImageReference: "repository/data",
			},
		},
	}
	Strategy.PrepareForCreate(build)
	if len(build.Status) == 0 || build.Status != buildapi.BuildStatusNew {
		t.Errorf("Build status is not New")
	}
	errs := Strategy.Validate(ctx, build)
	if len(errs) != 0 {
		t.Errorf("Unexpected error validating %v", errs)
	}

	build.ResourceVersion = "foo"
	errs = Strategy.ValidateUpdate(ctx, build, build)
	if len(errs) != 0 {
		t.Errorf("Unexpected error validating %v", errs)
	}
	invalidBuild := &buildapi.Build{}
	errs = Strategy.Validate(ctx, invalidBuild)
	if len(errs) == 0 {
		t.Errorf("Expected error validating")
	}
}

func TestBuildDecorator(t *testing.T) {
	build := &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{Name: "buildid", Namespace: "default"},
		Parameters: buildapi.BuildParameters{
			Source: buildapi.BuildSource{
				Type: buildapi.BuildSourceGit,
				Git: &buildapi.GitBuildSource{
					URI: "http://github.com/my/repository",
				},
				ContextDir: "context",
			},
			Strategy: buildapi.BuildStrategy{
				Type:           buildapi.DockerBuildStrategyType,
				DockerStrategy: &buildapi.DockerBuildStrategy{},
			},
			Output: buildapi.BuildOutput{
				DockerImageReference: "repository/data",
			},
		},
		Status: buildapi.BuildStatusNew,
	}
	now := util.Now()
	startTime := util.NewTime(now.Time.Add(-1 * time.Minute))
	build.StartTimestamp = &startTime
	err := Decorator(build)
	if err != nil {
		t.Errorf("Unexpected error decorating build")
	}
	if build.Duration <= 0 {
		t.Errorf("Build duration should be greater than zero")
	}
}
