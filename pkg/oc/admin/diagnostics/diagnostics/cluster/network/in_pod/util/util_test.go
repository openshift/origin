package util

import (
	"testing"
)

func TestTrimRegistryPath(t *testing.T) {
	testcases := map[string]struct {
		image         string
		expectedImage string
	}{
		"Empty image": {
			image:         "",
			expectedImage: "",
		},
		"Image with no slashes, no tags": {
			image:         "origin",
			expectedImage: "origin",
		},
		"Image with no slashes": {
			image:         "origin:v1.2.3",
			expectedImage: "origin:v1.2.3",
		},
		"Image with one slash, no tags": {
			image:         "openshift/origin",
			expectedImage: "openshift/origin",
		},
		"Image with one slash": {
			image:         "openshift/origin:v1.2.3",
			expectedImage: "openshift/origin:v1.2.3",
		},
		"Image with dns path, no port, no tags": {
			image:         "registry.access.redhat.com/openshift3/ose",
			expectedImage: "openshift3/ose",
		},
		"Image with dns path, no port": {
			image:         "registry.access.redhat.com/openshift3/ose:v1.2.3",
			expectedImage: "openshift3/ose:v1.2.3",
		},
		"Image with dns path": {
			image:         "registry.reg-aws.openshift.com:443/openshift3/ose:v1.2.3",
			expectedImage: "openshift3/ose:v1.2.3",
		},
	}

	for name, tc := range testcases {
		trimmedImage := trimRegistryPath(tc.image)
		if trimmedImage != tc.expectedImage {
			t.Fatalf("[%s] failed: expected %s but got %s", name, tc.expectedImage, trimmedImage)
		}
	}
}
