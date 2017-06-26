package user

import (
	"testing"

	"k8s.io/kubernetes/pkg/api"

	securityapi "github.com/openshift/origin/pkg/security/apis/security"
)

func TestMustRunAsOptions(t *testing.T) {
	var uid int64 = 1
	tests := map[string]struct {
		opts *securityapi.RunAsUserStrategyOptions
		pass bool
	}{
		"invalid opts": {
			opts: &securityapi.RunAsUserStrategyOptions{},
			pass: false,
		},
		"valid opts": {
			opts: &securityapi.RunAsUserStrategyOptions{UID: &uid},
			pass: true,
		},
	}
	for name, tc := range tests {
		_, err := NewMustRunAs(tc.opts)
		if err != nil && tc.pass {
			t.Errorf("%s expected to pass but received error %v", name, err)
		}
		if err == nil && !tc.pass {
			t.Errorf("%s expected to fail but did not receive an error", name)
		}
	}
}

func TestMustRunAsGenerate(t *testing.T) {
	var uid int64 = 1
	opts := &securityapi.RunAsUserStrategyOptions{UID: &uid}
	mustRunAs, err := NewMustRunAs(opts)
	if err != nil {
		t.Fatalf("unexpected error initializing NewMustRunAs %v", err)
	}
	generated, err := mustRunAs.Generate(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error generating uid %v", err)
	}
	if *generated != uid {
		t.Errorf("generated uid does not equal configured uid")
	}
}

func TestMustRunAsValidate(t *testing.T) {
	var uid int64 = 1
	var badUID int64 = 2
	opts := &securityapi.RunAsUserStrategyOptions{UID: &uid}
	mustRunAs, err := NewMustRunAs(opts)
	if err != nil {
		t.Fatalf("unexpected error initializing NewMustRunAs %v", err)
	}
	container := &api.Container{
		SecurityContext: &api.SecurityContext{
			RunAsUser: &badUID,
		},
	}

	errs := mustRunAs.Validate(nil, container)
	if len(errs) == 0 {
		t.Errorf("expected errors from mismatch uid but got none")
	}

	container.SecurityContext.RunAsUser = &uid
	errs = mustRunAs.Validate(nil, container)
	if len(errs) != 0 {
		t.Errorf("expected no errors from matching uid but got %v", errs)
	}
}
