package crdversionchecker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/sirupsen/logrus"
)

// CRDVersionInfo captures the version information for a single CRD version.
type CRDVersionInfo struct {
	Name       string `json:"name"`                 // Version name (e.g., v1, v1beta1)
	Served     bool   `json:"served"`               // Whether this version is served by the API server
	Storage    bool   `json:"storage"`              // Whether this is the storage version
	Deprecated bool   `json:"deprecated,omitempty"` // Whether this version is deprecated
}

// CRDCondition captures the condition information for a CRD.
type CRDCondition struct {
	Type               string `json:"type"`                         // Condition type (e.g., Established, NamesAccepted)
	Status             string `json:"status"`                       // True, False, or Unknown
	Reason             string `json:"reason,omitempty"`             // Machine-readable reason for the condition
	Message            string `json:"message,omitempty"`            // Human-readable message
	LastTransitionTime string `json:"lastTransitionTime,omitempty"` // When the condition last transitioned
}

// CRDInfo captures the essential information about a CustomResourceDefinition.
type CRDInfo struct {
	Name           string           `json:"name"`           // Full CRD name (e.g., machines.machine.openshift.io)
	Group          string           `json:"group"`          // API group
	Kind           string           `json:"kind"`           // Resource kind
	Versions       []CRDVersionInfo `json:"versions"`       // All defined versions
	StoredVersions []string         `json:"storedVersions"` // Versions that have data stored in etcd
	Conditions     []CRDCondition   `json:"conditions"`     // Current conditions of the CRD
}

// CRDSnapshot represents a point-in-time snapshot of all CRDs in the cluster.
type CRDSnapshot struct {
	CollectedAt time.Time          `json:"collectedAt"`
	CRDs        map[string]CRDInfo `json:"crds"` // Keyed by CRD name
}

// CRDSummary contains the before/after snapshots and computed differences.
type CRDSummary struct {
	BeforeUpgrade CRDSnapshot        `json:"beforeUpgrade"` // Snapshot of CRDs before the upgrade
	AfterUpgrade  CRDSnapshot        `json:"afterUpgrade"`  // Snapshot of CRDs after the upgrade
	AddedCRDs     []string           `json:"addedCRDs"`     // CRDs that were added during upgrade
	RemovedCRDs   []string           `json:"removedCRDs"`   // CRDs that were removed during upgrade
	ChangedCRDs   []CRDChangeSummary `json:"changedCRDs"`   // CRDs with version changes
}

// CRDChangeSummary describes changes to a specific CRD between snapshots.
type CRDChangeSummary struct {
	Name            string   `json:"name"`                      // Name of the CRD
	AddedVersions   []string `json:"addedVersions,omitempty"`   // Versions that were added
	RemovedVersions []string `json:"removedVersions,omitempty"` // Versions that were removed
	StorageChanged  bool     `json:"storageChanged,omitempty"`  // True if storage version changed
	OldStorage      string   `json:"oldStorage,omitempty"`      // Old storage version
	NewStorage      string   `json:"newStorage,omitempty"`      // New storage version
}

// crdVersionChecker monitors CRDs before and after an upgrade to detect version changes.
type crdVersionChecker struct {
	adminRESTConfig    *rest.Config
	notSupportedReason error

	// Snapshots collected during the test
	beforeSnapshot *CRDSnapshot
	afterSnapshot  *CRDSnapshot
}

// NewCRDVersionChecker creates a new monitor test that tracks CRD version changes during upgrades.
func NewCRDVersionChecker() monitortestframework.MonitorTest {
	return &crdVersionChecker{}
}

func (c *crdVersionChecker) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

// StartCollection gathers the initial CRD snapshot before the upgrade begins.
func (c *crdVersionChecker) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	c.adminRESTConfig = adminRESTConfig

	// Collect the initial CRD snapshot (before upgrade)
	snapshot, err := c.collectCRDSnapshot(ctx)
	if err != nil {
		logrus.WithError(err).Error("failed to collect initial CRD snapshot")
		return err
	}
	c.beforeSnapshot = snapshot
	logrus.Infof("Collected initial CRD snapshot with %d CRDs", len(snapshot.CRDs))

	return nil
}

func (c *crdVersionChecker) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if c.notSupportedReason != nil {
		return nil, nil, c.notSupportedReason
	}
	return nil, nil, nil
}

func (c *crdVersionChecker) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

// EvaluateTestsFromConstructedIntervals collects the post-upgrade CRD snapshot and runs validation checks.
// This method is called after the upgrade completes, making it the ideal place to gather the "after" state.
func (c *crdVersionChecker) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	if c.notSupportedReason != nil {
		return nil, nil
	}

	// Collect the post-upgrade CRD snapshot
	afterSnapshot, err := c.collectCRDSnapshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to collect post-upgrade CRD snapshot: %w", err)
	}
	c.afterSnapshot = afterSnapshot
	logrus.Infof("Collected post-upgrade CRD snapshot with %d CRDs", len(afterSnapshot.CRDs))

	// Run all validation checks
	junits := []*junitapi.JUnitTestCase{}

	// Check 1: Verify new API versions don't become storage versions immediately
	junits = append(junits, c.checkNewVersionsNotStoredImmediately()...)

	// Additional checks can be added here in the future
	// Check 2: ... (placeholder for future checks)

	return junits, nil
}

