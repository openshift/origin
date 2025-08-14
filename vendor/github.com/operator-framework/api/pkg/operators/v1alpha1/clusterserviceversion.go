package v1alpha1

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
)

const (
	CopiedLabelKey = "olm.copiedFrom"

	// ConditionsLengthLimit is the maximum length of Status.Conditions of a
	// given ClusterServiceVersion object. The oldest condition(s) are removed
	// from the list as it grows over time to keep it at limit.
	ConditionsLengthLimit = 20
)

// obsoleteReasons are the set of reasons that mean a CSV should no longer be processed as active
var obsoleteReasons = map[ConditionReason]struct{}{
	CSVReasonReplaced:      {},
	CSVReasonBeingReplaced: {},
}

// uncopiableReasons are the set of reasons that should prevent a CSV from being copied to target namespaces
var uncopiableReasons = map[ConditionReason]struct{}{
	CSVReasonCopied:                                      {},
	CSVReasonInvalidInstallModes:                         {},
	CSVReasonNoTargetNamespaces:                          {},
	CSVReasonUnsupportedOperatorGroup:                    {},
	CSVReasonNoOperatorGroup:                             {},
	CSVReasonTooManyOperatorGroups:                       {},
	CSVReasonInterOperatorGroupOwnerConflict:             {},
	CSVReasonCannotModifyStaticOperatorGroupProvidedAPIs: {},
}

// safeToAnnotateOperatorGroupReasons are the set of reasons that it's safe to attempt to update the operatorgroup
// annotations
var safeToAnnotateOperatorGroupReasons = map[ConditionReason]struct{}{
	CSVReasonOwnerConflict:                               {},
	CSVReasonInstallSuccessful:                           {},
	CSVReasonInvalidInstallModes:                         {},
	CSVReasonNoTargetNamespaces:                          {},
	CSVReasonUnsupportedOperatorGroup:                    {},
	CSVReasonNoOperatorGroup:                             {},
	CSVReasonTooManyOperatorGroups:                       {},
	CSVReasonInterOperatorGroupOwnerConflict:             {},
	CSVReasonCannotModifyStaticOperatorGroupProvidedAPIs: {},
}

// SetPhaseWithEventIfChanged emits a Kubernetes event with details of a phase change and sets the current phase if phase, reason, or message would changed
func (c *ClusterServiceVersion) SetPhaseWithEventIfChanged(phase ClusterServiceVersionPhase, reason ConditionReason, message string, now *metav1.Time, recorder record.EventRecorder) {
	if c.Status.Phase == phase && c.Status.Reason == reason && c.Status.Message == message {
		return
	}

	c.SetPhaseWithEvent(phase, reason, message, now, recorder)
}

// SetPhaseWithEvent generates a Kubernetes event with details about the phase change and sets the current phase
func (c *ClusterServiceVersion) SetPhaseWithEvent(phase ClusterServiceVersionPhase, reason ConditionReason, message string, now *metav1.Time, recorder record.EventRecorder) {
	var eventtype string
	if phase == CSVPhaseFailed {
		eventtype = v1.EventTypeWarning
	} else {
		eventtype = v1.EventTypeNormal
	}
	go recorder.Event(c, eventtype, string(reason), message)
	c.SetPhase(phase, reason, message, now)
}

// SetPhase sets the current phase and adds a condition if necessary
func (c *ClusterServiceVersion) SetPhase(phase ClusterServiceVersionPhase, reason ConditionReason, message string, now *metav1.Time) {
	newCondition := func() ClusterServiceVersionCondition {
		return ClusterServiceVersionCondition{
			Phase:              c.Status.Phase,
			LastTransitionTime: c.Status.LastTransitionTime,
			LastUpdateTime:     c.Status.LastUpdateTime,
			Message:            message,
			Reason:             reason,
		}
	}

	defer c.TrimConditionsIfLimitExceeded()

	c.Status.LastUpdateTime = now
	if c.Status.Phase != phase {
		c.Status.Phase = phase
		c.Status.LastTransitionTime = now
	}
	c.Status.Message = message
	c.Status.Reason = reason
	if len(c.Status.Conditions) == 0 {
		c.Status.Conditions = append(c.Status.Conditions, newCondition())
		return
	}

	previousCondition := c.Status.Conditions[len(c.Status.Conditions)-1]
	if previousCondition.Phase != c.Status.Phase || previousCondition.Reason != c.Status.Reason {
		c.Status.Conditions = append(c.Status.Conditions, newCondition())
	}
}

