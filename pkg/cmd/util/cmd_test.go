package util_test

import (
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	"k8s.io/kubernetes/pkg/kubectl"

	_ "github.com/openshift/origin/pkg/api/install"
	"github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

func TestResolveResource(t *testing.T) {
	mapper := clientcmd.ShortcutExpander{RESTMapper: kubectl.ShortcutExpander{RESTMapper: registered.RESTMapper()}}

	tests := []struct {
		name             string
		defaultResource  unversioned.GroupResource
		resourceString   string
		expectedResource unversioned.GroupResource
		expectedName     string
		expectedErr      bool
	}{
		{
			name:             "invalid case #1",
			defaultResource:  unversioned.GroupResource{Resource: "foo"},
			resourceString:   "a/b/c",
			expectedResource: unversioned.GroupResource{},
			expectedName:     "",
			expectedErr:      true,
		},
		{
			name:             "invalid case #2",
			defaultResource:  unversioned.GroupResource{Resource: "foo"},
			resourceString:   "foo/bar",
			expectedResource: unversioned.GroupResource{},
			expectedName:     "",
			expectedErr:      true,
		},
		{
			name:             "empty resource string case #1",
			defaultResource:  unversioned.GroupResource{Resource: ""},
			resourceString:   "",
			expectedResource: unversioned.GroupResource{Resource: ""},
			expectedName:     "",
			expectedErr:      false,
		},
		{
			name:             "empty resource string case #2",
			defaultResource:  unversioned.GroupResource{Resource: ""},
			resourceString:   "bar",
			expectedResource: unversioned.GroupResource{Resource: ""},
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "empty resource string case #3",
			defaultResource:  unversioned.GroupResource{Resource: "foo"},
			resourceString:   "bar",
			expectedResource: unversioned.GroupResource{Resource: "foo"},
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(KUBE) short name",
			defaultResource:  unversioned.GroupResource{Resource: "foo"},
			resourceString:   "rc/bar",
			expectedResource: unversioned.GroupResource{Resource: "replicationcontrollers"},
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(KUBE) long name, case insensitive #1",
			defaultResource:  unversioned.GroupResource{Resource: "foo"},
			resourceString:   "replicationcontroller/bar",
			expectedResource: unversioned.GroupResource{Resource: "replicationcontrollers"},
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(KUBE) long name, case insensitive #2",
			defaultResource:  unversioned.GroupResource{Resource: "foo"},
			resourceString:   "replicationcontrollers/bar",
			expectedResource: unversioned.GroupResource{Resource: "replicationcontrollers"},
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(KUBE) long name, case insensitive #3",
			defaultResource:  unversioned.GroupResource{Resource: "foo"},
			resourceString:   "ReplicationControllers/bar",
			expectedResource: unversioned.GroupResource{Resource: "replicationcontrollers"},
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(KUBE) long name, case insensitive #4",
			defaultResource:  unversioned.GroupResource{Resource: "foo"},
			resourceString:   "ReplicationControllers/bar",
			expectedResource: unversioned.GroupResource{Resource: "replicationcontrollers"},
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(KUBE) long name, case insensitive #5",
			defaultResource:  unversioned.GroupResource{Resource: "foo"},
			resourceString:   "ReplicationControllers/Bar",
			expectedResource: unversioned.GroupResource{Resource: "replicationcontrollers"},
			expectedName:     "Bar",
			expectedErr:      false,
		},
		{
			name:             "(ORIGIN) short name",
			defaultResource:  unversioned.GroupResource{Resource: "foo"},
			resourceString:   "bc/bar",
			expectedResource: unversioned.GroupResource{Resource: "buildconfigs"},
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(ORIGIN) long name, case insensitive #1",
			defaultResource:  unversioned.GroupResource{Resource: "foo"},
			resourceString:   "buildconfig/bar",
			expectedResource: unversioned.GroupResource{Resource: "buildconfigs"},
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(ORIGIN) long name, case insensitive #2",
			defaultResource:  unversioned.GroupResource{Resource: "foo"},
			resourceString:   "buildconfigs/bar",
			expectedResource: unversioned.GroupResource{Resource: "buildconfigs"},
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(ORIGIN) long name, case insensitive #3",
			defaultResource:  unversioned.GroupResource{Resource: "foo"},
			resourceString:   "BuildConfigs/bar",
			expectedResource: unversioned.GroupResource{Resource: "buildconfigs"},
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(ORIGIN) long name, case insensitive #4",
			defaultResource:  unversioned.GroupResource{Resource: "foo"},
			resourceString:   "BuildConfigs/bar",
			expectedResource: unversioned.GroupResource{Resource: "buildconfigs"},
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(ORIGIN) long name, case insensitive #5",
			defaultResource:  unversioned.GroupResource{Resource: "foo"},
			resourceString:   "BuildConfigs/Bar",
			expectedResource: unversioned.GroupResource{Resource: "buildconfigs"},
			expectedName:     "Bar",
			expectedErr:      false,
		},

		{
			name:             "singular, implicit api group",
			defaultResource:  unversioned.GroupResource{},
			resourceString:   "job/myjob",
			expectedResource: unversioned.GroupResource{Group: "extensions", Resource: "jobs"},
			expectedName:     "myjob",
			expectedErr:      false,
		},
		{
			name:             "singular, explicit extensions api group",
			defaultResource:  unversioned.GroupResource{},
			resourceString:   "job.extensions/myjob",
			expectedResource: unversioned.GroupResource{Group: "extensions", Resource: "jobs"},
			expectedName:     "myjob",
			expectedErr:      false,
		},
		{
			name:             "singular, explicit batch api group",
			defaultResource:  unversioned.GroupResource{},
			resourceString:   "job.batch/myjob",
			expectedResource: unversioned.GroupResource{Group: "batch", Resource: "jobs"},
			expectedName:     "myjob",
			expectedErr:      false,
		},
		{
			name:             "shortname, implicit api group",
			defaultResource:  unversioned.GroupResource{},
			resourceString:   "hpa/myhpa",
			expectedResource: unversioned.GroupResource{Group: "extensions", Resource: "horizontalpodautoscalers"},
			expectedName:     "myhpa",
			expectedErr:      false,
		},
		{
			name:             "shortname, explicit extensions api group",
			defaultResource:  unversioned.GroupResource{},
			resourceString:   "hpa.extensions/myhpa",
			expectedResource: unversioned.GroupResource{Group: "extensions", Resource: "horizontalpodautoscalers"},
			expectedName:     "myhpa",
			expectedErr:      false,
		},
		{
			name:             "shortname, explicit autoscaling api group",
			defaultResource:  unversioned.GroupResource{},
			resourceString:   "hpa.autoscaling/myhpa",
			expectedResource: unversioned.GroupResource{Group: "autoscaling", Resource: "horizontalpodautoscalers"},
			expectedName:     "myhpa",
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
		if !reflect.DeepEqual(gotResource, test.expectedResource) {
			t.Errorf("%s: expected resource type %#v, got %#v", test.name, test.expectedResource, gotResource)
			continue
		}
		if gotName != test.expectedName {
			t.Errorf("%s: expected resource name %s, got %s", test.name, test.expectedName, gotName)
			continue
		}
	}
}
