package api

import (
	"reflect"
	"strings"
	"testing"

	"k8s.io/kubernetes/pkg/apimachinery/registered"
	"k8s.io/kubernetes/pkg/util/sets"
)

func TestKnownAPIGroups(t *testing.T) {
	unexposedGroups := sets.NewString("authorization.k8s.io", "componentconfig", "metrics", "policy", "federation", "authentication.k8s.io", "rbac.authorization.k8s.io")

	enabledGroups := sets.NewString()
	for _, enabledVersion := range registered.EnabledVersions() {
		enabledGroups.Insert(enabledVersion.Group)
	}

	if missingKnownGroups := KnownKubeAPIGroups.Difference(enabledGroups); len(missingKnownGroups) > 0 {
		t.Errorf("KnownKubeAPIGroups are missing from registered.EnabledVersions: %v", missingKnownGroups.List())
	}
	if unknownEnabledGroups := enabledGroups.Difference(KnownKubeAPIGroups).Difference(unexposedGroups); len(unknownEnabledGroups) > 0 {
		t.Errorf("KnownKubeAPIGroups is missing groups from registered.EnabledVersions: %v", unknownEnabledGroups.List())
	}
}

func TestAllowedAPIVersions(t *testing.T) {
	// Make sure all versions we know about match registered versions
	for group, versions := range KubeAPIGroupsToAllowedVersions {
		enabled := sets.NewString()
		for _, enabledVersion := range registered.EnabledVersionsForGroup(group) {
			enabled.Insert(enabledVersion.Version)
		}
		expected := sets.NewString(versions...)
		actual := enabled.Difference(sets.NewString(KubeAPIGroupsToDeadVersions[group]...))
		if e, a := expected.List(), actual.List(); !reflect.DeepEqual(e, a) {
			t.Errorf("For group %s, expected versions %#v, got %#v", group, e, a)
		}
	}
}

func TestFeatureListAdd(t *testing.T) {
	orderedList := []string{FeatureBuilder, FeatureWebConsole, FeatureS2I}
	fl := FeatureList{}
	if err := fl.Add(FeatureBuilder); err != nil {
		t.Fatalf("failed to add feature %q: %v", FeatureBuilder, err)
	}
	if len(fl) != 1 {
		t.Fatalf("feature list shall contain 1 item")
	}
	if err := fl.Add(strings.ToUpper(FeatureWebConsole)); err != nil {
		t.Fatalf("failed to add feature %q: %v", FeatureWebConsole, err)
	}
	if len(fl) != 2 {
		t.Fatalf("feature list shall contain 1 item")
	}
	for i := 0; i < 2; i++ {
		if fl[i] != orderedList[i] {
			t.Errorf("fl[%d] == %q, but %q is right", i, fl[i], orderedList[i])
		}
	}
	// add already existing
	if err := fl.Add(FeatureBuilder); err != nil {
		t.Fatalf("failed to add feature %q: %v", FeatureBuilder, err)
	}
	// add unknown
	if err := fl.Add("unknown"); err == nil {
		t.Fatalf("adding unknown feature should have failed")
	}
	// add multiple at once
	if err := fl.Add(FeatureWebConsole, FeatureS2I, FeatureBuilder); err != nil {
		t.Fatalf("failed to add multiple features: %v", err)
	}
	if len(fl) != 3 {
		t.Fatalf("feature list has unexpected length (%d != %d)", len(fl), len(orderedList))
	}
	for i := 0; i < 3; i++ {
		if fl[i] != orderedList[i] {
			t.Errorf("fl[%d] == %q, but %q is right", i, fl[i], orderedList[i])
		}
	}
}

func TestFeatureListDelete(t *testing.T) {
	fl := FeatureList(KnownOpenShiftFeatures)
	// try to delete unknown feature
	fl.Delete("unknown")
	if len(fl) != len(KnownOpenShiftFeatures) {
		t.Fatalf("fl.Delete() unexpectedly modified the list: %d != %d", len(fl), len(KnownOpenShiftFeatures))
	}
	fl.Delete(strings.ToUpper(KnownOpenShiftFeatures[1]))
	if len(fl) != len(KnownOpenShiftFeatures)-1 {
		t.Fatalf("Delete() should have removed exactly 1 item: %d != %d", len(fl), len(KnownOpenShiftFeatures)-1)

	}
	flIndex := 0
	// ensure that the order of items is kept
	for i := 0; i < len(KnownOpenShiftFeatures); i++ {
		if i == 1 {
			continue
		}
		if fl[flIndex] != KnownOpenShiftFeatures[i] {
			t.Errorf("fl[%d] == %q, but %q is right", i, fl[flIndex], KnownOpenShiftFeatures[i])
		}
		flIndex++
	}
	// delete multiple items
	fl.Delete(KnownOpenShiftFeatures...)
	if len(fl) != 0 {
		t.Fatal("feature list should be empty")
	}
	// delete on empty list succeeds
	fl.Delete(KnownOpenShiftFeatures[0])
	if len(fl) != 0 {
		t.Fatal("feature list should be empty")
	}
}

