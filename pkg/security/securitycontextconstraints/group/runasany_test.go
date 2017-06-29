package group

import (
	"testing"
)

func TestRunAsAnyGenerate(t *testing.T) {
	s, err := NewRunAsAny()
	if err != nil {
		t.Fatalf("unexpected error initializing NewRunAsAny %v", err)
	}
	groups, err := s.Generate(nil)
	if len(groups) > 0 {
		t.Errorf("expected empty but got %v", groups)
	}
	if err != nil {
		t.Errorf("unexpected error generating groups: %v", err)
	}
}

func TestRunAsAnyGenerateSingle(t *testing.T) {
	s, err := NewRunAsAny()
	if err != nil {
		t.Fatalf("unexpected error initializing NewRunAsAny %v", err)
	}
	group, err := s.GenerateSingle(nil)
	if group != nil {
		t.Errorf("expected empty but got %v", group)
	}
	if err != nil {
		t.Errorf("unexpected error generating groups: %v", err)
	}
}

func TestRunAsAnyValidte(t *testing.T) {
	s, err := NewRunAsAny()
	if err != nil {
		t.Fatalf("unexpected error initializing NewRunAsAny %v", err)
	}
	errs := s.Validate(nil, nil)
	if len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}
