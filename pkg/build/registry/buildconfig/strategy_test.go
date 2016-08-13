package buildconfig

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

func testBC() *buildapi.BuildConfig {
	return &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "config-id", Namespace: "namespace"},
		Spec: buildapi.BuildConfigSpec{
			RunPolicy: buildapi.BuildRunPolicySerial,
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					GitHubWebHook: &buildapi.WebHookTrigger{Secret: "12345"},
					Type:          buildapi.GitHubWebHookBuildTriggerType,
				},
				{
					ImageChange: &buildapi.ImageChangeTrigger{
						From: &kapi.ObjectReference{
							Kind: "ImageStreamTag",
							Name: "testimage:latest",
						},
					},
				},
				{
					Type: "unknown",
				},
			},
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
		Status: buildapi.BuildConfigStatus{
			LastVersion: 10,
		},
	}
}

func TestBuildConfigStrategy(t *testing.T) {
	ctx := kapi.NewDefaultContext()
	if !Strategy.NamespaceScoped() {
		t.Errorf("BuildConfig is namespace scoped")
	}
	if Strategy.AllowCreateOnUpdate() {
		t.Errorf("BuildConfig should not allow create on update")
	}
	buildConfig := testBC()
	newBC := testBC()

	// Ensure PrepareForCreate and Validation work
	Strategy.PrepareForCreate(buildConfig)
	errs := Strategy.Validate(ctx, buildConfig)
	if len(errs) != 0 {
		t.Errorf("Unexpected error validating %v", errs)
	}

	// lastversion cannot go backwards
	newBC.Status.LastVersion = 9
	Strategy.PrepareForUpdate(newBC, buildConfig)
	if newBC.Status.LastVersion != buildConfig.Status.LastVersion {
		t.Errorf("Expected version=%d, got %d", buildConfig.Status.LastVersion, newBC.Status.LastVersion)
	}

	// lastversion can go forwards
	newBC.Status.LastVersion = 11
	Strategy.PrepareForUpdate(newBC, buildConfig)
	if newBC.Status.LastVersion != 11 {
		t.Errorf("Expected version=%d, got %d", 11, newBC.Status.LastVersion)
	}

	Strategy.PrepareForCreate(buildConfig)
	errs = Strategy.Validate(ctx, buildConfig)
	if len(errs) != 0 {
		t.Errorf("Unexpected error validating %v", errs)
	}

	invalidBuildConfig := &buildapi.BuildConfig{}
	errs = Strategy.Validate(ctx, invalidBuildConfig)
	if len(errs) == 0 {
		t.Errorf("Expected error validating")
	}
}

func TestBuildConfigStatusUpdate(t *testing.T) {
	original := testBC()

	tests := []struct {
		updated  func() *buildapi.BuildConfig
		validate func(bc *buildapi.BuildConfig)
	}{
		{
			// Ensure spec is not changed
			updated: func() *buildapi.BuildConfig {
				bc := testBC()
				bc.Spec.CommonSpec.Source.Git.URI = "https://github.com/different/repo"
				return bc
			},
			validate: func(bc *buildapi.BuildConfig) {
				if bc.Spec.CommonSpec.Source.Git.URI != original.Spec.CommonSpec.Source.Git.URI {
					t.Errorf("unexpected change to source URI. Got: %s, Expected: %s",
						bc.Spec.CommonSpec.Source.Git.URI,
						original.Spec.CommonSpec.Source.Git.URI)
				}
			},
		},
		{
			// Ensure status is updated
			updated: func() *buildapi.BuildConfig {
				bc := testBC()
				bc.Status.LastVersion = 100
				return bc
			},
			validate: func(bc *buildapi.BuildConfig) {
				if bc.Status.LastVersion != 100 {
					t.Errorf("expected status to be updated to %d, got: %d", 100, bc.Status.LastVersion)
				}
			},
		},
		{
			// Ensure that trigger is updated
			updated: func() *buildapi.BuildConfig {
				bc := testBC()
				bc.Spec.Triggers[1].ImageChange.LastTriggeredImageID = "last_triggered_image_id"
				return bc
			},
			validate: func(bc *buildapi.BuildConfig) {
				if bc.Spec.Triggers[1].ImageChange.LastTriggeredImageID != "last_triggered_image_id" {
					t.Errorf("expected an update to LastTriggeredImageID. Got: %s",
						bc.Spec.Triggers[1].ImageChange.LastTriggeredImageID)
				}
			},
		},
	}
	for _, test := range tests {
		newBC := test.updated()
		StatusStrategy.PrepareForUpdate(newBC, original)
		test.validate(newBC)
	}
}
