package user

import (
	"fmt"
	"strings"
	"testing"

	securityapi "github.com/openshift/origin/pkg/security/apis/security"
)

func TestMustRunAsRangeOptions(t *testing.T) {
	var uid int64 = 1
	tests := map[string]struct {
		opts *securityapi.RunAsUserStrategyOptions
		pass bool
	}{
		"invalid opts, required min and max": {
			opts: &securityapi.RunAsUserStrategyOptions{},
			pass: false,
		},
		"invalid opts, required max": {
			opts: &securityapi.RunAsUserStrategyOptions{UIDRangeMin: &uid},
			pass: false,
		},
		"invalid opts, required min": {
			opts: &securityapi.RunAsUserStrategyOptions{UIDRangeMax: &uid},
			pass: false,
		},
		"valid opts": {
			opts: &securityapi.RunAsUserStrategyOptions{UIDRangeMin: &uid, UIDRangeMax: &uid},
			pass: true,
		},
	}
	for name, tc := range tests {
		_, err := NewMustRunAsRange(tc.opts)
		if err != nil && tc.pass {
			t.Errorf("%s expected to pass but received error %v", name, err)
		}
		if err == nil && !tc.pass {
			t.Errorf("%s expected to fail but did not receive an error", name)
		}
	}
}

func TestMustRunAsRangeGenerate(t *testing.T) {
	var uidMin int64 = 1
	var uidMax int64 = 10
	opts := &securityapi.RunAsUserStrategyOptions{UIDRangeMin: &uidMin, UIDRangeMax: &uidMax}
	mustRunAsRange, err := NewMustRunAsRange(opts)
	if err != nil {
		t.Fatalf("unexpected error initializing NewMustRunAsRange %v", err)
	}
	generated, err := mustRunAsRange.Generate(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error generating uid %v", err)
	}
	if *generated != uidMin {
		t.Errorf("generated uid does not equal expected uid")
	}
}

func TestMustRunAsRangeValidate(t *testing.T) {
	var uidMin int64 = 1
	var uidMax int64 = 10
	opts := &securityapi.RunAsUserStrategyOptions{UIDRangeMin: &uidMin, UIDRangeMax: &uidMax}
	mustRunAsRange, err := NewMustRunAsRange(opts)
	if err != nil {
		t.Fatalf("unexpected error initializing NewMustRunAsRange %v", err)
	}

	errs := mustRunAsRange.Validate(nil, nil, nil, nil, nil)
	expectedMessage := "runAsUser: Required value"
	if len(errs) == 0 {
		t.Errorf("expected errors from nil runAsUser but got none")
	} else if !strings.Contains(errs[0].Error(), expectedMessage) {
		t.Errorf("expected error to contain %q but it did not: %v", expectedMessage, errs)
	}

	var lowUid int64 = 0
	errs = mustRunAsRange.Validate(nil, nil, nil, nil, &lowUid)
	expectedMessage = fmt.Sprintf("runAsUser: Invalid value: %d: must be in the ranges: [%d, %d]", lowUid, uidMin, uidMax)
	if len(errs) == 0 {
		t.Errorf("expected errors from mismatch uid but got none")
	} else if !strings.Contains(errs[0].Error(), expectedMessage) {
		t.Errorf("expected error to contain %q but it did not: %v", expectedMessage, errs)
	}

	var highUid int64 = 11
	errs = mustRunAsRange.Validate(nil, nil, nil, nil, &highUid)
	expectedMessage = fmt.Sprintf("runAsUser: Invalid value: %d: must be in the ranges: [%d, %d]", highUid, uidMin, uidMax)
	if len(errs) == 0 {
		t.Errorf("expected errors from mismatch uid but got none")
	} else if !strings.Contains(errs[0].Error(), expectedMessage) {
		t.Errorf("expected error to contain %q but it did not: %v", expectedMessage, errs)
	}

	var goodUid int64 = 5
	errs = mustRunAsRange.Validate(nil, nil, nil, nil, &goodUid)
	if len(errs) != 0 {
		t.Errorf("expected no errors from matching uid but got %v", errs)
	}
}
