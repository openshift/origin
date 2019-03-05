package management

import (
	"github.com/openshift/api/operator/v1"
)

var (
	allowOperatorUnmanagedState = true
	allowOperatorRemovedState   = true
)

// These are for unit testing
var (
	getAllowedOperatorUnmanaged = func() bool {
		return allowOperatorUnmanagedState
	}
	getAllowedOperatorRemovedState = func() bool {
		return allowOperatorRemovedState
	}
)

// SetOperatorAlwaysManaged is one time choice when an operator want to opt-out from supporting the "unmanaged" state.
// This is a case of control plane operators or operators that are required to always run otherwise the cluster will
// get into unstable state or critical components will stop working.
func SetOperatorAlwaysManaged() {
	allowOperatorUnmanagedState = false
}

// SetOperatorNotRemovable is one time choice the operator author can make to indicate the operator does not support
// removing of his operand. This makes sense for operators like kube-apiserver where removing operand will lead to a
// bricked, non-automatically recoverable state.
func SetOperatorNotRemovable() {
	allowOperatorRemovedState = false
}

// IsOperatorAlwaysManaged means the operator can't be set to unmanaged state.
func IsOperatorAlwaysManaged() bool {
	return !getAllowedOperatorUnmanaged()
}

// IsOperatorNotRemovable means the operator can't bet set to removed state.
func IsOperatorNotRemovable() bool {
	return !getAllowedOperatorRemovedState()
}

func IsOperatorUnknownState(state v1.ManagementState) bool {
	switch state {
	case v1.Managed, v1.Removed, v1.Unmanaged:
		return false
	default:
		return true
	}
}

// IsOperatorManaged indicates whether the operator management state allows the control loop to proceed and manage the operand.
func IsOperatorManaged(state v1.ManagementState) bool {
	if IsOperatorAlwaysManaged() || IsOperatorNotRemovable() {
		return true
	}
	switch state {
	case v1.Managed:
		return true
	case v1.Removed:
		return false
	case v1.Unmanaged:
		return false
	}
	return true
}
