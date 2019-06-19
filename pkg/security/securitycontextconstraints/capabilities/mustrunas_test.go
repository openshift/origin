package capabilities

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	api "k8s.io/kubernetes/pkg/apis/core"

	securityv1 "github.com/openshift/api/security/v1"
)

func TestGenerateAdds(t *testing.T) {
	tests := map[string]struct {
		defaultAddCaps   []corev1.Capability
		requiredDropCaps []corev1.Capability
		containerCaps    *api.Capabilities
		expectedCaps     *api.Capabilities
	}{
		"no required, no container requests": {
			expectedCaps: nil,
		},
		"no required, no container requests, non-nil": {
			containerCaps: &api.Capabilities{},
			expectedCaps:  &api.Capabilities{},
		},
		"required, no container requests": {
			defaultAddCaps: []corev1.Capability{"foo"},
			expectedCaps: &api.Capabilities{
				Add: []api.Capability{"foo"},
			},
		},
		"required, container requests add required": {
			defaultAddCaps: []corev1.Capability{"foo"},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"foo"},
			},
			expectedCaps: &api.Capabilities{
				Add: []api.Capability{"foo"},
			},
		},
		"multiple required, container requests add required": {
			defaultAddCaps: []corev1.Capability{"foo", "bar", "baz"},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"foo"},
			},
			expectedCaps: &api.Capabilities{
				Add: []api.Capability{"bar", "baz", "foo"},
			},
		},
		"required, container requests add non-required": {
			defaultAddCaps: []corev1.Capability{"foo"},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"bar"},
			},
			expectedCaps: &api.Capabilities{
				Add: []api.Capability{"bar", "foo"},
			},
		},
		"generation does not mutate unnecessarily": {
			defaultAddCaps: []corev1.Capability{"foo", "bar"},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"foo", "foo", "bar", "baz"},
			},
			expectedCaps: &api.Capabilities{
				Add: []api.Capability{"foo", "foo", "bar", "baz"},
			},
		},
		"generation dedupes": {
			defaultAddCaps: []corev1.Capability{"foo", "bar"},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"foo", "baz"},
			},
			expectedCaps: &api.Capabilities{
				Add: []api.Capability{"bar", "baz", "foo"},
			},
		},
		"generation is case sensitive - will not dedupe": {
			defaultAddCaps: []corev1.Capability{"foo"},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"FOO"},
			},
			expectedCaps: &api.Capabilities{
				Add: []api.Capability{"FOO", "foo"},
			},
		},
	}

	for k, v := range tests {
		container := &api.Container{
			SecurityContext: &api.SecurityContext{
				Capabilities: v.containerCaps,
			},
		}

		strategy, err := NewDefaultCapabilities(v.defaultAddCaps, v.requiredDropCaps, nil)
		if err != nil {
			t.Errorf("%s failed: %v", k, err)
			continue
		}
		generatedCaps, err := strategy.Generate(nil, container)
		if err != nil {
			t.Errorf("%s failed generating: %v", k, err)
			continue
		}
		if v.expectedCaps == nil && generatedCaps != nil {
			t.Errorf("%s expected nil caps to be generated but got %v", k, generatedCaps)
			continue
		}
		if !reflect.DeepEqual(v.expectedCaps, generatedCaps) {
			t.Errorf("%s did not generate correctly.  Expected: %#v, Actual: %#v", k, v.expectedCaps, generatedCaps)
		}
	}
}