// WriteContentToStorage writes the CRD summary to disk for later inspection.
// This is called regardless of test pass/fail status.
func (c *crdVersionChecker) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	if c.notSupportedReason != nil {
		return nil
	}

	// Only write if we have both snapshots (i.e., an upgrade happened)
	if c.beforeSnapshot == nil || c.afterSnapshot == nil {
		return nil
	}

	// Build the summary
	summary := c.buildSummary()

	// Write to JSON file
	summaryPath := filepath.Join(storageDir, fmt.Sprintf("crd-version-summary%s.json", timeSuffix))
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal CRD summary: %w", err)
	}

	if err := os.WriteFile(summaryPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write CRD summary: %w", err)
	}

	logrus.Infof("Wrote CRD version summary to %s", summaryPath)
	return nil
}

func (c *crdVersionChecker) Cleanup(ctx context.Context) error {
	return nil
}

// collectCRDSnapshot gathers information about all CRDs currently in the cluster.
func (c *crdVersionChecker) collectCRDSnapshot(ctx context.Context) (*CRDSnapshot, error) {
	client, err := apiextensionsclient.NewForConfig(c.adminRESTConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create apiextensions client: %w", err)
	}

	crdList, err := client.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list CRDs: %w", err)
	}

	snapshot := &CRDSnapshot{
		CollectedAt: time.Now(),
		CRDs:        make(map[string]CRDInfo),
	}

	for _, crd := range crdList.Items {
		info := crdInfoFromCRD(&crd)
		snapshot.CRDs[crd.Name] = info
	}

	return snapshot, nil
}

// crdInfoFromCRD extracts the relevant information from a CRD object.
func crdInfoFromCRD(crd *apiextensionsv1.CustomResourceDefinition) CRDInfo {
	versions := make([]CRDVersionInfo, 0, len(crd.Spec.Versions))
	for _, v := range crd.Spec.Versions {
		versions = append(versions, CRDVersionInfo{
			Name:       v.Name,
			Served:     v.Served,
			Storage:    v.Storage,
			Deprecated: v.Deprecated,
		})
	}

	conditions := make([]CRDCondition, 0, len(crd.Status.Conditions))
	for _, c := range crd.Status.Conditions {
		conditions = append(conditions, CRDCondition{
			Type:               string(c.Type),
			Status:             string(c.Status),
			Reason:             c.Reason,
			Message:            c.Message,
			LastTransitionTime: c.LastTransitionTime.Format(time.RFC3339),
		})
	}

	return CRDInfo{
		Name:           crd.Name,
		Group:          crd.Spec.Group,
		Kind:           crd.Spec.Names.Kind,
		Versions:       versions,
		StoredVersions: crd.Status.StoredVersions,
		Conditions:     conditions,
	}
}

// buildSummary computes the differences between before and after snapshots.
func (c *crdVersionChecker) buildSummary() *CRDSummary {
	summary := &CRDSummary{
		BeforeUpgrade: *c.beforeSnapshot,
		AfterUpgrade:  *c.afterSnapshot,
		AddedCRDs:     []string{},
		RemovedCRDs:   []string{},
		ChangedCRDs:   []CRDChangeSummary{},
	}

	// Find added CRDs (in after but not in before)
	for name := range c.afterSnapshot.CRDs {
		if _, exists := c.beforeSnapshot.CRDs[name]; !exists {
			summary.AddedCRDs = append(summary.AddedCRDs, name)
		}
	}
	sort.Strings(summary.AddedCRDs)

	// Find removed CRDs (in before but not in after)
	for name := range c.beforeSnapshot.CRDs {
		if _, exists := c.afterSnapshot.CRDs[name]; !exists {
			summary.RemovedCRDs = append(summary.RemovedCRDs, name)
		}
	}
	sort.Strings(summary.RemovedCRDs)

	// Find changed CRDs (exist in both but have version differences)
	for name, beforeCRD := range c.beforeSnapshot.CRDs {
		afterCRD, exists := c.afterSnapshot.CRDs[name]
		if !exists {
			continue // Already handled as removed
		}

		changeSummary := computeCRDChanges(name, beforeCRD, afterCRD)
		if changeSummary != nil {
			summary.ChangedCRDs = append(summary.ChangedCRDs, *changeSummary)
		}
	}

	// Sort changed CRDs by name for consistent output
	sort.Slice(summary.ChangedCRDs, func(i, j int) bool {
		return summary.ChangedCRDs[i].Name < summary.ChangedCRDs[j].Name
	})

	return summary
}

