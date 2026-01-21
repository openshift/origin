package crdversionchecker

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestComputeCRDChanges(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		before   CRDInfo
		after    CRDInfo
		expected *CRDChangeSummary
	}{
		{
			name: "no changes",
			before: CRDInfo{
				Name:  "test.example.io",
				Group: "example.io",
				Kind:  "Test",
				Versions: []CRDVersionInfo{
					{Name: "v1", Served: true, Storage: true},
				},
			},
			after: CRDInfo{
				Name:  "test.example.io",
				Group: "example.io",
				Kind:  "Test",
				Versions: []CRDVersionInfo{
					{Name: "v1", Served: true, Storage: true},
				},
			},
			expected: nil, // No changes
		},
		{
			name: "new version added, storage unchanged",
			before: CRDInfo{
				Name:  "test.example.io",
				Group: "example.io",
				Kind:  "Test",
				Versions: []CRDVersionInfo{
					{Name: "v1", Served: true, Storage: true},
				},
			},
			after: CRDInfo{
				Name:  "test.example.io",
				Group: "example.io",
				Kind:  "Test",
				Versions: []CRDVersionInfo{
					{Name: "v1", Served: true, Storage: true},
					{Name: "v2", Served: true, Storage: false},
				},
			},
			expected: &CRDChangeSummary{
				Name:           "test.example.io",
				AddedVersions:  []string{"v2"},
				StorageChanged: false,
				OldStorage:     "v1",
				NewStorage:     "v1",
			},
		},
		{
			name: "new version added and became storage (violation)",
			before: CRDInfo{
				Name:  "test.example.io",
				Group: "example.io",
				Kind:  "Test",
				Versions: []CRDVersionInfo{
					{Name: "v1", Served: true, Storage: true},
				},
			},
			after: CRDInfo{
				Name:  "test.example.io",
				Group: "example.io",
				Kind:  "Test",
				Versions: []CRDVersionInfo{
					{Name: "v1", Served: true, Storage: false},
					{Name: "v2", Served: true, Storage: true},
				},
			},
			expected: &CRDChangeSummary{
				Name:           "test.example.io",
				AddedVersions:  []string{"v2"},
				StorageChanged: true,
				OldStorage:     "v1",
				NewStorage:     "v2",
			},
		},
		{
			name: "version removed",
			before: CRDInfo{
				Name:  "test.example.io",
				Group: "example.io",
				Kind:  "Test",
				Versions: []CRDVersionInfo{
					{Name: "v1", Served: true, Storage: false},
					{Name: "v2", Served: true, Storage: true},
				},
			},
			after: CRDInfo{
				Name:  "test.example.io",
				Group: "example.io",
				Kind:  "Test",
				Versions: []CRDVersionInfo{
					{Name: "v2", Served: true, Storage: true},
				},
			},
			expected: &CRDChangeSummary{
				Name:            "test.example.io",
				RemovedVersions: []string{"v1"},
				StorageChanged:  false,
				OldStorage:      "v2",
				NewStorage:      "v2",
			},
		},
		{
			name: "storage changed between existing versions",
			before: CRDInfo{
				Name:  "test.example.io",
				Group: "example.io",
				Kind:  "Test",
				Versions: []CRDVersionInfo{
					{Name: "v1", Served: true, Storage: true},
					{Name: "v2", Served: true, Storage: false},
				},
			},
			after: CRDInfo{
				Name:  "test.example.io",
				Group: "example.io",
				Kind:  "Test",
				Versions: []CRDVersionInfo{
					{Name: "v1", Served: true, Storage: false},
					{Name: "v2", Served: true, Storage: true},
				},
			},
			expected: &CRDChangeSummary{
				Name:           "test.example.io",
				StorageChanged: true,
				OldStorage:     "v1",
				NewStorage:     "v2",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actual := computeCRDChanges(tc.before.Name, tc.before, tc.after)
			if diff := cmp.Diff(tc.expected, actual); diff != "" {
				t.Errorf("unexpected result (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetStorageVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		crd      CRDInfo
		expected string
	}{
		{
			name: "single version is storage",
			crd: CRDInfo{
				Versions: []CRDVersionInfo{
					{Name: "v1", Storage: true},
				},
			},
			expected: "v1",
		},
		{
			name: "second version is storage",
			crd: CRDInfo{
				Versions: []CRDVersionInfo{
					{Name: "v1", Storage: false},
					{Name: "v2", Storage: true},
				},
			},
			expected: "v2",
		},
		{
			name: "no storage version (invalid CRD)",
			crd: CRDInfo{
				Versions: []CRDVersionInfo{
					{Name: "v1", Storage: false},
				},
			},
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actual := getStorageVersion(tc.crd)
			if actual != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, actual)
			}
		})
	}
}

func TestIsVersionServed(t *testing.T) {
	t.Parallel()

	crd := CRDInfo{
		Versions: []CRDVersionInfo{
			{Name: "v1", Served: true},
			{Name: "v2", Served: false},
		},
	}

	testCases := []struct {
		version  string
		expected bool
	}{
		{"v1", true},
		{"v2", false},
		{"v3", false}, // doesn't exist
	}

	for _, tc := range testCases {
		t.Run(tc.version, func(t *testing.T) {
			t.Parallel()
			actual := isVersionServed(crd, tc.version)
			if actual != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, actual)
			}
		})
	}
}