func TestGenerateDrops(t *testing.T) {
	tests := map[string]struct {
		defaultAddCaps   []corev1.Capability
		requiredDropCaps []corev1.Capability
		containerCaps    *api.Capabilities
		expectedCaps     *api.Capabilities
	}{
		"no required, no container requests": {
			expectedCaps: nil,
		},
		"no required, no container requests, non-nil": {
			containerCaps: &api.Capabilities{},
			expectedCaps:  &api.Capabilities{},
		},
		"required drops are defaulted": {
			requiredDropCaps: []corev1.Capability{"foo"},
			expectedCaps: &api.Capabilities{
				Drop: []api.Capability{"foo"},
			},
		},
		"required drops are defaulted when making container requests": {
			requiredDropCaps: []corev1.Capability{"baz"},
			containerCaps: &api.Capabilities{
				Drop: []api.Capability{"foo", "bar"},
			},
			expectedCaps: &api.Capabilities{
				Drop: []api.Capability{"bar", "baz", "foo"},
			},
		},
		"required drops do not mutate unnecessarily": {
			requiredDropCaps: []corev1.Capability{"baz"},
			containerCaps: &api.Capabilities{
				Drop: []api.Capability{"foo", "bar", "baz"},
			},
			expectedCaps: &api.Capabilities{
				Drop: []api.Capability{"foo", "bar", "baz"},
			},
		},
		"can drop a required add": {
			defaultAddCaps: []corev1.Capability{"foo"},
			containerCaps: &api.Capabilities{
				Drop: []api.Capability{"foo"},
			},
			expectedCaps: &api.Capabilities{
				Drop: []api.Capability{"foo"},
			},
		},
		"can drop non-required add": {
			defaultAddCaps: []corev1.Capability{"foo"},
			containerCaps: &api.Capabilities{
				Drop: []api.Capability{"bar"},
			},
			expectedCaps: &api.Capabilities{
				Add:  []api.Capability{"foo"},
				Drop: []api.Capability{"bar"},
			},
		},
		"defaulting adds and drops, dropping a required add": {
			defaultAddCaps:   []corev1.Capability{"foo", "bar", "baz"},
			requiredDropCaps: []corev1.Capability{"abc"},
			containerCaps: &api.Capabilities{
				Drop: []api.Capability{"foo"},
			},
			expectedCaps: &api.Capabilities{
				Add:  []api.Capability{"bar", "baz"},
				Drop: []api.Capability{"abc", "foo"},
			},
		},
		"generation dedupes": {
			requiredDropCaps: []corev1.Capability{"baz", "foo"},
			containerCaps: &api.Capabilities{
				Drop: []api.Capability{"bar", "foo"},
			},
			expectedCaps: &api.Capabilities{
				Drop: []api.Capability{"bar", "baz", "foo"},
			},
		},
		"generation is case sensitive - will not dedupe": {
			requiredDropCaps: []corev1.Capability{"bar"},
			containerCaps: &api.Capabilities{
				Drop: []api.Capability{"BAR"},
			},
			expectedCaps: &api.Capabilities{
				Drop: []api.Capability{"BAR", "bar"},
			},
		},
	}
	for k, v := range tests {
		container := &api.Container{
			SecurityContext: &api.SecurityContext{
				Capabilities: v.containerCaps,
			},
		}

		strategy, err := NewDefaultCapabilities(v.defaultAddCaps, v.requiredDropCaps, nil)
		if err != nil {
			t.Errorf("%s failed: %v", k, err)
			continue
		}
		generatedCaps, err := strategy.Generate(nil, container)
		if err != nil {
			t.Errorf("%s failed generating: %v", k, err)
			continue
		}
		if v.expectedCaps == nil && generatedCaps != nil {
			t.Errorf("%s expected nil caps to be generated but got %#v", k, generatedCaps)
			continue
		}
		if !reflect.DeepEqual(v.expectedCaps, generatedCaps) {
			t.Errorf("%s did not generate correctly.  Expected: %#v, Actual: %#v", k, v.expectedCaps, generatedCaps)
		}
	}
}

