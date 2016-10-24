package describe

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"text/tabwriter"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/kubectl"

	api "github.com/openshift/origin/pkg/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/client/testclient"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	projectapi "github.com/openshift/origin/pkg/project/api"
	securityapi "github.com/openshift/origin/pkg/security/api"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/apis/autoscaling/install"
	_ "k8s.io/kubernetes/pkg/apis/batch/install"
	_ "k8s.io/kubernetes/pkg/apis/extensions/install"
)

type describeClient struct {
	T         *testing.T
	Namespace string
	Err       error
	*testclient.Fake
}

// DescriberCoverageExceptions is the list of API types that do NOT have corresponding describers
// If you add something to this list, explain why it doesn't need validation.  waaaa is not a valid
// reason.
var DescriberCoverageExceptions = []reflect.Type{
	reflect.TypeOf(&buildapi.BuildLog{}),                              // normal users don't ever look at these
	reflect.TypeOf(&buildapi.BuildLogOptions{}),                       // normal users don't ever look at these
	reflect.TypeOf(&buildapi.BinaryBuildRequestOptions{}),             // normal users don't ever look at these
	reflect.TypeOf(&buildapi.BuildRequest{}),                          // normal users don't ever look at these
	reflect.TypeOf(&deployapi.DeploymentConfigRollback{}),             // normal users don't ever look at these
	reflect.TypeOf(&deployapi.DeploymentLog{}),                        // normal users don't ever look at these
	reflect.TypeOf(&deployapi.DeploymentLogOptions{}),                 // normal users don't ever look at these
	reflect.TypeOf(&deployapi.DeploymentRequest{}),                    // normal users don't ever look at these
	reflect.TypeOf(&imageapi.DockerImage{}),                           // not a top level resource
	reflect.TypeOf(&imageapi.ImageStreamImport{}),                     // normal users don't ever look at these
	reflect.TypeOf(&oauthapi.OAuthAccessToken{}),                      // normal users don't ever look at these
	reflect.TypeOf(&oauthapi.OAuthAuthorizeToken{}),                   // normal users don't ever look at these
	reflect.TypeOf(&oauthapi.OAuthClientAuthorization{}),              // normal users don't ever look at these
	reflect.TypeOf(&projectapi.ProjectRequest{}),                      // normal users don't ever look at these
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
	c := &client.Client{}

main:
	for _, apiType := range kapi.Scheme.KnownTypes(api.SchemeGroupVersion) {
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

		_, ok := DescriberFor(api.SchemeGroupVersion.WithKind(apiType.Name()).GroupKind(), c, &ktestclient.Fake{}, "")
		if !ok {
			t.Errorf("missing describer for %v.  Check pkg/cmd/cli/describe/describer.go", apiType)
		}
	}
}

func TestDescribers(t *testing.T) {
	fake := &testclient.Fake{}
	fakeKube := &ktestclient.Fake{}
	c := &describeClient{T: t, Namespace: "foo", Fake: fake}

	testCases := []struct {
		d    kubectl.Describer
		name string
	}{
		{&BuildDescriber{c, fakeKube}, "bar"},
		{&BuildConfigDescriber{c, fakeKube, ""}, "bar"},
		{&ImageDescriber{c}, "bar"},
		{&ImageStreamDescriber{c}, "bar"},
		{&ImageStreamTagDescriber{c}, "bar:latest"},
		{&ImageStreamImageDescriber{c}, "bar@sha256:other"},
		{&RouteDescriber{c, fakeKube}, "bar"},
		{&ProjectDescriber{c, fakeKube}, "bar"},
		{&PolicyDescriber{c}, "bar"},
		{&PolicyBindingDescriber{c}, "bar"},
		{&TemplateDescriber{c, nil, nil, nil}, "bar"},
	}

	for _, test := range testCases {
		out, err := test.d.Describe("foo", test.name, kubectl.DescriberSettings{})
		if err != nil {
			t.Errorf("unexpected error for %v: %v", test.d, err)
		}
		if !strings.Contains(out, "Name:") || !strings.Contains(out, "Labels:") {
			t.Errorf("unexpected out: %s", out)
		}
	}
}

