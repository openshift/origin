package util_test

import (
	"testing"

	"k8s.io/kubernetes/pkg/kubectl"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

func TestResolveResource(t *testing.T) {
	mapper := clientcmd.ShortcutExpander{RESTMapper: kubectl.ShortcutExpander{RESTMapper: latest.RESTMapper}}

	tests := []struct {
		name             string
		defaultResource  string
		resourceString   string
		expectedResource string
		expectedName     string
		expectedErr      bool
	}{
		{
			name:             "invalid case #1",
			defaultResource:  "",
			resourceString:   "a/b/c",
			expectedResource: "",
			expectedName:     "",
			expectedErr:      true,
		},
		{
			name:             "invalid case #2",
			defaultResource:  "",
			resourceString:   "foo/bar",
			expectedResource: "",
			expectedName:     "",
			expectedErr:      true,
		},
		{
			name:             "empty resource string case #1",
			defaultResource:  "",
			resourceString:   "",
			expectedResource: "",
			expectedName:     "",
			expectedErr:      false,
		},
		{
			name:             "empty resource string case #2",
			defaultResource:  "",
			resourceString:   "bar",
			expectedResource: "",
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "empty resource string case #3",
			defaultResource:  "foo",
			resourceString:   "bar",
			expectedResource: "foo",
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(KUBE) short name",
			defaultResource:  "foo",
			resourceString:   "rc/bar",
			expectedResource: "replicationcontrollers",
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(KUBE) long name, case insensitive #1",
			defaultResource:  "foo",
			resourceString:   "replicationcontroller/bar",
			expectedResource: "replicationcontrollers",
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(KUBE) long name, case insensitive #2",
			defaultResource:  "foo",
			resourceString:   "replicationcontrollers/bar",
			expectedResource: "replicationcontrollers",
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(KUBE) long name, case insensitive #3",
			defaultResource:  "foo",
			resourceString:   "ReplicationControllers/bar",
			expectedResource: "replicationcontrollers",
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(KUBE) long name, case insensitive #4",
			defaultResource:  "foo",
			resourceString:   "ReplicationControllers/bar",
			expectedResource: "replicationcontrollers",
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(KUBE) long name, case insensitive #5",
			defaultResource:  "foo",
			resourceString:   "ReplicationControllers/Bar",
			expectedResource: "replicationcontrollers",
			expectedName:     "Bar",
			expectedErr:      false,
		},
		{
			name:             "(ORIGIN) short name",
			defaultResource:  "foo",
			resourceString:   "bc/bar",
			expectedResource: "buildconfigs",
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(ORIGIN) long name, case insensitive #1",
			defaultResource:  "foo",
			resourceString:   "buildconfig/bar",
			expectedResource: "buildconfigs",
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(ORIGIN) long name, case insensitive #2",
			defaultResource:  "foo",
			resourceString:   "buildconfigs/bar",
			expectedResource: "buildconfigs",
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(ORIGIN) long name, case insensitive #3",
			defaultResource:  "foo",
			resourceString:   "BuildConfigs/bar",
			expectedResource: "buildconfigs",
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(ORIGIN) long name, case insensitive #4",
			defaultResource:  "foo",
			resourceString:   "BuildConfigs/bar",
			expectedResource: "buildconfigs",
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(ORIGIN) long name, case insensitive #5",
			defaultResource:  "foo",
			resourceString:   "BuildConfigs/Bar",
			expectedResource: "buildconfigs",
			expectedName:     "Bar",
			expectedErr:      false,
		},
	}

	for _, test := range tests {
		gotResource, gotName, gotErr := util.ResolveResource(test.defaultResource, test.resourceString, mapper)
		if gotErr != nil && !test.expectedErr {
			t.Errorf("%s: expected no error, got %v", test.name, gotErr)
			continue
		}
		if gotErr == nil && test.expectedErr {
			t.Errorf("%s: expected error but got none", test.name)
			continue
		}
		if gotResource != test.expectedResource {
			t.Errorf("%s: expected resource type %s, got %s", test.name, test.expectedResource, gotResource)
			continue
		}
		if gotName != test.expectedName {
			t.Errorf("%s: expected resource name %s, got %s", test.name, test.expectedName, gotName)
			continue
		}
	}
}
