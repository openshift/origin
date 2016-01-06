package requestlimit

import (
	"testing"

	"k8s.io/kubernetes/pkg/util/validation/field"
)

func TestValidateProjectRequestLimitConfig(t *testing.T) {
	tests := []struct {
		config      ProjectRequestLimitConfig
		errExpected bool
		errType     field.ErrorType
		errField    string
	}{
		// 0: empty config
		{
			config: ProjectRequestLimitConfig{},
		},
		// 1: single default
		{
			config: ProjectRequestLimitConfig{
				Limits: []ProjectLimitBySelector{
					{
						Selector:    nil,
						MaxProjects: intp(1),
					},
				},
			},
		},
		// 2: multiple limits
		{
			config: ProjectRequestLimitConfig{
				Limits: []ProjectLimitBySelector{
					{
						Selector:    map[string]string{"foo": "bar", "foo2": "baz"},
						MaxProjects: intp(10),
					},
					{
						Selector:    map[string]string{"foo": "foo"},
						MaxProjects: intp(1),
					},
				},
			},
		},
		// 3: negative limit (error)
		{
			config: ProjectRequestLimitConfig{
				Limits: []ProjectLimitBySelector{
					{
						Selector:    map[string]string{"foo": "bar", "foo2": "baz"},
						MaxProjects: intp(10),
					},
					{
						Selector:    map[string]string{"foo": "foo"},
						MaxProjects: intp(-1),
					},
				},
			},
			errExpected: true,
			errType:     field.ErrorTypeInvalid,
			errField:    "limits[1].maxProjects",
		},
		// 4: invalid selector label (error)
		{
			config: ProjectRequestLimitConfig{
				Limits: []ProjectLimitBySelector{
					{
						Selector:    map[string]string{"foo": "bar", "foo2": "baz"},
						MaxProjects: intp(10),
					},
					{
						Selector:    nil,
						MaxProjects: intp(5),
					},
					{
						Selector:    map[string]string{"foo": "foo", "*invalid/label": "test"},
						MaxProjects: intp(1),
					},
				},
			},
			errExpected: true,
			errType:     field.ErrorTypeInvalid,
			errField:    "limits[2].selector",
		},
	}

	for i, tc := range tests {
		errs := ValidateProjectRequestLimitConfig(&tc.config)
		if len(errs) > 0 && !tc.errExpected {
			t.Errorf("%d: unexpected error: %v", i, errs.ToAggregate())
			continue
		}
		if len(errs) == 0 && tc.errExpected {
			t.Errorf("%d: did not get expected error", i)
			continue
		}
		if !tc.errExpected {
			continue
		}
		verr := errs[0]
		if verr.Type != tc.errType {
			t.Errorf("%d: did not get expected error type. Expected: %s. Got: %s", i, tc.errType, verr.Type)
		}
		if verr.Field != tc.errField {
			t.Errorf("%d: did not get expected error field. Expected: %s. Got: %s", i, tc.errField, verr.Field)
		}
	}
}