// computeCRDChanges determines what changed between two versions of a CRD.
func computeCRDChanges(name string, before, after CRDInfo) *CRDChangeSummary {
	beforeVersions := make(map[string]struct{})
	afterVersions := make(map[string]struct{})
	var beforeStorage, afterStorage string

	for _, v := range before.Versions {
		beforeVersions[v.Name] = struct{}{}
		if v.Storage {
			beforeStorage = v.Name
		}
	}

	for _, v := range after.Versions {
		afterVersions[v.Name] = struct{}{}
		if v.Storage {
			afterStorage = v.Name
		}
	}

	// Find added versions
	var addedVersions []string
	for v := range afterVersions {
		if _, ok := beforeVersions[v]; !ok {
			addedVersions = append(addedVersions, v)
		}
	}
	sort.Strings(addedVersions)

	// Find removed versions
	var removedVersions []string
	for v := range beforeVersions {
		if _, ok := afterVersions[v]; !ok {
			removedVersions = append(removedVersions, v)
		}
	}
	sort.Strings(removedVersions)

	// Check if storage version changed
	storageChanged := beforeStorage != afterStorage

	// Only return a change summary if something actually changed
	if len(addedVersions) == 0 && len(removedVersions) == 0 && !storageChanged {
		return nil
	}

	return &CRDChangeSummary{
		Name:            name,
		AddedVersions:   addedVersions,
		RemovedVersions: removedVersions,
		StorageChanged:  storageChanged,
		OldStorage:      beforeStorage,
		NewStorage:      afterStorage,
	}
}

// =============================================================================
// Validation Checks
// =============================================================================

// checkNewVersionsNotStoredImmediately verifies that newly added API versions
// are not immediately set as the storage version.
//
// Rationale: The storage version should remain as the old version for at least
// one release to ensure rollback safety. If the storage version is changed to
// a new version immediately upon upgrade, any objects written to etcd will be
// stored in the new format, and at that point if a rollback is required the old schema
// will not be able to decode these objects, causing issues.
//
// Hence the correct approach is to:
// - Introduce the new version but keep the old version as storage (even carring a patch to the CRD if necessary)
// - Serve both versions so clients can migrate
// - Wait at least one release (reducing rollback risk)
// - In a future release, change storage version and run migration
//
// This check FAILS if:
// - A CRD has a new version added AND that new version is the storage version
//
// This check PASSES if:
// - A new version is added to the CRD but the old version remains the storage version
func (c *crdVersionChecker) checkNewVersionsNotStoredImmediately() []*junitapi.JUnitTestCase {
	const testName = "[sig-api-machinery] CRDs with new API versions should not change storage version immediately"

	if c.beforeSnapshot == nil || c.afterSnapshot == nil {
		// No snapshots available (likely no upgrade occurred), return passing empty test
		return []*junitapi.JUnitTestCase{{Name: testName}}
	}

	var failures []string

	for crdName, beforeCRD := range c.beforeSnapshot.CRDs {
		afterCRD, exists := c.afterSnapshot.CRDs[crdName]
		if !exists {
			// CRD was removed, not relevant for this check
			continue
		}

		// Identify the storage version before and after
		beforeStorage := getStorageVersion(beforeCRD)
		afterStorage := getStorageVersion(afterCRD)

		// Identify newly added versions
		beforeVersionSet := make(map[string]struct{})
		for _, v := range beforeCRD.Versions {
			beforeVersionSet[v.Name] = struct{}{}
		}

		newVersions := []string{}
		for _, v := range afterCRD.Versions {
			if _, ok := beforeVersionSet[v.Name]; !ok {
				newVersions = append(newVersions, v.Name)
			}
		}

		// If no new versions were added, nothing to check
		if len(newVersions) == 0 {
			continue
		}

		// Check if a new version became the storage version
		for _, newVersion := range newVersions {
			if afterStorage == newVersion {
				// New version became storage - this is the violation we're looking for
				// Also verify the old version is still served
				oldVersionStillServed := isVersionServed(afterCRD, beforeStorage)

				failureMsg := fmt.Sprintf(
					"CRD %s: new version %q was immediately set as storage version (was %q). "+
						"New API versions should not become the storage version immediately after introduction. "+
						"Old version %q served: %v",
					crdName, newVersion, beforeStorage, beforeStorage, oldVersionStillServed,
				)
				failures = append(failures, failureMsg)
			}
		}
	}

	if len(failures) > 0 {
		return []*junitapi.JUnitTestCase{{
			Name: testName,
			FailureOutput: &junitapi.FailureOutput{
				Message: fmt.Sprintf("Found %d CRD(s) with new versions immediately set as storage", len(failures)),
				Output:  strings.Join(failures, "\n\n"),
			},
		}}
	}

	return []*junitapi.JUnitTestCase{{Name: testName}}
}

// =============================================================================
// Helper Functions
// =============================================================================

// getStorageVersion returns the storage version name for a CRD.
func getStorageVersion(crd CRDInfo) string {
	for _, v := range crd.Versions {
		if v.Storage {
			return v.Name
		}
	}
	return ""
}

// isVersionServed checks if a specific version is served by the CRD.
func isVersionServed(crd CRDInfo, versionName string) bool {
	for _, v := range crd.Versions {
		if v.Name == versionName {
			return v.Served
		}
	}
	return false
}
