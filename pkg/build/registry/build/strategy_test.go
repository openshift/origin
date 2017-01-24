package build

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

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
		Spec: buildapi.BuildSpec{
			CommonSpec: buildapi.CommonSpec{
				Source: buildapi.BuildSource{
					Git: &buildapi.GitBuildSource{
						URI: "http://github.com/my/repository",
					},
					ContextDir: "context",
				},
				Strategy: buildapi.BuildStrategy{
					DockerStrategy: &buildapi.DockerBuildStrategy{},
				},
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "repository/data",
					},
				},
			},
		},
	}
	Strategy.PrepareForCreate(ctx, build)
	if len(build.Status.Phase) == 0 || build.Status.Phase != buildapi.BuildPhaseNew {
		t.Errorf("Build phase is not New")
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
