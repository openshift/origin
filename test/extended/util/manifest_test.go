package util

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubectl/pkg/scheme"

	appsv1 "github.com/openshift/api/apps/v1"
)

func init() {
	appsv1.Install(scheme.Scheme)
}

func TestReadFixture(t *testing.T) {
	tt := []struct {
		name     string
		path     string
		expected runtime.Object
	}{
		{
			name:     "dc V1",
			path:     FixturePath("testdata", "deployments", "deployment-simple.yaml"),
			expected: &appsv1.DeploymentConfig{},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			obj, err := ReadFixture(tc.path)
			if err != nil {
				t.Error(err)
			}

			expected := reflect.TypeOf(tc.expected)
			got := reflect.TypeOf(obj)
			if expected != got {
				t.Errorf("expected %v, got %v", expected, got)
			}
		})
	}
}