func TestFeatureListDeleteByAnAlias(t *testing.T) {
	fl := FeatureList{FeatureWebConsole}
	// try to delete unknown feature
	fl.Delete("web console")
	if len(fl) != 0 {
		t.Fatalf("fl.Delete() should have removed feature %s", FeatureWebConsole)
	}
}

func TestFeatureListDeleteUknown(t *testing.T) {
	fl := FeatureList{FeatureWebConsole, "Unknown Feature"}
	// try to delete unknown feature
	fl.Delete("unknownfeatures")
	if len(fl) != 2 {
		t.Fatalf("fl.Delete() unexpectedly modified the list: %d != 2", len(fl))
	}
	fl.Delete("unknown feature")
	if len(fl) != 1 {
		t.Fatalf("Delete() should have removed exactly 1 item: %d != 1", len(fl))
	}
	if fl[0] != FeatureWebConsole {
		t.Fatalf("Delete() removed wrong item")
	}
}

func TestFeatureListDeleteAliases(t *testing.T) {
	fl := FeatureList{}
	for alias := range FeatureAliases {
		fl = append(fl, alias)
	}
	// try to delete unknown feature
	fl.Delete("unknown")
	if len(fl) != len(FeatureAliases) {
		t.Fatalf("fl.Delete() unexpectedly modified the list: %d != %d", len(fl), len(FeatureAliases))
	}
	fl.Delete("web console")
	if len(fl) != len(FeatureAliases)-1 {
		t.Fatalf("Delete() should have removed exactly 1 item: %d != %d", len(fl), len(FeatureAliases)-1)
	}
	fl.Delete("s2ibuilder")
	if len(fl) != len(FeatureAliases)-2 {
		t.Fatalf("Delete() should have removed exactly 1 item: %d != %d", len(fl), len(FeatureAliases)-2)
	}
}

func testFeatureListCases(t *testing.T, fl FeatureList, cases []string, good bool) {
	for _, name := range cases {
		if good {
			if !fl.Has(name) {
				t.Errorf("feature list {%s} should have %q", strings.Join(fl, ", "), name)
			}
		} else {
			if fl.Has(name) {
				t.Errorf("feature list {%s} shouldn't have %q", strings.Join(fl, ", "), name)
			}
		}
	}
}

func TestFeatureListHas(t *testing.T) {
	fl := FeatureList{FeatureBuilder, FeatureS2I}
	goodCases := []string{
		"builder",
		"BuilDer",
		"S2IBuilder",
		"S2ibuilder",
		"s2i builder",
	}
	badCases := []string{
		"console",
		"CONSOLE",
		"unknown",
		"web-console",
		"S2 I Builder",
	}
	testFeatureListCases(t, fl, goodCases, true)
	testFeatureListCases(t, fl, badCases, false)

	fl = FeatureList{FeatureWebConsole}
	goodCases = []string{
		"WebConsole",
		"Web Console",
		"wEBcONSOLE",
	}
	badCases = []string{
		"console",
		"CONSOLE",
		"unknown",
		"builder",
		"web-console",
		"S2 I Builder",
	}
	testFeatureListCases(t, fl, goodCases, true)
	testFeatureListCases(t, fl, badCases, false)
}

func TestFeatureListHasWithAliases(t *testing.T) {
	fl := FeatureList{}
	for alias := range FeatureAliases {
		fl = append(fl, alias)
	}
	goodCases := []string{
		"web console",
		"WebConsole",
		"S2iBuilder",
		"S2i Builder",
	}
	badCases := []string{
		"Builder",
	}
	testFeatureListCases(t, fl, goodCases, true)
	testFeatureListCases(t, fl, badCases, false)
}

func TestFeatureListHasWithUnknownValue(t *testing.T) {
	fl := FeatureList{"Unknown value"}
	goodCases := []string{
		"unknown value",
		"UNknown Value",
		"Unknown value",
	}
	badCases := []string{
		"web console",
		"WebConsole",
		"S2iBuilder",
		"S2i Builder",
		"unknownvalue",
	}
	testFeatureListCases(t, fl, goodCases, true)
	testFeatureListCases(t, fl, badCases, false)
}
