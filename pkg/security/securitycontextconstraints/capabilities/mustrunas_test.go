package capabilities

import (
	"reflect"
	"testing"

	api "k8s.io/kubernetes/pkg/apis/core"

	securityapi "github.com/openshift/origin/pkg/security/apis/security"
)

func TestGenerateAdds(t *testing.T) {
	tests := map[string]struct {
		defaultAddCaps   []api.Capability
		requiredDropCaps []api.Capability
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
			defaultAddCaps: []api.Capability{"foo"},
			expectedCaps: &api.Capabilities{
				Add: []api.Capability{"foo"},
			},
		},
		"required, container requests add required": {
			defaultAddCaps: []api.Capability{"foo"},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"foo"},
			},
			expectedCaps: &api.Capabilities{
				Add: []api.Capability{"foo"},
			},
		},
		"multiple required, container requests add required": {
			defaultAddCaps: []api.Capability{"foo", "bar", "baz"},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"foo"},
			},
			expectedCaps: &api.Capabilities{
				Add: []api.Capability{"bar", "baz", "foo"},
			},
		},
		"required, container requests add non-required": {
			defaultAddCaps: []api.Capability{"foo"},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"bar"},
			},
			expectedCaps: &api.Capabilities{
				Add: []api.Capability{"bar", "foo"},
			},
		},
		"generation does not mutate unnecessarily": {
			defaultAddCaps: []api.Capability{"foo", "bar"},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"foo", "foo", "bar", "baz"},
			},
			expectedCaps: &api.Capabilities{
				Add: []api.Capability{"foo", "foo", "bar", "baz"},
			},
		},
		"generation dedupes": {
			defaultAddCaps: []api.Capability{"foo", "bar"},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"foo", "baz"},
			},
			expectedCaps: &api.Capabilities{
				Add: []api.Capability{"bar", "baz", "foo"},
			},
		},
		"generation is case sensitive - will not dedupe": {
			defaultAddCaps: []api.Capability{"foo"},
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
		defaultAddCaps   []api.Capability
		requiredDropCaps []api.Capability
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
			requiredDropCaps: []api.Capability{"foo"},
			expectedCaps: &api.Capabilities{
				Drop: []api.Capability{"foo"},
			},
		},
		"required drops are defaulted when making container requests": {
			requiredDropCaps: []api.Capability{"baz"},
			containerCaps: &api.Capabilities{
				Drop: []api.Capability{"foo", "bar"},
			},
			expectedCaps: &api.Capabilities{
				Drop: []api.Capability{"bar", "baz", "foo"},
			},
		},
		"required drops do not mutate unnecessarily": {
			requiredDropCaps: []api.Capability{"baz"},
			containerCaps: &api.Capabilities{
				Drop: []api.Capability{"foo", "bar", "baz"},
			},
			expectedCaps: &api.Capabilities{
				Drop: []api.Capability{"foo", "bar", "baz"},
			},
		},
		"can drop a required add": {
			defaultAddCaps: []api.Capability{"foo"},
			containerCaps: &api.Capabilities{
				Drop: []api.Capability{"foo"},
			},
			expectedCaps: &api.Capabilities{
				Drop: []api.Capability{"foo"},
			},
		},
		"can drop non-required add": {
			defaultAddCaps: []api.Capability{"foo"},
			containerCaps: &api.Capabilities{
				Drop: []api.Capability{"bar"},
			},
			expectedCaps: &api.Capabilities{
				Add:  []api.Capability{"foo"},
				Drop: []api.Capability{"bar"},
			},
		},
		"defaulting adds and drops, dropping a required add": {
			defaultAddCaps:   []api.Capability{"foo", "bar", "baz"},
			requiredDropCaps: []api.Capability{"abc"},
			containerCaps: &api.Capabilities{
				Drop: []api.Capability{"foo"},
			},
			expectedCaps: &api.Capabilities{
				Add:  []api.Capability{"bar", "baz"},
				Drop: []api.Capability{"abc", "foo"},
			},
		},
		"generation dedupes": {
			requiredDropCaps: []api.Capability{"baz", "foo"},
			containerCaps: &api.Capabilities{
				Drop: []api.Capability{"bar", "foo"},
			},
			expectedCaps: &api.Capabilities{
				Drop: []api.Capability{"bar", "baz", "foo"},
			},
		},
		"generation is case sensitive - will not dedupe": {
			requiredDropCaps: []api.Capability{"bar"},
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
		defaultAddCaps   []api.Capability
		requiredDropCaps []api.Capability
		allowedCaps      []api.Capability
		containerCaps    *api.Capabilities
		shouldPass       bool
	}{
		// no container requests
		"no required, no allowed, no container requests": {
			shouldPass: true,
		},
		"no required, allowed, no container requests": {
			allowedCaps: []api.Capability{"foo"},
			shouldPass:  true,
		},
		"required, no allowed, no container requests": {
			defaultAddCaps: []api.Capability{"foo"},
			shouldPass:     false,
		},

		// container requests match required
		"required, no allowed, container requests valid": {
			defaultAddCaps: []api.Capability{"foo"},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"foo"},
			},
			shouldPass: true,
		},
		"required, no allowed, container requests invalid": {
			defaultAddCaps: []api.Capability{"foo"},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"bar"},
			},
			shouldPass: false,
		},

		// container requests match allowed
		"no required, allowed, container requests valid": {
			allowedCaps: []api.Capability{"foo"},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"foo"},
			},
			shouldPass: true,
		},
		"no required, all allowed, container requests valid": {
			allowedCaps: []api.Capability{securityapi.AllowAllCapabilities},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"foo"},
			},
			shouldPass: true,
		},
		"no required, allowed, container requests invalid": {
			allowedCaps: []api.Capability{"foo"},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"bar"},
			},
			shouldPass: false,
		},

		// required and allowed
		"required, allowed, container requests valid required": {
			defaultAddCaps: []api.Capability{"foo"},
			allowedCaps:    []api.Capability{"bar"},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"foo"},
			},
			shouldPass: true,
		},
		"required, allowed, container requests valid allowed": {
			defaultAddCaps: []api.Capability{"foo"},
			allowedCaps:    []api.Capability{"bar"},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"bar"},
			},
			shouldPass: true,
		},
		"required, allowed, container requests invalid": {
			defaultAddCaps: []api.Capability{"foo"},
			allowedCaps:    []api.Capability{"bar"},
			containerCaps: &api.Capabilities{
				Add: []api.Capability{"baz"},
			},
			shouldPass: false,
		},
		"validation is case sensitive": {
			defaultAddCaps: []api.Capability{"foo"},
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
		defaultAddCaps   []api.Capability
		requiredDropCaps []api.Capability
		containerCaps    *api.Capabilities
		shouldPass       bool
	}{
		// no container requests
		"no required, no container requests": {
			shouldPass: true,
		},
		"required, no container requests": {
			requiredDropCaps: []api.Capability{"foo"},
			shouldPass:       false,
		},

		// container requests match required
		"required, container requests valid": {
			requiredDropCaps: []api.Capability{"foo"},
			containerCaps: &api.Capabilities{
				Drop: []api.Capability{"foo"},
			},
			shouldPass: true,
		},
		"required, container requests invalid": {
			requiredDropCaps: []api.Capability{"foo"},
			containerCaps: &api.Capabilities{
				Drop: []api.Capability{"bar"},
			},
			shouldPass: false,
		},
		"validation is case sensitive": {
			requiredDropCaps: []api.Capability{"foo"},
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
