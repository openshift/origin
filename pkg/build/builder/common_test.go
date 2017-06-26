package builder

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/generate/git"
)

func TestBuildInfo(t *testing.T) {
	b := &buildapi.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample-app",
			Namespace: "default",
		},
		Spec: buildapi.BuildSpec{
			CommonSpec: buildapi.CommonSpec{
				Source: buildapi.BuildSource{
					Git: &buildapi.GitBuildSource{
						URI: "github.com/openshift/sample-app",
						Ref: "master",
					},
				},
				Strategy: buildapi.BuildStrategy{
					SourceStrategy: &buildapi.SourceBuildStrategy{
						Env: []kapi.EnvVar{
							{Name: "RAILS_ENV", Value: "production"},
						},
					},
				},
			},
		},
	}
	sourceInfo := &git.SourceInfo{}
	sourceInfo.CommitID = "1575a90c569a7cc0eea84fbd3304d9df37c9f5ee"
	got := buildInfo(b, sourceInfo)
	want := []KeyValue{
		{"OPENSHIFT_BUILD_NAME", "sample-app"},
		{"OPENSHIFT_BUILD_NAMESPACE", "default"},
		{"OPENSHIFT_BUILD_SOURCE", "github.com/openshift/sample-app"},
		{"OPENSHIFT_BUILD_REFERENCE", "master"},
		{"OPENSHIFT_BUILD_COMMIT", "1575a90c569a7cc0eea84fbd3304d9df37c9f5ee"},
		{"RAILS_ENV", "production"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildInfo(%+v) = %+v; want %+v", b, got, want)
	}

	b.Spec.Revision = &buildapi.SourceRevision{
		Git: &buildapi.GitSourceRevision{
			Commit: "1575a90c569a7cc0eea84fbd3304d9df37c9f5ee",
		},
	}
	got = buildInfo(b, nil)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildInfo(%+v) = %+v; want %+v", b, got, want)
	}

}

func TestRandomBuildTag(t *testing.T) {
	tests := []struct {
		namespace, name string
		want            string
	}{
		{"test", "build-1", "test/build-1:f1f85ff5"},
		// For long build namespace + build name, the returned random build tag
		// would be longer than the limit of reference.NameTotalLengthMax (255
		// chars). We do not truncate the repository name because it could create an
		// invalid repository name (e.g., namespace=abc, name=d, repo=abc/d,
		// trucated=abc/ -> invalid), so we simply take a SHA1 hash of the
		// repository name (which is guaranteed to be a valid repository name) and
		// preserve the random tag.
		{
			"namespace" + strings.Repeat(".namespace", 20),
			"name" + strings.Repeat(".name", 20),
			"47c1d5c686ce4563521c625457e79ca23c07bc27:f1f85ff5",
		},
	}
	for _, tt := range tests {
		rand.Seed(0)
		got := randomBuildTag(tt.namespace, tt.name)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("randomBuildTag(%q, %q) = %q, want %q", tt.namespace, tt.name, got, tt.want)
		}
	}
}

func TestRandomBuildTagNoDupes(t *testing.T) {
	rand.Seed(0)
	previous := make(map[string]struct{})
	for i := 0; i < 100; i++ {
		tag := randomBuildTag("test", "build-1")
		_, exists := previous[tag]
		if exists {
			t.Errorf("randomBuildTag returned a recently seen tag: %q", tag)
		}
		previous[tag] = struct{}{}
	}
}

func TestContainerName(t *testing.T) {
	rand.Seed(0)
	got := containerName("test-strategy", "my-build", "ns", "hook")
	want := "openshift_test-strategy-build_my-build_ns_hook_f1f85ff5"
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
