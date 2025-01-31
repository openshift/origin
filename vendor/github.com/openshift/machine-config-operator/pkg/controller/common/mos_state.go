package common

import (
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	mcfgv1alpha1 "github.com/openshift/api/machineconfiguration/v1alpha1"
	"github.com/openshift/machine-config-operator/pkg/apihelpers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// This is intended to provide a singular way to interrogate MachineConfigPool
// objects to determine if they're in a specific state or not. The eventual
// goal is to use this to mutate the MachineConfigPool object to provide a
// single and consistent interface for that purpose. In this current state, we
// do not perform any mutations.
type MachineOSBuildState struct {
	Build *mcfgv1alpha1.MachineOSBuild
}

type MachineOSConfigState struct {
	Config *mcfgv1alpha1.MachineOSConfig
}

func NewMachineOSConfigState(mosc *mcfgv1alpha1.MachineOSConfig) *MachineOSConfigState {
	return &MachineOSConfigState{
		Config: mosc,
	}
}

func NewMachineOSBuildState(mosb *mcfgv1alpha1.MachineOSBuild) *MachineOSBuildState {
	return &MachineOSBuildState{
		Build: mosb,
	}
}

// Returns the OS image, if one is present.
func (c *MachineOSConfigState) GetOSImage() string {
	osImage := c.Config.Status.CurrentImagePullspec
	return osImage
}

// Determines if a given MachineConfigPool has an available OS image. Returns
// false if the annotation is missing or set to an empty string.
func (c *MachineOSConfigState) HasOSImage() bool {
	val := c.Config.Status.CurrentImagePullspec
	return val != ""
}

// Clears the image pullspec annotation.
func (c *MachineOSConfigState) ClearImagePullspec() {
	c.Config.Spec.BuildInputs.RenderedImagePushspec = ""
	c.Config.Status.CurrentImagePullspec = ""
}

// Clears all build object conditions.
func (b *MachineOSBuildState) ClearAllBuildConditions() {
	b.Build.Status.Conditions = clearAllBuildConditions(b.Build.Status.Conditions)
}

// Determines if an OS image build is a success.
func (b *MachineOSBuildState) IsBuildSuccess() bool {
	return apihelpers.IsMachineOSBuildConditionTrue(b.Build.Status.Conditions, mcfgv1alpha1.MachineOSBuildSucceeded)
}

// Determines if an OS image build is pending.
func (b *MachineOSBuildState) IsBuildPending() bool {
	return apihelpers.IsMachineOSBuildConditionTrue(b.Build.Status.Conditions, mcfgv1alpha1.MachineOSBuilding)
}

// Determines if an OS image build is in progress.
func (b *MachineOSBuildState) IsBuilding() bool {
	return apihelpers.IsMachineOSBuildConditionTrue(b.Build.Status.Conditions, mcfgv1alpha1.MachineOSBuilding)
}

// Determines if an OS image build has failed.
func (b *MachineOSBuildState) IsBuildFailure() bool {
	return apihelpers.IsMachineOSBuildConditionTrue(b.Build.Status.Conditions, mcfgv1alpha1.MachineOSBuildFailed)
}

// Determines if an OS image build has failed.
func (b *MachineOSBuildState) IsBuildInterrupted() bool {
	return apihelpers.IsMachineOSBuildConditionTrue(b.Build.Status.Conditions, mcfgv1alpha1.MachineOSBuildInterrupted)
}

func (b *MachineOSBuildState) IsAnyDegraded() bool {
	condTypes := []mcfgv1alpha1.BuildProgress{
		mcfgv1alpha1.MachineOSBuildFailed,
		mcfgv1alpha1.MachineOSBuildInterrupted,
	}

	for _, condType := range condTypes {
		if apihelpers.IsMachineOSBuildConditionTrue(b.Build.Status.Conditions, condType) {
			return true
		}
	}

	return false
}

// Idempotently sets the supplied build conditions.
func (b *MachineOSBuildState) SetBuildConditions(conditions []metav1.Condition) {
	for _, condition := range conditions {
		condition := condition
		currentCondition := apihelpers.GetMachineOSBuildCondition(b.Build.Status, mcfgv1alpha1.BuildProgress(condition.Type))
		if currentCondition != nil && isConditionEqual(*currentCondition, condition) {
			continue
		}

		mosbCondition := apihelpers.NewMachineOSBuildCondition(condition.Type, condition.Status, condition.Reason, condition.Message)
		apihelpers.SetMachineOSBuildCondition(&b.Build.Status, *mosbCondition)
	}
}

// Determines if two conditions are equal. Note: I purposely do not include the
// timestamp in the equality test, since we do not directly set it.
func isConditionEqual(cond1, cond2 metav1.Condition) bool {
	return cond1.Type == cond2.Type &&
		cond1.Status == cond2.Status &&
		cond1.Message == cond2.Message &&
		cond1.Reason == cond2.Reason
}

func clearAllBuildConditions(inConditions []metav1.Condition) []metav1.Condition {
	conditions := []metav1.Condition{}

	for _, condition := range inConditions {
		buildConditionFound := false
		for _, buildConditionType := range getMachineConfigBuildConditions() {
			if condition.Type == string(buildConditionType) {
				buildConditionFound = true
				break
			}
		}

		if !buildConditionFound {
			conditions = append(conditions, condition)
		}
	}

	return conditions
}

func getMachineConfigBuildConditions() []mcfgv1alpha1.BuildProgress {
	return []mcfgv1alpha1.BuildProgress{
		mcfgv1alpha1.MachineOSBuildFailed,
		mcfgv1alpha1.MachineOSBuildInterrupted,
		mcfgv1alpha1.MachineOSBuildPrepared,
		mcfgv1alpha1.MachineOSBuildSucceeded,
		mcfgv1alpha1.MachineOSBuilding,
	}
}

func IsPoolAnyDegraded(pool *mcfgv1.MachineConfigPool) bool {
	condTypes := []mcfgv1.MachineConfigPoolConditionType{
		mcfgv1.MachineConfigPoolDegraded,
		mcfgv1.MachineConfigPoolNodeDegraded,
		mcfgv1.MachineConfigPoolRenderDegraded,
	}

	for _, condType := range condTypes {
		if apihelpers.IsMachineConfigPoolConditionTrue(pool.Status.Conditions, condType) {
			return true
		}
	}

	return false
}

// Determine if we have a config change.
func IsPoolConfigChange(oldPool, curPool *mcfgv1.MachineConfigPool) bool {
	return oldPool.Spec.Configuration.Name != curPool.Spec.Configuration.Name
}

func HasBuildObjectForCurrentMachineConfig(pool *mcfgv1.MachineConfigPool, mosb *mcfgv1alpha1.MachineOSBuild) bool {
	return pool.Spec.Configuration.Name == mosb.Spec.DesiredConfig.Name
}

// Determines if we should do a build based upon the state of our
// MachineConfigPool, the presence of a build pod, etc.
func BuildDueToPoolChange(oldPool, curPool *mcfgv1.MachineConfigPool, moscNew *mcfgv1alpha1.MachineOSConfig, mosbNew *mcfgv1alpha1.MachineOSBuild) bool {

	moscState := NewMachineOSConfigState(moscNew)
	mosbState := NewMachineOSBuildState(mosbNew)

	// If we don't have a layered pool, we should not build.
	poolStateSuggestsBuild := canPoolBuild(curPool, moscState, mosbState) &&
		// If we have a config change or we're missing an image pullspec label, we
		// should do a build.
		(IsPoolConfigChange(oldPool, curPool) || !moscState.HasOSImage())

	return poolStateSuggestsBuild

}

// Checks our pool to see if we can do a build. We base this off of a few criteria:
// 1. Is the pool opted into layering?
// 2. Do we have an object reference to an in-progress build?
// 3. Is the pool degraded?
// 4. Is our build in a specific state?
//
// Returns true if we are able to build.
func canPoolBuild(pool *mcfgv1.MachineConfigPool, moscNewState *MachineOSConfigState, mosbNewState *MachineOSBuildState) bool {
	// If we don't have a layered pool, we should not build.
	if !IsLayeredPool(moscNewState.Config, mosbNewState.Build) {
		return false
	}
	// If the pool is degraded, we should not build.
	if IsPoolAnyDegraded(pool) {
		return false
	}
	// If the new pool has an ongoing build, we should not build
	if mosbNewState.Build != nil {
		return false
	}
	return true
}

func IsLayeredPool(mosc *mcfgv1alpha1.MachineOSConfig, mosb *mcfgv1alpha1.MachineOSBuild) bool {
	return (mosc != nil || mosb != nil)
}
