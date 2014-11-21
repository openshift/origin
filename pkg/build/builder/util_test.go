package builder

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/openshift/origin/pkg/build/api"
)

func TestImageTag(t *testing.T) {
	type tagTest struct {
		build    api.Build
		expected string
	}
	tests := []tagTest{
		{
			build: api.Build{
				Parameters: api.BuildParameters{
					Output: api.BuildOutput{
						ImageTag: "test/tag",
					},
				},
			},
			expected: "test/tag",
		},
		{
			build: api.Build{
				Parameters: api.BuildParameters{
					Output: api.BuildOutput{
						ImageTag: "test/tag",
						Registry: "registry-server.test:5000",
					},
				},
			},
			expected: "registry-server.test:5000/test/tag",
		},
		{
			build: api.Build{
				Parameters: api.BuildParameters{
					Output: api.BuildOutput{
						ImageTag: "registry-server.test:5000/test/tag",
						Registry: "registry-server.test:5000",
					},
				},
			},
			expected: "registry-server.test:5000/test/tag",
		},
	}
	for _, x := range tests {
		result := imageTag(&x.build)
		if result != x.expected {
			t.Errorf("Unexpected imageTag result. Expected: %s, Actual: %s",
				result, x.expected)
		}
	}
}

func TestGetBuildEnvVars(t *testing.T) {
	b := &api.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name: "1234",
		},
		Parameters: api.BuildParameters{
			Source: api.BuildSource{
				Git: &api.GitBuildSource{
					URI: "github.com/build/uri",
					Ref: "my-branch",
				},
			},
			Revision: &api.SourceRevision{
				Git: &api.GitSourceRevision{
					Commit: "56789",
				},
			},
		},
	}

	vars := getBuildEnvVars(b)
	expected := map[string]string{
		"OPENSHIFT_BUILD_NAME":      "1234",
		"OPENSHIFT_BUILD_SOURCE":    "github.com/build/uri",
		"OPENSHIFT_BUILD_REFERENCE": "my-branch",
		"OPENSHIFT_BUILD_COMMIT":    "56789",
	}
	for k, v := range expected {
		if vars[k] != v {
			t.Errorf("Expected: %s,%s, Got: %s,%s", k, v, k, vars[k])
		}
	}
}
