package apihelpers

// TODO(jkyros): This is here in its own package because it was with the API, but when we migrated the API, it couldn't go with
// it because it was only used in the MCO. I wanted to stuff it in common, but because of how our test suite is set up, that would
// have caused a dependency cycle, so now it's here by itself.

import (
	"fmt"

	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	opv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/machine-config-operator/pkg/daemon/constants"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
)

var (
	// This is the list of MCO's default node disruption policies.
	defaultClusterPolicies = opv1.NodeDisruptionPolicyClusterStatus{
		Files: []opv1.NodeDisruptionPolicyStatusFile{
			{
				Path: constants.KubeletAuthFile,
				Actions: []opv1.NodeDisruptionPolicyStatusAction{
					{
						Type: opv1.NoneStatusAction,
					},
				},
			},
			{
				Path: constants.GPGNoRebootPath,
				Actions: []opv1.NodeDisruptionPolicyStatusAction{
					{
						Type: opv1.ReloadStatusAction,
						Reload: &opv1.ReloadService{
							ServiceName: "crio.service",
						},
					},
				},
			},
			{
				Path: constants.ContainerRegistryPolicyPath,
				Actions: []opv1.NodeDisruptionPolicyStatusAction{
					{
						Type: opv1.ReloadStatusAction,
						Reload: &opv1.ReloadService{
							ServiceName: "crio.service",
						},
					},
				},
			},
			{
				Path: constants.ContainerRegistryConfPath,
				Actions: []opv1.NodeDisruptionPolicyStatusAction{
					{
						Type: opv1.SpecialStatusAction,
					},
				},
			},
			{
				Path: constants.SigstoreRegistriesConfigDir,
				Actions: []opv1.NodeDisruptionPolicyStatusAction{
					{
						Type: opv1.ReloadStatusAction,
						Reload: &opv1.ReloadService{
							ServiceName: "crio.service",
						},
					},
				},
			},
			{
				Path: constants.CrioPoliciesDir,
				Actions: []opv1.NodeDisruptionPolicyStatusAction{
					{
						Type: opv1.ReloadStatusAction,
						Reload: &opv1.ReloadService{
							ServiceName: "crio.service",
						},
					},
				},
			},
			{
				Path: constants.OpenShiftNMStateConfigDir,
				Actions: []opv1.NodeDisruptionPolicyStatusAction{
					{
						Type: opv1.NoneStatusAction,
					},
				},
			},
			{
				Path: constants.UserCABundlePath,
				Actions: []opv1.NodeDisruptionPolicyStatusAction{
					{
						Type: opv1.RestartStatusAction,
						Restart: &opv1.RestartService{
							ServiceName: constants.UpdateCATrustServiceName,
						},
					},
					{
						Type: opv1.RestartStatusAction,
						Restart: &opv1.RestartService{
							ServiceName: "crio.service",
						},
					},
				},
			},
		},
		SSHKey: opv1.NodeDisruptionPolicyStatusSSHKey{
			Actions: []opv1.NodeDisruptionPolicyStatusAction{
				{
					Type: opv1.NoneStatusAction,
				},
			},
		},
	}
)

// NewMachineConfigPoolCondition creates a new MachineConfigPool condition.
func NewMachineConfigPoolCondition(condType mcfgv1.MachineConfigPoolConditionType, status corev1.ConditionStatus, reason, message string) *mcfgv1.MachineConfigPoolCondition {
	return &mcfgv1.MachineConfigPoolCondition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

// GetMachineConfigPoolCondition returns the condition with the provided type.
func GetMachineConfigPoolCondition(status mcfgv1.MachineConfigPoolStatus, condType mcfgv1.MachineConfigPoolConditionType) *mcfgv1.MachineConfigPoolCondition {
	// in case of sync errors, return the last condition that matches, not the first
	// this exists for redundancy and potential race conditions.
	var LatestState *mcfgv1.MachineConfigPoolCondition
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			LatestState = &c
		}
	}
	return LatestState
}

// SetMachineConfigPoolCondition updates the MachineConfigPool to include the provided condition. If the condition that
// we are about to add already exists and has the same status and reason then we are not going to update.
func SetMachineConfigPoolCondition(status *mcfgv1.MachineConfigPoolStatus, condition mcfgv1.MachineConfigPoolCondition) {
	currentCond := GetMachineConfigPoolCondition(*status, condition.Type)
	if currentCond != nil && currentCond.Status == condition.Status && currentCond.Reason == condition.Reason && currentCond.Message == condition.Message {
		return
	}
	// Do not update lastTransitionTime if the status of the condition doesn't change.
	if currentCond != nil && currentCond.Status == condition.Status {
		condition.LastTransitionTime = currentCond.LastTransitionTime
	}
	newConditions := filterOutMachineConfigPoolCondition(status.Conditions, condition.Type)
	newConditions = append(newConditions, condition)
	status.Conditions = newConditions

}

