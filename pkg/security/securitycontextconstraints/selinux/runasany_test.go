package selinux

import (
	"testing"

	api "k8s.io/kubernetes/pkg/apis/core"

	securityapi "github.com/openshift/origin/pkg/security/apis/security"
)

func TestRunAsAnyOptions(t *testing.T) {
	_, err := NewRunAsAny(nil)
	if err != nil {
		t.Fatalf("unexpected error initializing NewRunAsAny %v", err)
	}
	_, err = NewRunAsAny(&securityapi.SELinuxContextStrategyOptions{})
	if err != nil {
		t.Errorf("unexpected error initializing NewRunAsAny %v", err)
	}
}

func TestRunAsAnyGenerate(t *testing.T) {
	s, err := NewRunAsAny(&securityapi.SELinuxContextStrategyOptions{})
	if err != nil {
		t.Fatalf("unexpected error initializing NewRunAsAny %v", err)
	}
	uid, err := s.Generate(nil, nil)
	if uid != nil {
		t.Errorf("expected nil uid but got %v", *uid)
	}
	if err != nil {
		t.Errorf("unexpected error generating uid %v", err)
	}
}

func TestRunAsAnyValidate(t *testing.T) {
	s, err := NewRunAsAny(&securityapi.SELinuxContextStrategyOptions{
		SELinuxOptions: &api.SELinuxOptions{
			Level: "foo",
		},
	},
	)
	if err != nil {
		t.Fatalf("unexpected error initializing NewRunAsAny %v", err)
	}
	errs := s.Validate(nil, nil, nil, nil)
	if len(errs) != 0 {
		t.Errorf("unexpected errors validating with ")
	}
	s, err = NewRunAsAny(&securityapi.SELinuxContextStrategyOptions{})
	if err != nil {
		t.Fatalf("unexpected error initializing NewRunAsAny %v", err)
	}
	errs = s.Validate(nil, nil, nil, nil)
	if len(errs) != 0 {
		t.Errorf("unexpected errors validating %v", errs)
	}
}