// SetRequirementStatus adds the status of all requirements to the CSV status
func (c *ClusterServiceVersion) SetRequirementStatus(statuses []RequirementStatus) {
	c.Status.RequirementStatus = statuses
}

// IsObsolete returns if this CSV is being replaced or is marked for deletion
func (c *ClusterServiceVersion) IsObsolete() bool {
	for _, condition := range c.Status.Conditions {
		_, ok := obsoleteReasons[condition.Reason]
		if ok {
			return true
		}
	}
	return false
}

// IsCopied returns true if the CSV has been copied and false otherwise.
func (c *ClusterServiceVersion) IsCopied() bool {
	return c.Status.Reason == CSVReasonCopied || IsCopied(c)
}

func IsCopied(o metav1.Object) bool {
	annotations := o.GetAnnotations()
	if annotations != nil {
		operatorNamespace, ok := annotations[OperatorGroupNamespaceAnnotationKey]
		if ok && o.GetNamespace() != operatorNamespace {
			return true
		}
	}

	if labels := o.GetLabels(); labels != nil {
		if _, ok := labels[CopiedLabelKey]; ok {
			return true
		}
	}
	return false
}

func (c *ClusterServiceVersion) IsUncopiable() bool {
	if c.Status.Phase == CSVPhaseNone {
		return true
	}
	_, ok := uncopiableReasons[c.Status.Reason]
	return ok
}

func (c *ClusterServiceVersion) IsSafeToUpdateOperatorGroupAnnotations() bool {
	_, ok := safeToAnnotateOperatorGroupReasons[c.Status.Reason]
	return ok
}

// NewInstallModeSet returns an InstallModeSet instantiated from the given list of InstallModes.
// If the given list is not a set, an error is returned.
func NewInstallModeSet(modes []InstallMode) (InstallModeSet, error) {
	set := InstallModeSet{}
	for _, mode := range modes {
		if _, exists := set[mode.Type]; exists {
			return nil, fmt.Errorf("InstallMode list contains duplicates, cannot make set: %v", modes)
		}
		set[mode.Type] = mode.Supported
	}

	return set, nil
}

// Supports returns an error if the InstallModeSet does not support configuration for
// the given operatorNamespace and list of target namespaces.
func (set InstallModeSet) Supports(operatorNamespace string, namespaces []string) error {
	numNamespaces := len(namespaces)
	switch {
	case numNamespaces == 0:
		return fmt.Errorf("operatorgroup has invalid selected namespaces, cannot configure to watch zero namespaces")
	case numNamespaces == 1:
		switch namespaces[0] {
		case operatorNamespace:
			if !set[InstallModeTypeOwnNamespace] {
				return fmt.Errorf("%s InstallModeType not supported, cannot configure to watch own namespace", InstallModeTypeOwnNamespace)
			}
		case v1.NamespaceAll:
			if !set[InstallModeTypeAllNamespaces] {
				return fmt.Errorf("%s InstallModeType not supported, cannot configure to watch all namespaces", InstallModeTypeAllNamespaces)
			}
		default:
			if !set[InstallModeTypeSingleNamespace] {
				return fmt.Errorf("%s InstallModeType not supported, cannot configure to watch one namespace", InstallModeTypeSingleNamespace)
			}
		}
	case numNamespaces > 1 && !set[InstallModeTypeMultiNamespace]:
		return fmt.Errorf("%s InstallModeType not supported, cannot configure to watch %d namespaces", InstallModeTypeMultiNamespace, numNamespaces)
	case numNamespaces > 1:
		for _, namespace := range namespaces {
			if namespace == operatorNamespace && !set[InstallModeTypeOwnNamespace] {
				return fmt.Errorf("%s InstallModeType not supported, cannot configure to watch own namespace", InstallModeTypeOwnNamespace)
			}
			if namespace == v1.NamespaceAll {
				return fmt.Errorf("operatorgroup has invalid selected namespaces, NamespaceAll found when |selected namespaces| > 1")
			}
		}
	}

	return nil
}

func (c *ClusterServiceVersion) TrimConditionsIfLimitExceeded() {
	if len(c.Status.Conditions) <= ConditionsLengthLimit {
		return
	}

	firstIndex := len(c.Status.Conditions) - ConditionsLengthLimit
	c.Status.Conditions = c.Status.Conditions[firstIndex:len(c.Status.Conditions)]
}