// RemoveMachineConfigPoolCondition removes the MachineConfigPool condition with the provided type.
func RemoveMachineConfigPoolCondition(status *mcfgv1.MachineConfigPoolStatus, condType mcfgv1.MachineConfigPoolConditionType) {
	status.Conditions = filterOutMachineConfigPoolCondition(status.Conditions, condType)
}

// filterOutCondition returns a new slice of MachineConfigPool conditions without conditions with the provided type.
func filterOutMachineConfigPoolCondition(conditions []mcfgv1.MachineConfigPoolCondition, condType mcfgv1.MachineConfigPoolConditionType) []mcfgv1.MachineConfigPoolCondition {
	var newConditions []mcfgv1.MachineConfigPoolCondition
	for _, c := range conditions {
		if c.Type == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}

// IsMachineConfigPoolConditionTrue returns true when the conditionType is present and set to `ConditionTrue`
func IsMachineConfigPoolConditionTrue(conditions []mcfgv1.MachineConfigPoolCondition, conditionType mcfgv1.MachineConfigPoolConditionType) bool {
	return IsMachineConfigPoolConditionPresentAndEqual(conditions, conditionType, corev1.ConditionTrue)
}

// IsMachineConfigPoolConditionFalse returns true when the conditionType is present and set to `ConditionFalse`
func IsMachineConfigPoolConditionFalse(conditions []mcfgv1.MachineConfigPoolCondition, conditionType mcfgv1.MachineConfigPoolConditionType) bool {
	return IsMachineConfigPoolConditionPresentAndEqual(conditions, conditionType, corev1.ConditionFalse)
}

// IsMachineConfigPoolConditionPresentAndEqual returns true when conditionType is present and equal to status.
func IsMachineConfigPoolConditionPresentAndEqual(conditions []mcfgv1.MachineConfigPoolCondition, conditionType mcfgv1.MachineConfigPoolConditionType, status corev1.ConditionStatus) bool {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return condition.Status == status
		}
	}
	return false
}

// NewKubeletConfigCondition returns an instance of a KubeletConfigCondition
func NewKubeletConfigCondition(condType mcfgv1.KubeletConfigStatusConditionType, status corev1.ConditionStatus, message string) *mcfgv1.KubeletConfigCondition {
	return &mcfgv1.KubeletConfigCondition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Message:            message,
	}
}

func NewCondition(condType string, status metav1.ConditionStatus, reason, message string) *metav1.Condition {
	return &metav1.Condition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

// NewContainerRuntimeConfigCondition returns an instance of a ContainerRuntimeConfigCondition
func NewContainerRuntimeConfigCondition(condType mcfgv1.ContainerRuntimeConfigStatusConditionType, status corev1.ConditionStatus, message string) *mcfgv1.ContainerRuntimeConfigCondition {
	return &mcfgv1.ContainerRuntimeConfigCondition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Message:            message,
	}
}

// NewControllerConfigStatusCondition creates a new ControllerConfigStatus condition.
func NewControllerConfigStatusCondition(condType mcfgv1.ControllerConfigStatusConditionType, status corev1.ConditionStatus, reason, message string) *mcfgv1.ControllerConfigStatusCondition {
	return &mcfgv1.ControllerConfigStatusCondition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

// GetControllerConfigStatusCondition returns the condition with the provided type.
func GetControllerConfigStatusCondition(status mcfgv1.ControllerConfigStatus, condType mcfgv1.ControllerConfigStatusConditionType) *mcfgv1.ControllerConfigStatusCondition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}

// SetControllerConfigStatusCondition updates the ControllerConfigStatus to include the provided condition. If the condition that
// we are about to add already exists and has the same status and reason then we are not going to update.
func SetControllerConfigStatusCondition(status *mcfgv1.ControllerConfigStatus, condition mcfgv1.ControllerConfigStatusCondition) {
	currentCond := GetControllerConfigStatusCondition(*status, condition.Type)
	if currentCond != nil && currentCond.Status == condition.Status && currentCond.Reason == condition.Reason {
		return
	}
	// Do not update lastTransitionTime if the status of the condition doesn't change.
	if currentCond != nil && currentCond.Status == condition.Status {
		condition.LastTransitionTime = currentCond.LastTransitionTime
	}
	newConditions := filterOutControllerConfigStatusCondition(status.Conditions, condition.Type)
	newConditions = append(newConditions, condition)
	status.Conditions = newConditions
}

// RemoveControllerConfigStatusCondition removes the ControllerConfigStatus condition with the provided type.
func RemoveControllerConfigStatusCondition(status *mcfgv1.ControllerConfigStatus, condType mcfgv1.ControllerConfigStatusConditionType) {
	status.Conditions = filterOutControllerConfigStatusCondition(status.Conditions, condType)
}

