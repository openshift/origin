package api

import (
	"strings"
	"testing"
)

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

func TestFeatureListHas(t *testing.T) {
	fl := FeatureList{FeatureBuilder, FeatureS2I}
	goodCases := []string{
		"builder",
		"BuilDer",
		"S2I Builder",
		"S2i builder",
	}
	badCases := []string{
		"console",
		"CONSOLE",
		"unknown",
		"s2ibuilder",
	}
	for _, gc := range goodCases {
		if !fl.Has(gc) {
			t.Errorf("feature list should have %q", gc)
		}
	}
	for _, bc := range badCases {
		if fl.Has(bc) {
			t.Errorf("feature list shouldn't have %q", bc)
		}
	}
}
