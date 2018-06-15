package user

import (
	"strings"
	"testing"

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
		t.Fatalf("unexpected error initializing NewRunAsNonRoot %v", err)
	}

	errs := s.Validate(nil, nil, nil, nil, &badUID)
	expectedMessage := "runAsUser: Invalid value: 0: running with the root UID is forbidden"
	if len(errs) == 0 {
		t.Errorf("expected errors from root uid but got none")
	} else if !strings.Contains(errs[0].Error(), expectedMessage) {
		t.Errorf("expected error to contain %q but it did not: %v", expectedMessage, errs)
	}

	errs = s.Validate(nil, nil, nil, nil, nil)
	expectedMessage = "runAsNonRoot: Required value: must be true"
	if len(errs) == 0 {
		t.Errorf("expected error when neither runAsUser nor runAsNonRoot are specified but got none")
	} else if !strings.Contains(errs[0].Error(), expectedMessage) {
		t.Errorf("expected error to contain %q but it did not: %v", expectedMessage, errs)
	}

	no := false
	errs = s.Validate(nil, nil, nil, &no, nil)
	expectedMessage = "runAsNonRoot: Invalid value: false: must be true"
	if len(errs) == 0 {
		t.Errorf("expected error when runAsNonRoot is false but got none")
	} else if !strings.Contains(errs[0].Error(), expectedMessage) {
		t.Errorf("expected error to contain %q but it did not: %v", expectedMessage, errs)
	}

	errs = s.Validate(nil, nil, nil, nil, &uid)
	if len(errs) != 0 {
		t.Errorf("expected no errors from non-root uid but got %v", errs)
	}

	yes := true
	errs = s.Validate(nil, nil, nil, &yes, nil)
	if len(errs) != 0 {
		t.Errorf("expected no errors from nil uid but got %v", errs)
	}
}