// filterOutCondition returns a new slice of ControllerConfigStatus conditions without conditions with the provided type.
func filterOutControllerConfigStatusCondition(conditions []mcfgv1.ControllerConfigStatusCondition, condType mcfgv1.ControllerConfigStatusConditionType) []mcfgv1.ControllerConfigStatusCondition {
	var newConditions []mcfgv1.ControllerConfigStatusCondition
	for _, c := range conditions {
		if c.Type == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}

// IsControllerConfigStatusConditionTrue returns true when the conditionType is present and set to `ConditionTrue`
func IsControllerConfigStatusConditionTrue(conditions []mcfgv1.ControllerConfigStatusCondition, conditionType mcfgv1.ControllerConfigStatusConditionType) bool {
	return IsControllerConfigStatusConditionPresentAndEqual(conditions, conditionType, corev1.ConditionTrue)
}

// IsControllerConfigStatusConditionFalse returns true when the conditionType is present and set to `ConditionFalse`
func IsControllerConfigStatusConditionFalse(conditions []mcfgv1.ControllerConfigStatusCondition, conditionType mcfgv1.ControllerConfigStatusConditionType) bool {
	return IsControllerConfigStatusConditionPresentAndEqual(conditions, conditionType, corev1.ConditionFalse)
}

// IsControllerConfigStatusConditionPresentAndEqual returns true when conditionType is present and equal to status.
func IsControllerConfigStatusConditionPresentAndEqual(conditions []mcfgv1.ControllerConfigStatusCondition, conditionType mcfgv1.ControllerConfigStatusConditionType, status corev1.ConditionStatus) bool {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return condition.Status == status
		}
	}
	return false
}

// IsControllerConfigCompleted checks whether a ControllerConfig is completed by the Template Controller
func IsControllerConfigCompleted(ccName string, ccGetter func(string) (*mcfgv1.ControllerConfig, error)) error {
	cur, err := ccGetter(ccName)
	if err != nil {
		return err
	}

	if cur.Generation != cur.Status.ObservedGeneration {
		return fmt.Errorf("status for ControllerConfig %s is being reported for %d, expecting it for %d", ccName, cur.Status.ObservedGeneration, cur.Generation)
	}

	completed := IsControllerConfigStatusConditionTrue(cur.Status.Conditions, mcfgv1.TemplateControllerCompleted)
	running := IsControllerConfigStatusConditionTrue(cur.Status.Conditions, mcfgv1.TemplateControllerRunning)
	failing := IsControllerConfigStatusConditionTrue(cur.Status.Conditions, mcfgv1.TemplateControllerFailing)
	if completed &&
		!running &&
		!failing {
		return nil
	}
	return fmt.Errorf("ControllerConfig has not completed: completed(%v) running(%v) failing(%v)", completed, running, failing)
}

// Merges the cluster's default node disruption policies with the user defined policies, if any.
func MergeClusterPolicies(userDefinedClusterPolicies opv1.NodeDisruptionPolicyConfig) opv1.NodeDisruptionPolicyClusterStatus {

	mergedClusterPolicies := opv1.NodeDisruptionPolicyClusterStatus{}

	// Add default file policies to the merged list.
	mergedClusterPolicies.Files = append(mergedClusterPolicies.Files, defaultClusterPolicies.Files...)

	// Iterate through user file policies.
	// If there is a conflict with default policy, replace that entry in the merged list with the user defined policy.
	// If there was no conflict, add the user defined policy as a new entry to the merged list.
	for _, userDefinedPolicyFile := range userDefinedClusterPolicies.Files {
		override := false
		for i, defaultPolicyFile := range defaultClusterPolicies.Files {
			if defaultPolicyFile.Path == userDefinedPolicyFile.Path {
				mergedClusterPolicies.Files[i] = convertSpecFileToStatusFile(userDefinedPolicyFile)
				override = true
				break
			}
		}
		if !override {
			mergedClusterPolicies.Files = append(mergedClusterPolicies.Files, convertSpecFileToStatusFile(userDefinedPolicyFile))
		}
	}

	// Add default service unit policies to the merged list.
	mergedClusterPolicies.Units = append(mergedClusterPolicies.Units, defaultClusterPolicies.Units...)

	// Iterate through user service unit policies.
	// If there is a conflict with default policy, replace that entry in the merged list with the user defined policy.
	// If there was no conflict, add the user defined policy as a new entry to the merged list.
	for _, userDefinedPolicyUnit := range userDefinedClusterPolicies.Units {
		override := false
		for i, defaultPolicyUnit := range defaultClusterPolicies.Units {
			if defaultPolicyUnit.Name == userDefinedPolicyUnit.Name {
				mergedClusterPolicies.Units[i] = convertSpecUnitToStatusUnit(userDefinedPolicyUnit)
				override = true
				break
			}
		}
		if !override {
			mergedClusterPolicies.Units = append(mergedClusterPolicies.Units, convertSpecUnitToStatusUnit(userDefinedPolicyUnit))
		}
	}

	// If no user defined SSH policy exists, use the cluster defaults.
	if len(userDefinedClusterPolicies.SSHKey.Actions) == 0 {
		mergedClusterPolicies.SSHKey = *defaultClusterPolicies.SSHKey.DeepCopy()
	} else {
		mergedClusterPolicies.SSHKey = convertSpecSSHKeyToStatusSSHKey(*userDefinedClusterPolicies.SSHKey.DeepCopy())
	}
	return mergedClusterPolicies
}

