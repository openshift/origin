package scheduler

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"

	projectcache "github.com/openshift/origin/pkg/project/cache"
)

type FakeNodeInfo kapi.Node

func (n FakeNodeInfo) GetNodeInfo(nodeName string) (*kapi.Node, error) {
	node := kapi.Node(n)
	return &node, nil
}

func TestPodFitsProjectSelector(t *testing.T) {
	mockClient := &testclient.Fake{}
	project := &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "testProject",
			Namespace: "",
		},
	}
	projectStore := cache.NewStore(cache.MetaNamespaceIndexFunc)
	projectStore.Add(project)

	pod := kapi.Pod{ObjectMeta: kapi.ObjectMeta{Name: "testPod"}}
	node := kapi.Node{ObjectMeta: kapi.ObjectMeta{Name: "testNode"}}

	tests := []struct {
		defaultNodeSelector string
		projectNodeSelector string
		nodeLabels          map[string]string
		fits                bool
		testName            string
	}{
		{
			defaultNodeSelector: "",
			projectNodeSelector: "",
			nodeLabels:          map[string]string{"infra": "false"},
			fits:                true,
			testName:            "No node selectors",
		},
		{
			defaultNodeSelector: "infra=false",
			projectNodeSelector: "",
			nodeLabels:          map[string]string{"infra": "false"},
			fits:                true,
			testName:            "Matches default node selector",
		},
		{
			defaultNodeSelector: "env=test",
			projectNodeSelector: "",
			nodeLabels:          map[string]string{"infra": "false"},
			fits:                false,
			testName:            "Doesn't match default node selector",
		},
		{
			defaultNodeSelector: "",
			projectNodeSelector: "infra=false",
			nodeLabels:          map[string]string{"infra": "false"},
			fits:                true,
			testName:            "Matches project node selector",
		},
		{
			defaultNodeSelector: "infra=false",
			projectNodeSelector: "env=test",
			nodeLabels:          map[string]string{"infra": "false"},
			fits:                false,
			testName:            "Doesn't match project node selector",
		},
	}
	for _, test := range tests {
		node.ObjectMeta.Labels = test.nodeLabels
		projectcache.FakeProjectCache(mockClient, projectStore, test.defaultNodeSelector)
		project.ObjectMeta.Annotations = map[string]string{"nodeSelector": test.projectNodeSelector}
		predicate := projectNodeSelector{FakeNodeInfo(node)}
		fits, err := predicate.ProjectSelectorMatches(pod, []kapi.Pod{}, "machine")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if fits != test.fits {
			t.Errorf("%s: expected: %v got %v", test.testName, test.fits, fits)
		}
	}
}