func TestCheckNewVersionsNotStoredImmediately(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		before         *CRDSnapshot
		after          *CRDSnapshot
		expectFailure  bool
		expectTestName string
	}{
		{
			name:           "nil snapshots returns passing test",
			before:         nil,
			after:          nil,
			expectFailure:  false,
			expectTestName: "[sig-api-machinery] CRDs with new API versions should not change storage version immediately",
		},
		{
			name: "no new versions - passes",
			before: &CRDSnapshot{
				CRDs: map[string]CRDInfo{
					"test.example.io": {
						Name: "test.example.io",
						Versions: []CRDVersionInfo{
							{Name: "v1", Served: true, Storage: true},
						},
					},
				},
			},
			after: &CRDSnapshot{
				CRDs: map[string]CRDInfo{
					"test.example.io": {
						Name: "test.example.io",
						Versions: []CRDVersionInfo{
							{Name: "v1", Served: true, Storage: true},
						},
					},
				},
			},
			expectFailure: false,
		},
		{
			name: "new version added but not storage - passes",
			before: &CRDSnapshot{
				CRDs: map[string]CRDInfo{
					"test.example.io": {
						Name: "test.example.io",
						Versions: []CRDVersionInfo{
							{Name: "v1", Served: true, Storage: true},
						},
					},
				},
			},
			after: &CRDSnapshot{
				CRDs: map[string]CRDInfo{
					"test.example.io": {
						Name: "test.example.io",
						Versions: []CRDVersionInfo{
							{Name: "v1", Served: true, Storage: true},
							{Name: "v2", Served: true, Storage: false},
						},
					},
				},
			},
			expectFailure: false,
		},
		{
			name: "new version added and is storage - fails",
			before: &CRDSnapshot{
				CRDs: map[string]CRDInfo{
					"test.example.io": {
						Name: "test.example.io",
						Versions: []CRDVersionInfo{
							{Name: "v1", Served: true, Storage: true},
						},
					},
				},
			},
			after: &CRDSnapshot{
				CRDs: map[string]CRDInfo{
					"test.example.io": {
						Name: "test.example.io",
						Versions: []CRDVersionInfo{
							{Name: "v1", Served: true, Storage: false},
							{Name: "v2", Served: true, Storage: true},
						},
					},
				},
			},
			expectFailure: true,
		},
		{
			name: "CRD removed during upgrade - passes",
			before: &CRDSnapshot{
				CRDs: map[string]CRDInfo{
					"test.example.io": {
						Name: "test.example.io",
						Versions: []CRDVersionInfo{
							{Name: "v1", Served: true, Storage: true},
						},
					},
				},
			},
			after: &CRDSnapshot{
				CRDs: map[string]CRDInfo{},
			},
			expectFailure: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			checker := &crdVersionChecker{
				beforeSnapshot: tc.before,
				afterSnapshot:  tc.after,
			}

			results := checker.checkNewVersionsNotStoredImmediately()

			if len(results) == 0 {
				t.Fatal("expected at least one test result")
			}

			result := results[0]
			hasFailure := result.FailureOutput != nil

			if hasFailure != tc.expectFailure {
				t.Errorf("expected failure=%v, got failure=%v", tc.expectFailure, hasFailure)
				if result.FailureOutput != nil {
					t.Logf("failure message: %s", result.FailureOutput.Message)
					t.Logf("failure output: %s", result.FailureOutput.Output)
				}
			}
		})
	}
}

func TestBuildSummary(t *testing.T) {
	t.Parallel()

	checker := &crdVersionChecker{
		beforeSnapshot: &CRDSnapshot{
			CRDs: map[string]CRDInfo{
				"existing.example.io": {
					Name: "existing.example.io",
					Versions: []CRDVersionInfo{
						{Name: "v1", Served: true, Storage: true},
					},
					Conditions: []CRDCondition{},
				},
				"removed.example.io": {
					Name: "removed.example.io",
					Versions: []CRDVersionInfo{
						{Name: "v1", Served: true, Storage: true},
					},
					Conditions: []CRDCondition{},
				},
			},
		},
		afterSnapshot: &CRDSnapshot{
			CRDs: map[string]CRDInfo{
				"existing.example.io": {
					Name: "existing.example.io",
					Versions: []CRDVersionInfo{
						{Name: "v1", Served: true, Storage: true},
						{Name: "v2", Served: true, Storage: false},
					},
					Conditions: []CRDCondition{},
				},
				"added.example.io": {
					Name: "added.example.io",
					Versions: []CRDVersionInfo{
						{Name: "v1", Served: true, Storage: true},
					},
					Conditions: []CRDCondition{},
				},
			},
		},
	}

	summary := checker.buildSummary()

	// Check added CRDs
	if len(summary.AddedCRDs) != 1 || summary.AddedCRDs[0] != "added.example.io" {
		t.Errorf("expected added CRDs [added.example.io], got %v", summary.AddedCRDs)
	}

	// Check removed CRDs
	if len(summary.RemovedCRDs) != 1 || summary.RemovedCRDs[0] != "removed.example.io" {
		t.Errorf("expected removed CRDs [removed.example.io], got %v", summary.RemovedCRDs)
	}

	// Check changed CRDs
	if len(summary.ChangedCRDs) != 1 {
		t.Errorf("expected 1 changed CRD, got %d", len(summary.ChangedCRDs))
	} else {
		change := summary.ChangedCRDs[0]
		if change.Name != "existing.example.io" {
			t.Errorf("expected changed CRD name 'existing.example.io', got %q", change.Name)
		}
		if len(change.AddedVersions) != 1 || change.AddedVersions[0] != "v2" {
			t.Errorf("expected added version [v2], got %v", change.AddedVersions)
		}
	}
}
