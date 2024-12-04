package common

import (
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	"github.com/openshift/machine-config-operator/pkg/apihelpers"
)

// This is intended to provide a singular way to interrogate MachineConfigPool
// objects to determine if they're in a specific state or not. The eventual
// goal is to use this to mutate the MachineConfigPool object to provide a
// single and consistent interface for that purpose. In this current state, we
// do not perform any mutations.
type LayeredPoolState struct {
	pool *mcfgv1.MachineConfigPool
}

func NewLayeredPoolState(pool *mcfgv1.MachineConfigPool) *LayeredPoolState {
	return &LayeredPoolState{pool: pool}
}

// Returns the OS image, if one is present.
func (l *LayeredPoolState) GetOSImage() string {
	osImage := l.pool.Annotations[ExperimentalNewestLayeredImageEquivalentConfigAnnotationKey]
	return osImage
}

// Determines if a given MachineConfigPool has an available OS image. Returns
// false if the annotation is missing or set to an empty string.
func (l *LayeredPoolState) HasOSImage() bool {
	val, ok := l.pool.Annotations[ExperimentalNewestLayeredImageEquivalentConfigAnnotationKey]
	return ok && val != ""
}

// Determines if an OS image build is a success.
func (l *LayeredPoolState) IsBuildSuccess() bool {
	return apihelpers.IsMachineConfigPoolConditionTrue(l.pool.Status.Conditions, mcfgv1.MachineConfigPoolBuildSuccess)
}

// Determines if an OS image build is pending.
func (l *LayeredPoolState) IsBuildPending() bool {
	return apihelpers.IsMachineConfigPoolConditionTrue(l.pool.Status.Conditions, mcfgv1.MachineConfigPoolBuildPending)
}

// Determines if an OS image build is in progress.
func (l *LayeredPoolState) IsBuilding() bool {
	return apihelpers.IsMachineConfigPoolConditionTrue(l.pool.Status.Conditions, mcfgv1.MachineConfigPoolBuilding)
}

// Determines if an OS image build has failed.
func (l *LayeredPoolState) IsBuildFailure() bool {
	return apihelpers.IsMachineConfigPoolConditionTrue(l.pool.Status.Conditions, mcfgv1.MachineConfigPoolBuildFailed)
}

func (l *LayeredPoolState) IsAnyDegraded() bool {
	condTypes := []mcfgv1.MachineConfigPoolConditionType{
		mcfgv1.MachineConfigPoolDegraded,
		mcfgv1.MachineConfigPoolNodeDegraded,
		mcfgv1.MachineConfigPoolRenderDegraded,
	}

	for _, condType := range condTypes {
		if apihelpers.IsMachineConfigPoolConditionTrue(l.pool.Status.Conditions, condType) {
			return true
		}
	}

	return false
}

func (l *LayeredPoolState) IsInterrupted() bool {
	return apihelpers.IsMachineConfigPoolConditionTrue(l.pool.Status.Conditions, mcfgv1.MachineConfigPoolBuildInterrupted)
}

func (l *LayeredPoolState) IsDegraded() bool {
	return apihelpers.IsMachineConfigPoolConditionTrue(l.pool.Status.Conditions, mcfgv1.MachineConfigPoolDegraded)
}

func (l *LayeredPoolState) IsNodeDegraded() bool {
	return apihelpers.IsMachineConfigPoolConditionTrue(l.pool.Status.Conditions, mcfgv1.MachineConfigPoolNodeDegraded)
}

func (l *LayeredPoolState) IsRenderDegraded() bool {
	return apihelpers.IsMachineConfigPoolConditionTrue(l.pool.Status.Conditions, mcfgv1.MachineConfigPoolRenderDegraded)
}
