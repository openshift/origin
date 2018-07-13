package describe

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"text/tabwriter"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kfake "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	api "github.com/openshift/origin/pkg/api"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/apis/autoscaling/install"
	_ "k8s.io/kubernetes/pkg/apis/batch/install"
	_ "k8s.io/kubernetes/pkg/apis/core/install"
	_ "k8s.io/kubernetes/pkg/apis/extensions/install"
)

type describeClient struct {
	T         *testing.T
	Namespace string
	Err       error
}

// DescriberCoverageExceptions is the list of API types that do NOT have corresponding describers
// If you add something to this list, explain why it doesn't need validation.  waaaa is not a valid
// reason.
var DescriberCoverageExceptions = []reflect.Type{
	reflect.TypeOf(&buildapi.BuildLog{}),                              // normal users don't ever look at these
	reflect.TypeOf(&buildapi.BuildLogOptions{}),                       // normal users don't ever look at these
	reflect.TypeOf(&buildapi.BinaryBuildRequestOptions{}),             // normal users don't ever look at these
	reflect.TypeOf(&buildapi.BuildRequest{}),                          // normal users don't ever look at these
	reflect.TypeOf(&appsapi.DeploymentConfigRollback{}),               // normal users don't ever look at these
	reflect.TypeOf(&appsapi.DeploymentLog{}),                          // normal users don't ever look at these
	reflect.TypeOf(&appsapi.DeploymentLogOptions{}),                   // normal users don't ever look at these
	reflect.TypeOf(&appsapi.DeploymentRequest{}),                      // normal users don't ever look at these
	reflect.TypeOf(&imageapi.DockerImage{}),                           // not a top level resource
	reflect.TypeOf(&imageapi.ImageStreamImport{}),                     // normal users don't ever look at these
	reflect.TypeOf(&oauthapi.OAuthAccessToken{}),                      // normal users don't ever look at these
	reflect.TypeOf(&oauthapi.OAuthAuthorizeToken{}),                   // normal users don't ever look at these
	reflect.TypeOf(&oauthapi.OAuthClientAuthorization{}),              // normal users don't ever look at these
	reflect.TypeOf(&projectapi.ProjectRequest{}),                      // normal users don't ever look at these
	reflect.TypeOf(&templateapi.TemplateInstance{}),                   // normal users don't ever look at these
	reflect.TypeOf(&templateapi.BrokerTemplateInstance{}),             // normal users don't ever look at these
	reflect.TypeOf(&authorizationapi.IsPersonalSubjectAccessReview{}), // not a top level resource
	// ATM image signature doesn't provide any human readable information
	reflect.TypeOf(&imageapi.ImageSignature{}),

	// these resources can't be "GET"ed, so you can't make a describer for them
	reflect.TypeOf(&authorizationapi.SubjectAccessReviewResponse{}),
	reflect.TypeOf(&authorizationapi.ResourceAccessReviewResponse{}),
	reflect.TypeOf(&authorizationapi.SubjectAccessReview{}),
	reflect.TypeOf(&authorizationapi.ResourceAccessReview{}),
	reflect.TypeOf(&authorizationapi.LocalSubjectAccessReview{}),
	reflect.TypeOf(&authorizationapi.LocalResourceAccessReview{}),
	reflect.TypeOf(&authorizationapi.SelfSubjectRulesReview{}),
	reflect.TypeOf(&authorizationapi.SubjectRulesReview{}),
	reflect.TypeOf(&securityapi.PodSecurityPolicySubjectReview{}),
	reflect.TypeOf(&securityapi.PodSecurityPolicySelfSubjectReview{}),
	reflect.TypeOf(&securityapi.PodSecurityPolicyReview{}),
	reflect.TypeOf(&oauthapi.OAuthRedirectReference{}),
}

// MissingDescriberCoverageExceptions is the list of types that were missing describer methods when I started
// You should never add to this list
// TODO describers should be added for these types
var MissingDescriberCoverageExceptions = []reflect.Type{
	reflect.TypeOf(&imageapi.ImageStreamMapping{}),
	reflect.TypeOf(&oauthapi.OAuthClient{}),
}

func TestDescriberCoverage(t *testing.T) {

main:
	for _, apiType := range legacyscheme.Scheme.KnownTypes(api.SchemeGroupVersion) {
		if !strings.HasPrefix(apiType.PkgPath(), "github.com/openshift/origin") || strings.HasPrefix(apiType.PkgPath(), "github.com/openshift/origin/vendor/") {
			continue
		}
		// we don't describe lists
		if strings.HasSuffix(apiType.Name(), "List") {
			continue
		}

		ptrType := reflect.PtrTo(apiType)
		for _, exception := range DescriberCoverageExceptions {
			if ptrType == exception {
				continue main
			}
		}
		for _, exception := range MissingDescriberCoverageExceptions {
			if ptrType == exception {
				continue main
			}
		}

		gk := api.SchemeGroupVersion.WithKind(apiType.Name()).GroupKind()
		_, ok := DescriberFor(gk, &rest.Config{}, kfake.NewSimpleClientset(), "")
		if !ok {
			t.Errorf("missing describer for %v.  Check pkg/cmd/cli/describe/describer.go", apiType)
		}
	}
}

