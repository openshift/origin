package kubernetes

import (
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/genericapiserver"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

func TestGetAPIGroupVersionOverrides(t *testing.T) {
	testcases := map[string]struct {
		DisabledVersions  map[string][]string
		ExpectedOverrides map[string]genericapiserver.APIGroupVersionOverride
	}{
		"empty": {
			DisabledVersions:  nil,
			ExpectedOverrides: map[string]genericapiserver.APIGroupVersionOverride{},
		},
		"* -> v1": {
			DisabledVersions:  map[string][]string{"": {"*"}},
			ExpectedOverrides: map[string]genericapiserver.APIGroupVersionOverride{"api/v1": {Disable: true}},
		},
		"v1": {
			DisabledVersions:  map[string][]string{"": {"v1"}},
			ExpectedOverrides: map[string]genericapiserver.APIGroupVersionOverride{"api/v1": {Disable: true}},
		},
		"* -> v1beta1": {
			DisabledVersions:  map[string][]string{"extensions": {"*"}},
			ExpectedOverrides: map[string]genericapiserver.APIGroupVersionOverride{"extensions/v1beta1": {Disable: true}},
		},
		"extensions/v1beta1": {
			DisabledVersions:  map[string][]string{"extensions": {"v1beta1"}},
			ExpectedOverrides: map[string]genericapiserver.APIGroupVersionOverride{"extensions/v1beta1": {Disable: true}},
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
