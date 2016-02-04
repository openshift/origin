package kubernetes

import (
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/master"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

func TestGetAPIGroupVersionOverrides(t *testing.T) {
	testcases := map[string]struct {
		DisabledVersions  map[string][]string
		ExpectedOverrides map[string]master.APIGroupVersionOverride
	}{
		"empty": {
			DisabledVersions:  nil,
			ExpectedOverrides: map[string]master.APIGroupVersionOverride{},
		},
		"* -> v1": {
			DisabledVersions:  map[string][]string{"": {"*"}},
			ExpectedOverrides: map[string]master.APIGroupVersionOverride{"api/v1": {Disable: true}},
		},
		"v1": {
			DisabledVersions:  map[string][]string{"": {"v1"}},
			ExpectedOverrides: map[string]master.APIGroupVersionOverride{"api/v1": {Disable: true}},
		},
		"* -> v1beta1": {
			DisabledVersions:  map[string][]string{"extensions": {"*"}},
			ExpectedOverrides: map[string]master.APIGroupVersionOverride{"extensions/v1beta1": {Disable: true}},
		},
		"extensions/v1beta1": {
			DisabledVersions:  map[string][]string{"extensions": {"v1beta1"}},
			ExpectedOverrides: map[string]master.APIGroupVersionOverride{"extensions/v1beta1": {Disable: true}},
		},
	}

	for k, tc := range testcases {
		config := configapi.MasterConfig{KubernetesMasterConfig: &configapi.KubernetesMasterConfig{DisabledAPIGroupVersions: tc.DisabledVersions}}
		overrides := getAPIGroupVersionOverrides(config)
		if !reflect.DeepEqual(overrides, tc.ExpectedOverrides) {
			t.Errorf("%s: Expected\n%#v\ngot\n%#v", k, tc.ExpectedOverrides, overrides)
		}
	}
}