func TestDescribeBuildDuration(t *testing.T) {
	type testBuild struct {
		build  *buildapi.Build
		output string
	}

	// now a minute ago
	now := metav1.Now()
	minuteAgo := metav1.Unix(now.Rfc3339Copy().Time.Unix()-60, 0)
	twoMinutesAgo := metav1.Unix(now.Rfc3339Copy().Time.Unix()-120, 0)
	threeMinutesAgo := metav1.Unix(now.Rfc3339Copy().Time.Unix()-180, 0)

	tests := []testBuild{
		{ // 0 - build new
			&buildapi.Build{
				ObjectMeta: metav1.ObjectMeta{CreationTimestamp: minuteAgo},
				Status: buildapi.BuildStatus{
					Phase: buildapi.BuildPhaseNew,
				},
			},
			"waiting for 1m",
		},
		{ // 1 - build pending
			&buildapi.Build{
				ObjectMeta: metav1.ObjectMeta{CreationTimestamp: minuteAgo},
				Status: buildapi.BuildStatus{
					Phase: buildapi.BuildPhasePending,
				},
			},
			"waiting for 1m",
		},
		{ // 2 - build running
			&buildapi.Build{
				ObjectMeta: metav1.ObjectMeta{CreationTimestamp: twoMinutesAgo},
				Status: buildapi.BuildStatus{
					StartTimestamp: &minuteAgo,
					Phase:          buildapi.BuildPhaseRunning,
				},
			},
			"running for 1m",
		},
		{ // 3 - build completed
			&buildapi.Build{
				ObjectMeta: metav1.ObjectMeta{CreationTimestamp: threeMinutesAgo},
				Status: buildapi.BuildStatus{
					StartTimestamp:      &twoMinutesAgo,
					CompletionTimestamp: &minuteAgo,
					Phase:               buildapi.BuildPhaseComplete,
				},
			},
			"1m",
		},
		{ // 4 - build failed
			&buildapi.Build{
				ObjectMeta: metav1.ObjectMeta{CreationTimestamp: threeMinutesAgo},
				Status: buildapi.BuildStatus{
					StartTimestamp:      &twoMinutesAgo,
					CompletionTimestamp: &minuteAgo,
					Phase:               buildapi.BuildPhaseFailed,
				},
			},
			"1m",
		},
		{ // 5 - build error
			&buildapi.Build{
				ObjectMeta: metav1.ObjectMeta{CreationTimestamp: threeMinutesAgo},
				Status: buildapi.BuildStatus{
					StartTimestamp:      &twoMinutesAgo,
					CompletionTimestamp: &minuteAgo,
					Phase:               buildapi.BuildPhaseError,
				},
			},
			"1m",
		},
		{ // 6 - build cancelled before running, start time wasn't set yet
			&buildapi.Build{
				ObjectMeta: metav1.ObjectMeta{CreationTimestamp: threeMinutesAgo},
				Status: buildapi.BuildStatus{
					CompletionTimestamp: &minuteAgo,
					Phase:               buildapi.BuildPhaseCancelled,
				},
			},
			"waited for 2m",
		},
		{ // 7 - build cancelled while running, start time is set already
			&buildapi.Build{
				ObjectMeta: metav1.ObjectMeta{CreationTimestamp: threeMinutesAgo},
				Status: buildapi.BuildStatus{
					StartTimestamp:      &twoMinutesAgo,
					CompletionTimestamp: &minuteAgo,
					Phase:               buildapi.BuildPhaseCancelled,
				},
			},
			"1m",
		},
		{ // 8 - build failed before running, start time wasn't set yet
			&buildapi.Build{
				ObjectMeta: metav1.ObjectMeta{CreationTimestamp: threeMinutesAgo},
				Status: buildapi.BuildStatus{
					CompletionTimestamp: &minuteAgo,
					Phase:               buildapi.BuildPhaseFailed,
				},
			},
			"waited for 2m",
		},
		{ // 9 - build error before running, start time wasn't set yet
			&buildapi.Build{
				ObjectMeta: metav1.ObjectMeta{CreationTimestamp: threeMinutesAgo},
				Status: buildapi.BuildStatus{
					CompletionTimestamp: &minuteAgo,
					Phase:               buildapi.BuildPhaseError,
				},
			},
			"waited for 2m",
		},
	}

	for i, tc := range tests {
		if actual, expected := describeBuildDuration(tc.build), tc.output; !strings.Contains(actual, expected) {
			t.Errorf("(%d) expected duration output %s, got %s", i, expected, actual)
		}
	}
}

