package util_test

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	kapi "k8s.io/kubernetes/pkg/api"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	_ "github.com/openshift/origin/pkg/api/install"
	"github.com/openshift/origin/pkg/cmd/util"
)

func TestResolveResource(t *testing.T) {
	dc := fake.NewSimpleClientset().Discovery()
	mapper, err := kcmdutil.NewShortcutExpander(kapi.Registry.RESTMapper(), dc)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	tests := []struct {
		name             string
		defaultResource  schema.GroupResource
		resourceString   string
		expectedResource schema.GroupResource
		expectedName     string
		expectedErr      bool
	}{
		{
			name:             "invalid case #1",
			defaultResource:  schema.GroupResource{Resource: "foo"},
			resourceString:   "a/b/c",
			expectedResource: schema.GroupResource{},
			expectedName:     "",
			expectedErr:      true,
		},
		{
			name:             "invalid case #2",
			defaultResource:  schema.GroupResource{Resource: "foo"},
			resourceString:   "foo/bar",
			expectedResource: schema.GroupResource{},
			expectedName:     "",
			expectedErr:      true,
		},
		{
			name:             "empty resource string case #1",
			defaultResource:  schema.GroupResource{Resource: ""},
			resourceString:   "",
			expectedResource: schema.GroupResource{Resource: ""},
			expectedName:     "",
			expectedErr:      false,
		},
		{
			name:             "empty resource string case #2",
			defaultResource:  schema.GroupResource{Resource: ""},
			resourceString:   "bar",
			expectedResource: schema.GroupResource{Resource: ""},
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "empty resource string case #3",
			defaultResource:  schema.GroupResource{Resource: "foo"},
			resourceString:   "bar",
			expectedResource: schema.GroupResource{Resource: "foo"},
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(KUBE) short name",
			defaultResource:  schema.GroupResource{Resource: "foo"},
			resourceString:   "rc/bar",
			expectedResource: schema.GroupResource{Resource: "replicationcontrollers"},
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(KUBE) long name, case insensitive #1",
			defaultResource:  schema.GroupResource{Resource: "foo"},
			resourceString:   "replicationcontroller/bar",
			expectedResource: schema.GroupResource{Resource: "replicationcontrollers"},
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(KUBE) long name, case insensitive #2",
			defaultResource:  schema.GroupResource{Resource: "foo"},
			resourceString:   "replicationcontrollers/bar",
			expectedResource: schema.GroupResource{Resource: "replicationcontrollers"},
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(KUBE) long name, case insensitive #3",
			defaultResource:  schema.GroupResource{Resource: "foo"},
			resourceString:   "ReplicationControllers/bar",
			expectedResource: schema.GroupResource{Resource: "replicationcontrollers"},
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(KUBE) long name, case insensitive #4",
			defaultResource:  schema.GroupResource{Resource: "foo"},
			resourceString:   "ReplicationControllers/bar",
			expectedResource: schema.GroupResource{Resource: "replicationcontrollers"},
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(KUBE) long name, case insensitive #5",
			defaultResource:  schema.GroupResource{Resource: "foo"},
			resourceString:   "ReplicationControllers/Bar",
			expectedResource: schema.GroupResource{Resource: "replicationcontrollers"},
			expectedName:     "Bar",
			expectedErr:      false,
		},
		{
			name:             "(ORIGIN) short name",
			defaultResource:  schema.GroupResource{Resource: "foo"},
			resourceString:   "bc/bar",
			expectedResource: schema.GroupResource{Resource: "buildconfigs"},
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(ORIGIN) long name, case insensitive #1",
			defaultResource:  schema.GroupResource{Resource: "foo"},
			resourceString:   "buildconfig/bar",
			expectedResource: schema.GroupResource{Resource: "buildconfigs"},
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(ORIGIN) long name, case insensitive #2",
			defaultResource:  schema.GroupResource{Resource: "foo"},
			resourceString:   "buildconfigs/bar",
			expectedResource: schema.GroupResource{Resource: "buildconfigs"},
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(ORIGIN) long name, case insensitive #3",
			defaultResource:  schema.GroupResource{Resource: "foo"},
			resourceString:   "BuildConfigs/bar",
			expectedResource: schema.GroupResource{Resource: "buildconfigs"},
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(ORIGIN) long name, case insensitive #4",
			defaultResource:  schema.GroupResource{Resource: "foo"},
			resourceString:   "BuildConfigs/bar",
			expectedResource: schema.GroupResource{Resource: "buildconfigs"},
			expectedName:     "bar",
			expectedErr:      false,
		},
		{
			name:             "(ORIGIN) long name, case insensitive #5",
			defaultResource:  schema.GroupResource{Resource: "foo"},
			resourceString:   "BuildConfigs/Bar",
			expectedResource: schema.GroupResource{Resource: "buildconfigs"},
			expectedName:     "Bar",
			expectedErr:      false,
		},

		{
			name:             "singular, implicit api group",
			defaultResource:  schema.GroupResource{},
			resourceString:   "job/myjob",
			expectedResource: schema.GroupResource{Group: "batch", Resource: "jobs"},
			expectedName:     "myjob",
			expectedErr:      false,
		},
		{
			name:             "singular, explicit extensions api group",
			defaultResource:  schema.GroupResource{},
			resourceString:   "job.extensions/myjob",
			expectedResource: schema.GroupResource{},
			expectedName:     "",
			expectedErr:      true,
		},
		{
			name:             "singular, explicit batch api group",
			defaultResource:  schema.GroupResource{},
			resourceString:   "job.batch/myjob",
			expectedResource: schema.GroupResource{Group: "batch", Resource: "jobs"},
			expectedName:     "myjob",
			expectedErr:      false,
		},
		{
			name:             "shortname, implicit api group",
			defaultResource:  schema.GroupResource{},
			resourceString:   "hpa/myhpa",
			expectedResource: schema.GroupResource{Group: "autoscaling", Resource: "horizontalpodautoscalers"}, // there is no extensions hpa anymore
			expectedName:     "myhpa",
			expectedErr:      false,
		},
		{
			name:             "shortname, explicit autoscaling api group",
			defaultResource:  schema.GroupResource{},
			resourceString:   "hpa.autoscaling/myhpa",
			expectedResource: schema.GroupResource{Group: "autoscaling", Resource: "horizontalpodautoscalers"},
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