func TestDescribeBuildDuration(t *testing.T) {
	type testBuild struct {
		build  *buildapi.Build
		output string
	}

	creation := unversioned.Date(2015, time.April, 9, 6, 0, 0, 0, time.Local)
	// now a minute ago
	minuteAgo := unversioned.Unix(unversioned.Now().Rfc3339Copy().Time.Unix()-60, 0)
	start := unversioned.Date(2015, time.April, 9, 6, 1, 0, 0, time.Local)
	completion := unversioned.Date(2015, time.April, 9, 6, 2, 0, 0, time.Local)
	duration := completion.Rfc3339Copy().Time.Sub(start.Rfc3339Copy().Time)
	zeroDuration := time.Duration(0)

	tests := []testBuild{
		{ // 0 - build new
			&buildapi.Build{
				ObjectMeta: kapi.ObjectMeta{CreationTimestamp: minuteAgo},
				Status: buildapi.BuildStatus{
					Phase:    buildapi.BuildPhaseNew,
					Duration: zeroDuration,
				},
			},
			"waiting for 1m",
		},
		{ // 1 - build pending
			&buildapi.Build{
				ObjectMeta: kapi.ObjectMeta{CreationTimestamp: minuteAgo},
				Status: buildapi.BuildStatus{
					Phase:    buildapi.BuildPhasePending,
					Duration: zeroDuration,
				},
			},
			"waiting for 1m",
		},
		{ // 2 - build running
			&buildapi.Build{
				ObjectMeta: kapi.ObjectMeta{CreationTimestamp: creation},
				Status: buildapi.BuildStatus{
					StartTimestamp: &start,
					Phase:          buildapi.BuildPhaseRunning,
					Duration:       duration,
				},
			},
			"running for 1m",
		},
		{ // 3 - build completed
			&buildapi.Build{
				ObjectMeta: kapi.ObjectMeta{CreationTimestamp: creation},
				Status: buildapi.BuildStatus{
					StartTimestamp:      &start,
					CompletionTimestamp: &completion,
					Phase:               buildapi.BuildPhaseComplete,
					Duration:            duration,
				},
			},
			"1m",
		},
		{ // 4 - build failed
			&buildapi.Build{
				ObjectMeta: kapi.ObjectMeta{CreationTimestamp: creation},
				Status: buildapi.BuildStatus{
					StartTimestamp:      &start,
					CompletionTimestamp: &completion,
					Phase:               buildapi.BuildPhaseFailed,
					Duration:            duration,
				},
			},
			"1m",
		},
		{ // 5 - build error
			&buildapi.Build{
				ObjectMeta: kapi.ObjectMeta{CreationTimestamp: creation},
				Status: buildapi.BuildStatus{
					StartTimestamp:      &start,
					CompletionTimestamp: &completion,
					Phase:               buildapi.BuildPhaseError,
					Duration:            duration,
				},
			},
			"1m",
		},
		{ // 6 - build cancelled before running, start time wasn't set yet
			&buildapi.Build{
				ObjectMeta: kapi.ObjectMeta{CreationTimestamp: creation},
				Status: buildapi.BuildStatus{
					CompletionTimestamp: &completion,
					Phase:               buildapi.BuildPhaseCancelled,
					Duration:            duration,
				},
			},
			"waited for 2m",
		},
		{ // 7 - build cancelled while running, start time is set already
			&buildapi.Build{
				ObjectMeta: kapi.ObjectMeta{CreationTimestamp: creation},
				Status: buildapi.BuildStatus{
					StartTimestamp:      &start,
					CompletionTimestamp: &completion,
					Phase:               buildapi.BuildPhaseCancelled,
					Duration:            duration,
				},
			},
			"1m",
		},
		{ // 8 - build failed before running, start time wasn't set yet
			&buildapi.Build{
				ObjectMeta: kapi.ObjectMeta{CreationTimestamp: creation},
				Status: buildapi.BuildStatus{
					CompletionTimestamp: &completion,
					Phase:               buildapi.BuildPhaseFailed,
					Duration:            duration,
				},
			},
			"waited for 2m",
		},
		{ // 9 - build error before running, start time wasn't set yet
			&buildapi.Build{
				ObjectMeta: kapi.ObjectMeta{CreationTimestamp: creation},
				Status: buildapi.BuildStatus{
					CompletionTimestamp: &completion,
					Phase:               buildapi.BuildPhaseError,
					Duration:            duration,
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
		ObjectMeta: kapi.ObjectMeta{Name: "PodName"},
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
