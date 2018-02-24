package storage

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveNames(t *testing.T) {
	cases := []struct {
		name                 string
		additionalRegistries []string
		imageName            string
		expected             []string
		err                  bool
		errContains          string
	}{
		{
			name:                 "test unqualified images get correctly qualified in order and correct tag",
			additionalRegistries: []string{"testregistry.com", "registry.access.redhat.com", "docker.io"},
			imageName:            "openshift3/ose-deployer:sometag",
			expected:             []string{"testregistry.com/openshift3/ose-deployer:sometag", "registry.access.redhat.com/openshift3/ose-deployer:sometag", "docker.io/openshift3/ose-deployer:sometag"},
			err:                  false,
		},
		{
			name:                 "test unqualified images get correctly qualified in order and correct digest",
			additionalRegistries: []string{"testregistry.com", "registry.access.redhat.com", "docker.io"},
			imageName:            "openshift3/ose-deployer@sha256:dc5f67a48da730d67bf4bfb8824ea8a51be26711de090d6d5a1ffff2723168a3",
			expected:             []string{"testregistry.com/openshift3/ose-deployer@sha256:dc5f67a48da730d67bf4bfb8824ea8a51be26711de090d6d5a1ffff2723168a3", "registry.access.redhat.com/openshift3/ose-deployer@sha256:dc5f67a48da730d67bf4bfb8824ea8a51be26711de090d6d5a1ffff2723168a3", "docker.io/openshift3/ose-deployer@sha256:dc5f67a48da730d67bf4bfb8824ea8a51be26711de090d6d5a1ffff2723168a3"},
			err:                  false,
		},
		{
			name:                 "test unqualified images get correctly qualified in order",
			additionalRegistries: []string{"testregistry.com", "registry.access.redhat.com", "docker.io"},
			imageName:            "openshift3/ose-deployer:latest",
			expected:             []string{"testregistry.com/openshift3/ose-deployer:latest", "registry.access.redhat.com/openshift3/ose-deployer:latest", "docker.io/openshift3/ose-deployer:latest"},
			err:                  false,
		},
		{
			name:                 "test unqualified images get correctly qualified from official library",
			additionalRegistries: []string{"testregistry.com", "registry.access.redhat.com", "docker.io"},
			imageName:            "nginx:latest",
			expected:             []string{"testregistry.com/nginx:latest", "registry.access.redhat.com/nginx:latest", "docker.io/library/nginx:latest"},
			err:                  false,
		},
		{
			name:                 "test qualified images returns just qualified",
			additionalRegistries: []string{"testregistry.com", "registry.access.redhat.com", "docker.io"},
			imageName:            "mypersonalregistry.com/nginx:latest",
			expected:             []string{"mypersonalregistry.com/nginx:latest"},
			err:                  false,
		},
		{
			name:      "test we don't have names w/o registries",
			imageName: "openshift3/ose-deployer:latest",
			err:       true,
		},
		{
			name:        "test we cannot resolve names from an image ID",
			imageName:   "6ad733544a6317992a6fac4eb19fe1df577d4dec7529efec28a5bd0edad0fd30",
			err:         true,
			errContains: "cannot parse an image ID",
		},
	}
	for _, c := range cases {
		svc := &imageService{
			registries: c.additionalRegistries,
		}
		names, err := svc.ResolveNames(c.imageName)
		if !c.err {
			require.NoError(t, err, c.name)
			if !reflect.DeepEqual(names, c.expected) {
				t.Fatalf("Exepected: %v, Got: %v: %q", c.expected, names, c.name)
			}
		} else {
			require.Error(t, err, c.name)
			if c.errContains != "" {
				assert.Contains(t, err.Error(), c.errContains)
			}
		}
	}
}
