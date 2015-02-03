package server

import (
	"os"
	"testing"

	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/version"
)

func TestExpandDefaultImage(t *testing.T) {
	variable.OverrideVersion = version.Get()
	variable.OverrideVersion.GitVersion = "v1.0"

	os.Setenv("OPENSHIFT_COMPONENT_IMAGE", "test")

	tests := []struct {
		component string
		template  string
		latest    bool
		output    string
	}{
		{"*", "openshift/origin-${component}", true, "openshift/origin-*"},
		{"version", "openshift/origin-${component}", true, "openshift/origin-version"},
		{"version", "openshift/origin-${component}:${version}", true, "openshift/origin-version:latest"},
		{"version", "openshift/origin-${component}:${version}", false, "openshift/origin-version:v1.0"},
		{"component", "openshift/origin-${component}:${version}", true, "test"},
	}
	for _, test := range tests {
		if s := expandImage(test.component, test.template, test.latest); s != test.output {
			t.Errorf("unexpected image expansion for %#v: %s", test, s)
		}
	}
}