func mkPod(status kapi.PodPhase, exitCode int) *kapi.Pod {
	return &kapi.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "PodName"},
		Status: kapi.PodStatus{
			Phase: status,
			ContainerStatuses: []kapi.ContainerStatus{
				{
					State: kapi.ContainerState{
						Terminated: &kapi.ContainerStateTerminated{ExitCode: int32(exitCode)},
					},
				},
			},
		},
	}
}

func TestDescribePostCommitHook(t *testing.T) {
	tests := []struct {
		hook buildapi.BuildPostCommitSpec
		want string
	}{
		{
			hook: buildapi.BuildPostCommitSpec{},
			want: "",
		},
		{
			hook: buildapi.BuildPostCommitSpec{
				Script: "go test",
			},
			want: `"/bin/sh", "-ic", "go test"`,
		},
		{
			hook: buildapi.BuildPostCommitSpec{
				Command: []string{"go", "test"},
			},
			want: `"go", "test"`,
		},
		{
			hook: buildapi.BuildPostCommitSpec{
				Args: []string{"go", "test"},
			},
			want: `"<image-entrypoint>", "go", "test"`,
		},
		{
			hook: buildapi.BuildPostCommitSpec{
				Script: `go test "$@"`,
				Args:   []string{"-v", "-timeout", "2s"},
			},
			want: `"/bin/sh", "-ic", "go test \"$@\"", "/bin/sh", "-v", "-timeout", "2s"`,
		},
		{
			hook: buildapi.BuildPostCommitSpec{
				Command: []string{"go", "test"},
				Args:    []string{"-v", "-timeout", "2s"},
			},
			want: `"go", "test", "-v", "-timeout", "2s"`,
		},
		{
			// Invalid hook: Script and Command are not allowed
			// together. For printing, Script takes precedence.
			hook: buildapi.BuildPostCommitSpec{
				Script:  "go test -v",
				Command: []string{"go", "test"},
			},
			want: `"/bin/sh", "-ic", "go test -v"`,
		},
	}
	for _, tt := range tests {
		var b bytes.Buffer
		out := tabwriter.NewWriter(&b, 0, 8, 0, '\t', 0)
		describePostCommitHook(tt.hook, out)
		if err := out.Flush(); err != nil {
			t.Fatalf("%+v: flush error: %v", tt.hook, err)
		}
		var want string
		if tt.want != "" {
			want = fmt.Sprintf("Post Commit Hook:\t[%s]\n", tt.want)
		}
		if got := b.String(); got != want {
			t.Errorf("describePostCommitHook(%+v, out) = %q, want %q", tt.hook, got, want)
		}
	}
}

func TestDescribeBuildSpec(t *testing.T) {
	tests := []struct {
		spec buildapi.BuildSpec
		want string
	}{
		{
			spec: buildapi.BuildSpec{
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
			want: "URL",
		},
		{
			spec: buildapi.BuildSpec{
				CommonSpec: buildapi.CommonSpec{
					Source: buildapi.BuildSource{},
					Strategy: buildapi.BuildStrategy{
						SourceStrategy: &buildapi.SourceBuildStrategy{
							From: kapi.ObjectReference{
								Kind: "DockerImage",
								Name: "myimage:tag",
							},
						},
					},
					Output: buildapi.BuildOutput{
						To: &kapi.ObjectReference{
							Kind: "DockerImage",
							Name: "repository/data",
						},
					},
				},
			},
			want: "Empty Source",
		},
		{
			spec: buildapi.BuildSpec{
				CommonSpec: buildapi.CommonSpec{
					Source: buildapi.BuildSource{},
					Strategy: buildapi.BuildStrategy{
						CustomStrategy: &buildapi.CustomBuildStrategy{
							From: kapi.ObjectReference{
								Kind: "DockerImage",
								Name: "myimage:tag",
							},
						},
					},
					Output: buildapi.BuildOutput{
						To: &kapi.ObjectReference{
							Kind: "DockerImage",
							Name: "repository/data",
						},
					},
				},
			},
			want: "Empty Source",
		},
		{
			spec: buildapi.BuildSpec{
				CommonSpec: buildapi.CommonSpec{
					Source: buildapi.BuildSource{},
					Strategy: buildapi.BuildStrategy{
						JenkinsPipelineStrategy: &buildapi.JenkinsPipelineBuildStrategy{
							Jenkinsfile: "openshiftBuild",
						},
					},
				},
			},
			want: "openshiftBuild",
		},
	}
	for _, tt := range tests {
		var b bytes.Buffer
		out := tabwriter.NewWriter(&b, 0, 8, 0, '\t', 0)
		describeCommonSpec(tt.spec.CommonSpec, out)
		if err := out.Flush(); err != nil {
			t.Fatalf("%+v: flush error: %v", tt.spec, err)
		}
		if got := b.String(); !strings.Contains(got, tt.want) {
			t.Errorf("describeBuildSpec(%+v, out) = %q, should contain %q", tt.spec, got, tt.want)
		}
	}
}