func TestValidateAdds(t *testing.T) {
	tests := map[string]struct {
		defaultAddCaps   []corev1.Capability
		requiredDropCaps []corev1.Capability
		allowedCaps      []corev1.Capability
		containerCaps    *api.Capabilities
		shouldPass       bool
	}{
		// no container requests
		"no required, no allowed, no container requests": {
			shouldPass: true,
		},
		"no required, allowed, no container requests": {
			allowedCaps: []corev1.Capability{"foo"},
			shouldPass:  true,
		},
		"required, no allowed, no container requests": {
			defaultAddCaps: []corev1.Capability{"foo"},
			shouldPass:     false,
		},

		// container requests match required
		"required, no allowed, container requests valid": {
			defaultAddCaps: []corev1.Capability{"foo"},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"foo"},
			},
			shouldPass: true,
		},
		"required, no allowed, container requests invalid": {
			defaultAddCaps: []corev1.Capability{"foo"},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"bar"},
			},
			shouldPass: false,
		},

		// container requests match allowed
		"no required, allowed, container requests valid": {
			allowedCaps: []corev1.Capability{"foo"},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"foo"},
			},
			shouldPass: true,
		},
		"no required, all allowed, container requests valid": {
			allowedCaps: []corev1.Capability{securityv1.AllowAllCapabilities},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"foo"},
			},
			shouldPass: true,
		},
		"no required, allowed, container requests invalid": {
			allowedCaps: []corev1.Capability{"foo"},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"bar"},
			},
			shouldPass: false,
		},

		// required and allowed
		"required, allowed, container requests valid required": {
			defaultAddCaps: []corev1.Capability{"foo"},
			allowedCaps:    []corev1.Capability{"bar"},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"foo"},
			},
			shouldPass: true,
		},
		"required, allowed, container requests valid allowed": {
			defaultAddCaps: []corev1.Capability{"foo"},
			allowedCaps:    []corev1.Capability{"bar"},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"bar"},
			},
			shouldPass: true,
		},
		"required, allowed, container requests invalid": {
			defaultAddCaps: []corev1.Capability{"foo"},
			allowedCaps:    []corev1.Capability{"bar"},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"baz"},
			},
			shouldPass: false,
		},
		"validation is case sensitive": {
			defaultAddCaps: []corev1.Capability{"foo"},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"FOO"},
			},
			shouldPass: false,
		},
	}

	for k, v := range tests {
		strategy, err := NewDefaultCapabilities(v.defaultAddCaps, v.requiredDropCaps, v.allowedCaps)
		if err != nil {
			t.Errorf("%s failed: %v", k, err)
			continue
		}
		errs := strategy.Validate(nil, nil, v.containerCaps)
		if v.shouldPass && len(errs) > 0 {
			t.Errorf("%s should have passed but had errors %v", k, errs)
			continue
		}
		if !v.shouldPass && len(errs) == 0 {
			t.Errorf("%s should have failed but recieved no errors", k)
		}
	}
}

func TestValidateDrops(t *testing.T) {
	tests := map[string]struct {
		defaultAddCaps   []corev1.Capability
		requiredDropCaps []corev1.Capability
		containerCaps    *api.Capabilities
		shouldPass       bool
	}{
		// no container requests
		"no required, no container requests": {
			shouldPass: true,
		},
		"required, no container requests": {
			requiredDropCaps: []corev1.Capability{"foo"},
			shouldPass:       false,
		},

		// container requests match required
		"required, container requests valid": {
			requiredDropCaps: []corev1.Capability{"foo"},
			containerCaps: &api.Capabilities{
				Drop: []api.Capability{"foo"},
			},
			shouldPass: true,
		},
		"required, container requests invalid": {
			requiredDropCaps: []corev1.Capability{"foo"},
			containerCaps: &api.Capabilities{
				Drop: []api.Capability{"bar"},
			},
			shouldPass: false,
		},
		"validation is case sensitive": {
			requiredDropCaps: []corev1.Capability{"foo"},
			containerCaps: &api.Capabilities{
				Drop: []api.Capability{"FOO"},
			},
			shouldPass: false,
		},
	}

	for k, v := range tests {
		strategy, err := NewDefaultCapabilities(v.defaultAddCaps, v.requiredDropCaps, nil)
		if err != nil {
			t.Errorf("%s failed: %v", k, err)
			continue
		}
		errs := strategy.Validate(nil, nil, v.containerCaps)
		if v.shouldPass && len(errs) > 0 {
			t.Errorf("%s should have passed but had errors %v", k, errs)
			continue
		}
		if !v.shouldPass && len(errs) == 0 {
			t.Errorf("%s should have failed but recieved no errors", k)
		}
	}
}
