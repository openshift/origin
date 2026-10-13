package management

import (
	v1 "github.com/openshift/api/operator/v1"
)

var (
	allowOperatorUnmanagedState = true
	allowOperatorRemovedState   = true
)

// SetOperatorAlwaysManaged is one time choice when an operator want to opt-out from supporting the "unmanaged" state.
// This is a case of control plane operators or operators that are required to always run otherwise the cluster will
// get into unstable state or critical components will stop working.
func SetOperatorAlwaysManaged() {
	allowOperatorUnmanagedState = false
}

// SetOperatorUnmanageable is one time choice when an operator wants to support the "unmanaged" state.
// This is the default setting, provided here mostly for unit tests.
func SetOperatorUnmanageable() {
	allowOperatorUnmanagedState = true
}

// SetOperatorNotRemovable is one time choice the operator author can make to indicate the operator does not support
// removing of his operand. This makes sense for operators like kube-apiserver where removing operand will lead to a
// bricked, non-automatically recoverable state.
func SetOperatorNotRemovable() {
	allowOperatorRemovedState = false
}

// SetOperatorRemovable is one time choice the operator author can make to indicate the operator supports
// removing of his operand.
// This is the default setting, provided here mostly for unit tests.
func SetOperatorRemovable() {
	allowOperatorRemovedState = true
}

// IsOperatorAlwaysManaged means the operator can't be set to unmanaged state.
func IsOperatorAlwaysManaged() bool {
	return !allowOperatorUnmanagedState
}

// IsOperatorNotRemovable means the operator can't be set to removed state.
func IsOperatorNotRemovable() bool {
	return !allowOperatorRemovedState
}

// IsOperatorRemovable means the operator can be set to removed state.
func IsOperatorRemovable() bool {
	return allowOperatorRemovedState
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
