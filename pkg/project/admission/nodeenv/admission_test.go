package admission

import (
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/admission"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"

	projectcache "github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/util/labelselector"
)

// TestPodAdmission verifies various scenarios involving pod/project/global node label selectors
func TestPodAdmission(t *testing.T) {
	mockClient := &testclient.Fake{}
	project := &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "testProject",
			Namespace: "",
		},
	}
	projectStore := cache.NewStore(cache.MetaNamespaceIndexFunc)
	projectStore.Add(project)

	handler := &podNodeEnvironment{client: mockClient}
	pod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{Name: "testPod"},
	}

	tests := []struct {
		defaultNodeSelector       string
		projectNodeSelector       string
		podNodeSelector           map[string]string
		mergedNodeSelector        map[string]string
		ignoreProjectNodeSelector bool
		admit                     bool
		testName                  string
	}{
		{
			defaultNodeSelector:       "",
			podNodeSelector:           map[string]string{},
			mergedNodeSelector:        map[string]string{},
			ignoreProjectNodeSelector: true,
			admit:    true,
			testName: "No node selectors",
		},
		{
			defaultNodeSelector:       "infra = false",
			podNodeSelector:           map[string]string{},
			mergedNodeSelector:        map[string]string{"infra": "false"},
			ignoreProjectNodeSelector: true,
			admit:    true,
			testName: "Default node selector and no conflicts",
		},
		{
			defaultNodeSelector: "",
			projectNodeSelector: "infra = false",
			podNodeSelector:     map[string]string{},
			mergedNodeSelector:  map[string]string{"infra": "false"},
			admit:               true,
			testName:            "Project node selector and no conflicts",
		},
		{
			defaultNodeSelector: "infra = false",
			projectNodeSelector: "",
			podNodeSelector:     map[string]string{},
			mergedNodeSelector:  map[string]string{},
			admit:               true,
			testName:            "Empty project node selector and no conflicts",
		},
		{
			defaultNodeSelector: "infra = false",
			projectNodeSelector: "infra=true",
			podNodeSelector:     map[string]string{},
			mergedNodeSelector:  map[string]string{"infra": "true"},
			admit:               true,
			testName:            "Default and project node selector, no conflicts",
		},
		{
			defaultNodeSelector: "infra = false",
			projectNodeSelector: "infra=true",
			podNodeSelector:     map[string]string{"env": "test"},
			mergedNodeSelector:  map[string]string{"infra": "true", "env": "test"},
			admit:               true,
			testName:            "Project and pod node selector, no conflicts",
		},
		{
			defaultNodeSelector: "env = test",
			projectNodeSelector: "infra=true",
			podNodeSelector:     map[string]string{"infra": "false"},
			mergedNodeSelector:  map[string]string{"infra": "false"},
			admit:               false,
			testName:            "Conflicting pod and project node selector, one label",
		},
		{
			defaultNodeSelector: "env=dev",
			projectNodeSelector: "infra=false, env = test",
			podNodeSelector:     map[string]string{"env": "dev", "color": "blue"},
			mergedNodeSelector:  map[string]string{"env": "dev", "color": "blue"},
			admit:               false,
			testName:            "Conflicting pod and project node selector, multiple labels",
		},
	}
	for _, test := range tests {
		projectcache.FakeProjectCache(mockClient, projectStore, test.defaultNodeSelector)
		if !test.ignoreProjectNodeSelector {
			project.ObjectMeta.Annotations = map[string]string{"openshift.io/node-selector": test.projectNodeSelector}
		}
		pod.Spec = kapi.PodSpec{NodeSelector: test.podNodeSelector}

		err := handler.Admit(admission.NewAttributesRecord(pod, "Pod", "namespace", project.ObjectMeta.Name, "pods", "", admission.Create, nil))
		if test.admit && err != nil {
			t.Errorf("Test: %s, expected no error but got: %s", test.testName, err)
		} else if !test.admit && err == nil {
			t.Errorf("Test: %s, expected an error", test.testName)
		}

		if !labelselector.Equals(test.mergedNodeSelector, pod.Spec.NodeSelector) {
			t.Errorf("Test: %s, expected: %s but got: %s", test.testName, test.mergedNodeSelector, pod.Spec.NodeSelector)
		}
	}
}
