package user

import (
	"testing"

	"k8s.io/kubernetes/pkg/api"

	securityapi "github.com/openshift/origin/pkg/security/apis/security"
)

func TestNonRootOptions(t *testing.T) {
	_, err := NewRunAsNonRoot(nil)
	if err != nil {
		t.Fatalf("unexpected error initializing NewRunAsNonRoot %v", err)
	}
	_, err = NewRunAsNonRoot(&securityapi.RunAsUserStrategyOptions{})
	if err != nil {
		t.Errorf("unexpected error initializing NewRunAsNonRoot %v", err)
	}
}

func TestNonRootGenerate(t *testing.T) {
	s, err := NewRunAsNonRoot(&securityapi.RunAsUserStrategyOptions{})
	if err != nil {
		t.Fatalf("unexpected error initializing NewRunAsNonRoot %v", err)
	}
	uid, err := s.Generate(nil, nil)
	if uid != nil {
		t.Errorf("expected nil uid but got %d", *uid)
	}
	if err != nil {
		t.Errorf("unexpected error generating uid %v", err)
	}
}

func TestNonRootValidate(t *testing.T) {
	var uid int64 = 1
	var badUID int64 = 0
	s, err := NewRunAsNonRoot(&securityapi.RunAsUserStrategyOptions{})
	if err != nil {
		t.Fatalf("unexpected error initializing NewMustRunAs %v", err)
	}
	container := &api.Container{
		SecurityContext: &api.SecurityContext{
			RunAsUser: &badUID,
		},
	}

	errs := s.Validate(nil, container)
	if len(errs) == 0 {
		t.Errorf("expected errors from root uid but got none")
	}

	container.SecurityContext.RunAsUser = &uid
	errs = s.Validate(nil, container)
	if len(errs) != 0 {
		t.Errorf("expected no errors from non-root uid but got %v", errs)
	}

	container.SecurityContext.RunAsUser = nil
	errs = s.Validate(nil, container)
	if len(errs) != 0 {
		t.Errorf("expected no errors from nil uid but got %v", errs)
	}
}
