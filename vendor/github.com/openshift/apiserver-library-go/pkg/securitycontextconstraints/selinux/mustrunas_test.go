package selinux

import (
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	coreapi "k8s.io/kubernetes/pkg/apis/core"

	securityv1 "github.com/openshift/api/security/v1"
)

func TestMustRunAsOptions(t *testing.T) {
	tests := map[string]struct {
		opts *securityv1.SELinuxContextStrategyOptions
		pass bool
	}{
		"invalid opts": {
			opts: &securityv1.SELinuxContextStrategyOptions{},
			pass: false,
		},
		"valid opts": {
			opts: &securityv1.SELinuxContextStrategyOptions{SELinuxOptions: &corev1.SELinuxOptions{}},
			pass: true,
		},
	}
	for name, tc := range tests {
		_, err := NewMustRunAs(tc.opts)
		if err != nil && tc.pass {
			t.Errorf("%s expected to pass but received error %#v", name, err)
		}
		if err == nil && !tc.pass {
			t.Errorf("%s expected to fail but did not receive an error", name)
		}
	}
}

func TestMustRunAsGenerate(t *testing.T) {
	opts := &securityv1.SELinuxContextStrategyOptions{
		SELinuxOptions: &corev1.SELinuxOptions{
			User:  "user",
			Role:  "role",
			Type:  "type",
			Level: "level",
		},
	}
	mustRunAs, err := NewMustRunAs(opts)
	if err != nil {
		t.Fatalf("unexpected error initializing NewMustRunAs %v", err)
	}
	generated, err := mustRunAs.Generate(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error generating selinux %v", err)
	}
	if !reflect.DeepEqual(generated, ToInternalSELinuxOptionsOrDie(opts.SELinuxOptions)) {
		t.Errorf("generated selinux does not equal configured selinux")
	}
}

func TestMustRunAsValidate(t *testing.T) {
	newValidOpts := func() *corev1.SELinuxOptions {
		return &corev1.SELinuxOptions{
			User:  "user",
			Role:  "role",
			Level: "s0:c0,c6",
			Type:  "type",
		}
	}

	newValidOptsWithLevel := func(level string) *corev1.SELinuxOptions {
		opts := newValidOpts()
		opts.Level = level
		return opts
	}

	role := newValidOpts()
	role.Role = "invalid"

	user := newValidOpts()
	user.User = "invalid"

	seType := newValidOpts()
	seType.Type = "invalid"

	validOpts := newValidOpts()

	tests := map[string]struct {
		podSeLinux  *coreapi.SELinuxOptions
		sccSeLinux  *corev1.SELinuxOptions
		expectedMsg string
	}{
		"invalid role": {
			podSeLinux:  ToInternalSELinuxOptionsOrDie(role),
			sccSeLinux:  validOpts,
			expectedMsg: "role: Invalid value",
		},
		"invalid user": {
			podSeLinux:  ToInternalSELinuxOptionsOrDie(user),
			sccSeLinux:  validOpts,
			expectedMsg: "user: Invalid value",
		},
		"levels are not equal": {
			podSeLinux:  ToInternalSELinuxOptionsOrDie(newValidOptsWithLevel("s0")),
			sccSeLinux:  newValidOptsWithLevel("s0:c1,c2"),
			expectedMsg: "level: Invalid value",
		},
		"levels differ by sensitivity": {
			podSeLinux:  ToInternalSELinuxOptionsOrDie(newValidOptsWithLevel("s0:c6")),
			sccSeLinux:  newValidOptsWithLevel("s1:c6"),
			expectedMsg: "level: Invalid value",
		},
		"levels differ by categories": {
			podSeLinux:  ToInternalSELinuxOptionsOrDie(newValidOptsWithLevel("s0:c0,c8")),
			sccSeLinux:  newValidOptsWithLevel("s0:c1,c7"),
			expectedMsg: "level: Invalid value",
		},
		"valid": {
			podSeLinux:  ToInternalSELinuxOptionsOrDie(validOpts),
			sccSeLinux:  validOpts,
			expectedMsg: "",
		},
		"valid with different order of categories": {
			podSeLinux:  ToInternalSELinuxOptionsOrDie(newValidOptsWithLevel("s0:c6,c0")),
			sccSeLinux:  validOpts,
			expectedMsg: "",
		},
	}

	for name, tc := range tests {
		opts := &securityv1.SELinuxContextStrategyOptions{
			SELinuxOptions: tc.sccSeLinux,
		}
		mustRunAs, err := NewMustRunAs(opts)
		if err != nil {
			t.Errorf("unexpected error initializing NewMustRunAs for testcase %s: %#v", name, err)
			continue
		}

		errs := mustRunAs.Validate(nil, nil, nil, tc.podSeLinux)
		//should've passed but didn't
		if len(tc.expectedMsg) == 0 && len(errs) > 0 {
			t.Errorf("%s expected no errors but received %v", name, errs)
		}
		//should've failed but didn't
		if len(tc.expectedMsg) != 0 && len(errs) == 0 {
			t.Errorf("%s expected error %s but received no errors", name, tc.expectedMsg)
		}
		//failed with additional messages
		if len(tc.expectedMsg) != 0 && len(errs) > 1 {
			t.Errorf("%s expected error %s but received multiple errors: %v", name, tc.expectedMsg, errs)
		}
		//check that we got the right message
		if len(tc.expectedMsg) != 0 && len(errs) == 1 {
			if !strings.Contains(errs[0].Error(), tc.expectedMsg) {
				t.Errorf("%s expected error to contain %s but it did not: %v", name, tc.expectedMsg, errs)
			}
		}
	}
}
