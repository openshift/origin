package api

import (
	"testing"

	"k8s.io/kubernetes/pkg/util/sets"
)

func TestEscalating(t *testing.T) {
	escalatingResources := NormalizeResources(sets.NewString(groupsToResources[escalatingResourcesGroupName]...))
	nonEscalatingResources := NormalizeResources(sets.NewString(groupsToResources[nonescalatingResourcesGroupName]...))
	if len(nonEscalatingResources) <= len(escalatingResources) {
		t.Errorf("groups look bad: escalating=%v nonescalating=%v", escalatingResources.List(), nonEscalatingResources.List())
	}
}

func TestNormalizeResources(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		expected string
	}{
		{"capA", "capA", "capa"},
		{"capH", "capH", "caph"},
		{"capZ", "capZ", "capz"},
		{"group", buildGroupName, "builds"},
	}

	for _, test := range tests {
		normalizedNames := NormalizeResources(sets.NewString(test.resource))

		if !normalizedNames.Has(test.expected) {
			t.Errorf("%s: expected %s, got %v", test.name, test.expected, normalizedNames)
		}

	}
}

func TestNeedsNormalization(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		expected bool
	}{
		{"cap", "G", true},
		{"lowera", "lowera", false},
		{"lowerh", "lowerh", false},
		{"lowerz", "lowerz", false},
		{"0", "0", false},
		{"5", "5", false},
		{"9", "9", false},
		{"/", "/", false},
		{"-", "-", false},
		{".", ".", false},
		{resourceGroupPrefix, resourceGroupPrefix, true},
	}

	for _, test := range tests {
		needsNormalizing := needsNormalizing(test.resource)

		if needsNormalizing != test.expected {
			t.Errorf("%s: expected %v, got %v", test.name, test.expected, needsNormalizing)
		}

	}
}