// converts NodeDisruptionPolicySpecFile -> NodeDisruptionPolicyStatusFile
func convertSpecFileToStatusFile(specFile opv1.NodeDisruptionPolicySpecFile) opv1.NodeDisruptionPolicyStatusFile {
	statusFile := opv1.NodeDisruptionPolicyStatusFile{Path: specFile.Path, Actions: []opv1.NodeDisruptionPolicyStatusAction{}}
	for _, action := range specFile.Actions {
		statusFile.Actions = append(statusFile.Actions, convertSpecActiontoStatusAction(action))
	}
	return statusFile
}

// converts NodeDisruptionPolicySpecUnit -> NodeDisruptionPolicyStatusUnit
func convertSpecUnitToStatusUnit(specUnit opv1.NodeDisruptionPolicySpecUnit) opv1.NodeDisruptionPolicyStatusUnit {
	statusUnit := opv1.NodeDisruptionPolicyStatusUnit{Name: specUnit.Name, Actions: []opv1.NodeDisruptionPolicyStatusAction{}}
	for _, action := range specUnit.Actions {
		statusUnit.Actions = append(statusUnit.Actions, convertSpecActiontoStatusAction(action))
	}
	return statusUnit
}

// converts NodeDisruptionPolicySpecSSHKey -> NodeDisruptionPolicyStatusSSHKey
func convertSpecSSHKeyToStatusSSHKey(specSSHKey opv1.NodeDisruptionPolicySpecSSHKey) opv1.NodeDisruptionPolicyStatusSSHKey {
	statusSSHKey := opv1.NodeDisruptionPolicyStatusSSHKey{Actions: []opv1.NodeDisruptionPolicyStatusAction{}}
	for _, action := range specSSHKey.Actions {
		statusSSHKey.Actions = append(statusSSHKey.Actions, convertSpecActiontoStatusAction(action))
	}
	return statusSSHKey
}

// converts NodeDisruptionPolicySpecAction -> NodeDisruptionPolicyStatusAction
func convertSpecActiontoStatusAction(action opv1.NodeDisruptionPolicySpecAction) opv1.NodeDisruptionPolicyStatusAction {
	switch action.Type {
	case opv1.DaemonReloadSpecAction:
		return opv1.NodeDisruptionPolicyStatusAction{Type: opv1.DaemonReloadStatusAction}
	case opv1.DrainSpecAction:
		return opv1.NodeDisruptionPolicyStatusAction{Type: opv1.DrainStatusAction}
	case opv1.NoneSpecAction:
		return opv1.NodeDisruptionPolicyStatusAction{Type: opv1.NoneStatusAction}
	case opv1.RebootSpecAction:
		return opv1.NodeDisruptionPolicyStatusAction{Type: opv1.RebootStatusAction}
	case opv1.ReloadSpecAction:
		return opv1.NodeDisruptionPolicyStatusAction{Type: opv1.ReloadStatusAction, Reload: &opv1.ReloadService{
			ServiceName: action.Reload.ServiceName,
		}}
	case opv1.RestartSpecAction:
		return opv1.NodeDisruptionPolicyStatusAction{Type: opv1.RestartStatusAction, Restart: &opv1.RestartService{
			ServiceName: action.Restart.ServiceName,
		}}
	default: // We should never be here as this is guarded by API validation. The return statement is to silence errors.
		klog.Fatal("Unexpected action type found in Node Disruption Status calculation")
		return opv1.NodeDisruptionPolicyStatusAction{Type: opv1.RebootStatusAction}
	}
}

// Checks if a list of NodeDisruptionActions contain any action from the set of target actions
func CheckNodeDisruptionActionsForTargetActions(actions []opv1.NodeDisruptionPolicyStatusAction, targetActions ...opv1.NodeDisruptionPolicyStatusActionType) bool {

	currentActions := sets.New[opv1.NodeDisruptionPolicyStatusActionType]()
	for _, action := range actions {
		currentActions.Insert(action.Type)
	}

	return currentActions.HasAny(targetActions...)
}
