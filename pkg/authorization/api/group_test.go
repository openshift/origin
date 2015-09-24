package api

import (
	"testing"

	"k8s.io/kubernetes/pkg/util/sets"
)

func TestEscalating(t *testing.T) {
	escalatingResources := ExpandResources(sets.NewString(GroupsToResources[EscalatingResourcesGroupName]...))
	nonEscalatingResources := ExpandResources(sets.NewString(GroupsToResources[NonEscalatingResourcesGroupName]...))
	if len(nonEscalatingResources) <= len(escalatingResources) {
		t.Errorf("groups look bad: escalating=%v nonescalating=%v", escalatingResources.List(), nonEscalatingResources.List())
	}
}
