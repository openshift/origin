package controller

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"
)

// EnsureOwnerRef adds the ownerref if needed. Removes ownerrefs with conflicting UIDs.
// Returns true if the input is mutated.
func EnsureOwnerRef(metadata metav1.Object, newOwnerRef metav1.OwnerReference) bool {
	foundButNotEqual := false
	for _, existingOwnerRef := range metadata.GetOwnerReferences() {
		if existingOwnerRef.APIVersion == newOwnerRef.APIVersion &&
			existingOwnerRef.Kind == newOwnerRef.Kind &&
			existingOwnerRef.Name == newOwnerRef.Name {

			// if we're completely the same, there's nothing to do
			if kapihelper.Semantic.DeepEqual(existingOwnerRef, newOwnerRef) {
				return false
			}

			foundButNotEqual = true
			break
		}
	}

	// if we weren't found, then we just need to add ourselves
	if !foundButNotEqual {
		metadata.SetOwnerReferences(append(metadata.GetOwnerReferences(), newOwnerRef))
		return true
	}

	// if we need to remove an existing ownerRef, just do the easy thing and build it back from scratch
	newOwnerRefs := []metav1.OwnerReference{newOwnerRef}
	for i := range metadata.GetOwnerReferences() {
		existingOwnerRef := metadata.GetOwnerReferences()[i]
		if existingOwnerRef.APIVersion == newOwnerRef.APIVersion &&
			existingOwnerRef.Kind == newOwnerRef.Kind &&
			existingOwnerRef.Name == newOwnerRef.Name {
			continue
		}
		newOwnerRefs = append(newOwnerRefs, existingOwnerRef)
	}
	metadata.SetOwnerReferences(newOwnerRefs)
	return true
}

// HasOwnerRef checks to see if an object has a particular owner.  It is not opinionated about
// the bool fields
func HasOwnerRef(metadata metav1.Object, needle metav1.OwnerReference) bool {
	for _, existingOwnerRef := range metadata.GetOwnerReferences() {
		if existingOwnerRef.APIVersion == needle.APIVersion &&
			existingOwnerRef.Kind == needle.Kind &&
			existingOwnerRef.Name == needle.Name &&
			existingOwnerRef.UID == needle.UID {
			return true
		}
	}
	return false
}
