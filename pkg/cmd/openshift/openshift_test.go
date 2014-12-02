package openshift

import (
	"strings"
	"testing"
)

func TestCommandFor(t *testing.T) {
	cmd := CommandFor("openshift-router")
	if !strings.HasPrefix(cmd.Use, "openshift-router ") {
		t.Errorf("expected command to start with prefix: %#v", cmd)
	}

	cmd = CommandFor("unknown")
	if cmd.Use != "openshift" {
		t.Errorf("expected command to be openshift: %#v", cmd)
	}
}
