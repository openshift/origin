package openshift

import (
	"testing"
)

func TestCommandFor(t *testing.T) {
	cmd := CommandFor("unknown")
	if cmd.Use != "openshift" {
		t.Errorf("expected command to be openshift: %#v", cmd)
	}
}
