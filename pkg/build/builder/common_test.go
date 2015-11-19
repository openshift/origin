package builder

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/build/api"
)

func TestBuildInfo(t *testing.T) {
	b := &api.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "sample-app",
			Namespace: "default",
		},
		Spec: api.BuildSpec{
			Source: api.BuildSource{
				Git: &api.GitBuildSource{
					URI: "github.com/openshift/sample-app",
					Ref: "master",
				},
			},
			Strategy: api.BuildStrategy{
				Type: api.SourceBuildStrategyType,
				SourceStrategy: &api.SourceBuildStrategy{
					Env: []kapi.EnvVar{
						{Name: "RAILS_ENV", Value: "production"},
					},
				},
			},
			Revision: &api.SourceRevision{
				Git: &api.GitSourceRevision{
					Commit: "1575a90c569a7cc0eea84fbd3304d9df37c9f5ee",
				},
			},
		},
	}
	got := buildInfo(b)
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
}
