package builder

import (
	"bytes"
	"reflect"
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
						DockerImageReference: "test/tag",
					},
				},
			},
			expected: "test/tag",
		},
		{
			build: api.Build{
				Parameters: api.BuildParameters{
					Output: api.BuildOutput{
						DockerImageReference: "registry-server.test:5000/test/tag",
					},
				},
			},
			expected: "registry-server.test:5000/test/tag",
		},
	}
	for _, x := range tests {
		result := x.build.Parameters.Output.DockerImageReference
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

func TestReadDNSConfig(t *testing.T) {
	const sampleFile = `# /etc/resolv.conf

domain localdomain
nameserver 1.2.3.4
nameserver 5.6.7.8
search example.com test.local
options ndots:5 timeout:10 attempts:3 rotate
options attempts 3o
`
	dns, dnsSearch := readDNSConfig(bytes.NewBufferString(sampleFile))
	if !reflect.DeepEqual(dns, []string{"1.2.3.4", "5.6.7.8"}) {
		t.Errorf("Unexpected value for dns: %#v\n", dns)
	}
	if !reflect.DeepEqual(dnsSearch, []string{"example.com", "test.local"}) {
		t.Errorf("Unexpected value for dnsSearch: %#v\n", dnsSearch)
	}
}
