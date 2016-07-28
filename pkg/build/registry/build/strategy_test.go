package build

import (
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"

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
	build := testBuild()
	Strategy.PrepareForCreate(build)
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

func testBuild() *buildapi.Build {
	return &buildapi.Build{
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
		Status: buildapi.BuildStatus{
			Phase: buildapi.BuildPhaseNew,
		},
	}
}

func TestBuildDecorator(t *testing.T) {
	build := testBuild()
	now := unversioned.Now()
	startTime := unversioned.NewTime(now.Time.Add(-1 * time.Minute))
	build.Status.StartTimestamp = &startTime
	err := Decorator(build)
	if err != nil {
		t.Errorf("Unexpected error decorating build")
	}
	if build.Status.Duration <= 0 {
		t.Errorf("Build duration should be greater than zero")
	}
}

func TestBuildStatusUpdate(t *testing.T) {
	original := testBuild()

	tests := []struct {
		updated  func() *buildapi.Build
		validate func(bc *buildapi.Build)
	}{
		{
			// Ensure spec is not changed
			updated: func() *buildapi.Build {
				b := testBuild()
				b.Spec.CommonSpec.Source.Git.URI = "https://github.com/different/repo"
				return b
			},
			validate: func(b *buildapi.Build) {
				if b.Spec.CommonSpec.Source.Git.URI != original.Spec.CommonSpec.Source.Git.URI {
					t.Errorf("unexpected change to source URI. Got: %s, Expected: %s",
						b.Spec.CommonSpec.Source.Git.URI,
						original.Spec.CommonSpec.Source.Git.URI)
				}
			},
		},
		{
			// Ensure status is updated
			updated: func() *buildapi.Build {
				b := testBuild()
				b.Status.Phase = buildapi.BuildPhasePending
				return b
			},
			validate: func(b *buildapi.Build) {
				if b.Status.Phase != buildapi.BuildPhasePending {
					t.Errorf("expected status.phase to be updated to %s, got: %s", buildapi.BuildPhasePending, b.Status.Phase)
				}
			},
		},
	}
	for _, test := range tests {
		newBuild := test.updated()
		StatusStrategy.PrepareForUpdate(newBuild, original)
		test.validate(newBuild)
	}
}
