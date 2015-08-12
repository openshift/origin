package api

import (
	"testing"

	"k8s.io/kubernetes/pkg/util"
)

func TestEscalating(t *testing.T) {
	escalatingResources := ExpandResources(util.NewStringSet(GroupsToResources[EscalatingResourcesGroupName]...))
	nonEscalatingResources := ExpandResources(util.NewStringSet(GroupsToResources[NonEscalatingResourcesGroupName]...))
	if len(nonEscalatingResources) <= len(escalatingResources) {
		t.Errorf("groups look bad: escalating=%v nonescalating=%v", escalatingResources.List(), nonEscalatingResources.List())
	}
}
