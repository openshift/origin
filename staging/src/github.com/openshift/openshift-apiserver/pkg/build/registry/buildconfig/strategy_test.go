package buildconfig

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
)

func TestBuildConfigGroupStrategy(t *testing.T) {
	ctx := apirequest.NewDefaultContext()
	if !GroupStrategy.NamespaceScoped() {
		t.Errorf("BuildConfig is namespace scoped")
	}
	if GroupStrategy.AllowCreateOnUpdate() {
		t.Errorf("BuildConfig should not allow create on update")
	}
	buildConfig := &buildapi.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "config-id", Namespace: "namespace"},
		Spec: buildapi.BuildConfigSpec{
			RunPolicy: buildapi.BuildRunPolicySerial,
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					GitHubWebHook: &buildapi.WebHookTrigger{Secret: "12345"},
					Type:          buildapi.GitHubWebHookBuildTriggerType,
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
	newBC := &buildapi.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "config-id", Namespace: "namespace"},
		Spec: buildapi.BuildConfigSpec{
			RunPolicy: buildapi.BuildRunPolicySerial,
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					GitHubWebHook: &buildapi.WebHookTrigger{Secret: "12345"},
					Type:          buildapi.GitHubWebHookBuildTriggerType,
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
			LastVersion: 9,
		},
	}
	GroupStrategy.PrepareForCreate(ctx, buildConfig)
	errs := GroupStrategy.Validate(ctx, buildConfig)
	if len(errs) != 0 {
		t.Errorf("Unexpected error validating %v", errs)
	}
	if buildConfig.Spec.SuccessfulBuildsHistoryLimit == nil {
		t.Errorf("Expected successful limit %d, got nil", buildapi.DefaultSuccessfulBuildsHistoryLimit)
	}
	if *buildConfig.Spec.SuccessfulBuildsHistoryLimit != buildapi.DefaultSuccessfulBuildsHistoryLimit {
		t.Errorf("Expected successful limit %d, got %d", buildapi.DefaultSuccessfulBuildsHistoryLimit, *buildConfig.Spec.SuccessfulBuildsHistoryLimit)
	}
	if buildConfig.Spec.FailedBuildsHistoryLimit == nil {
		t.Errorf("Expected failed limit %d, got nil", buildapi.DefaultFailedBuildsHistoryLimit)
	}
	if *buildConfig.Spec.FailedBuildsHistoryLimit != buildapi.DefaultFailedBuildsHistoryLimit {
		t.Errorf("Expected failed limit %d, got %d", buildapi.DefaultFailedBuildsHistoryLimit, *buildConfig.Spec.FailedBuildsHistoryLimit)
	}

	// lastversion cannot go backwards
	newBC.Status.LastVersion = 9
	GroupStrategy.PrepareForUpdate(ctx, newBC, buildConfig)
	if newBC.Status.LastVersion != buildConfig.Status.LastVersion {
		t.Errorf("Expected version=%d, got %d", buildConfig.Status.LastVersion, newBC.Status.LastVersion)
	}

	// lastversion can go forwards
	newBC.Status.LastVersion = 11
	GroupStrategy.PrepareForUpdate(ctx, newBC, buildConfig)
	if newBC.Status.LastVersion != 11 {
		t.Errorf("Expected version=%d, got %d", 11, newBC.Status.LastVersion)
	}

	GroupStrategy.PrepareForCreate(ctx, buildConfig)
	errs = GroupStrategy.Validate(ctx, buildConfig)
	if len(errs) != 0 {
		t.Errorf("Unexpected error validating %v", errs)
	}

	invalidBuildConfig := &buildapi.BuildConfig{}
	errs = GroupStrategy.Validate(ctx, invalidBuildConfig)
	if len(errs) == 0 {
		t.Errorf("Expected error validating")
	}
}

func TestBuildConfigLegacyStrategy(t *testing.T) {
	ctx := apirequest.NewDefaultContext()
	if !LegacyStrategy.NamespaceScoped() {
		t.Errorf("BuildConfig is namespace scoped")
	}
	if LegacyStrategy.AllowCreateOnUpdate() {
		t.Errorf("BuildConfig should not allow create on update")
	}
	buildConfig := &buildapi.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "config-id", Namespace: "namespace"},
		Spec: buildapi.BuildConfigSpec{
			RunPolicy: buildapi.BuildRunPolicySerial,
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					GitHubWebHook: &buildapi.WebHookTrigger{Secret: "12345"},
					Type:          buildapi.GitHubWebHookBuildTriggerType,
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
	newBC := &buildapi.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "config-id", Namespace: "namespace"},
		Spec: buildapi.BuildConfigSpec{
			RunPolicy: buildapi.BuildRunPolicySerial,
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					GitHubWebHook: &buildapi.WebHookTrigger{Secret: "12345"},
					Type:          buildapi.GitHubWebHookBuildTriggerType,
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
			LastVersion: 9,
		},
	}
	LegacyStrategy.PrepareForCreate(ctx, buildConfig)
	errs := LegacyStrategy.Validate(ctx, buildConfig)
	if len(errs) != 0 {
		t.Errorf("Unexpected error validating %v", errs)
	}
	if buildConfig.Spec.SuccessfulBuildsHistoryLimit != nil {
		t.Errorf("Expected successful limit to be nil, got %d", *buildConfig.Spec.SuccessfulBuildsHistoryLimit)
	}
	if buildConfig.Spec.FailedBuildsHistoryLimit != nil {
		t.Errorf("Expected failed limit to be nil, got %d", *buildConfig.Spec.FailedBuildsHistoryLimit)
	}

	// lastversion cannot go backwards
	newBC.Status.LastVersion = 9
	LegacyStrategy.PrepareForUpdate(ctx, newBC, buildConfig)
	if newBC.Status.LastVersion != buildConfig.Status.LastVersion {
		t.Errorf("Expected version=%d, got %d", buildConfig.Status.LastVersion, newBC.Status.LastVersion)
	}

	// lastversion can go forwards
	newBC.Status.LastVersion = 11
	LegacyStrategy.PrepareForUpdate(ctx, newBC, buildConfig)
	if newBC.Status.LastVersion != 11 {
		t.Errorf("Expected version=%d, got %d", 11, newBC.Status.LastVersion)
	}

	LegacyStrategy.PrepareForCreate(ctx, buildConfig)
	errs = LegacyStrategy.Validate(ctx, buildConfig)
	if len(errs) != 0 {
		t.Errorf("Unexpected error validating %v", errs)
	}

	invalidBuildConfig := &buildapi.BuildConfig{}
	errs = LegacyStrategy.Validate(ctx, invalidBuildConfig)
	if len(errs) == 0 {
		t.Errorf("Expected error validating")
	}
}
