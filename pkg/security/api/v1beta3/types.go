package v1beta3

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	kapi "k8s.io/kubernetes/pkg/api/v1beta3"
)

// PodSpecSubjectReview checks whether a particular user/SA tuple can create the PodSpec.
type PodSpecSubjectReview struct {
	unversioned.TypeMeta `json:",inline"`

	// Spec defines specification for the PodSpecSubjectReview
	Spec PodSpecSubjectReviewSpec `json:"spec"`

	// Status represents the current information/status for the PodSpecSubjectReview
	Status PodSpecSubjectReviewStatus `json:"status,omitempty"`
}

// PodSpecSubjectReviewSpec defines specification for PodSpecSubjectReview
type PodSpecSubjectReviewSpec struct {
	// Spec is the PodSpec to check.
	PodSpec kapi.PodSpec `json:"spec"`

	// User is the user you're testing for.
	// If you specify "User" but not "Group", then is it interpreted as "What if User were not a member of any groups
	// If User and Groups are empty, then the check is performed using *only* the ServiceAccountName in the PodSpec
	User string `json:"user,omitempty"`
	// Groups is the groups you're testing for.
	Groups []string `json:"groups,omitempty"`
}

// PodSpecSubjectReviewStatus contains information/status for PodSpecSubjectReview
type PodSpecSubjectReviewStatus struct {
	// AllowedBy is a reference to the rule that allows the PodSpec.  A `nil`, indicates that it was denied
	AllowedBy *kapi.ObjectReference `json:"allowedSubjects"`

	// Reason is optional.  It indicates why a request was allowed or denied.
	Reason string `json:"reason,omitempty"`

	// PodSpec is the PodSpec after the defaulting is applied
	PodSpec kapi.PodSpec `json:"spec"`
}

// PodSpecSelfSubjectReview checks whether this user/SA tuple can create the PodSpec
type PodSpecSelfSubjectReview struct {
	unversioned.TypeMeta `json:",inline"`

	// Spec defines specification the PodSpecSelfSubjectReview
	Spec PodSpecSelfSubjectReviewSpec `json:"spec"`

	// Status represents the current information/status for the PodSpecSelfSubjectReview
	Status PodSpecSubjectReviewStatus `json:"status,omitempty"`
}

// PodSpecSelfSubjectReviewSpec contains specification for PodSpecSelfSubjectReview
type PodSpecSelfSubjectReviewSpec struct {
	// Spec is the PodSpec to check.
	PodSpec kapi.PodSpec `json:"spec"`

	// ExcludeSelf indicates that the check is performed using *only* the ServiceAccountName in the PodSpec
	ExcludeSelf bool `json:"excluseSelf,omitempty"`
}

// PodSpecReview checks which service accounts (not users, since that would be cluster-wide) can create the `PodSpec` in question
type PodSpecReview struct {
	unversioned.TypeMeta `json:",inline"`

	// Spec is the PodSpec to check.  The ServiceAccountName field is ignored for this check.
	Spec kapi.PodSpec `json:"spec"`

	// Status represents the current information/status for the PodSpecReview
	Status PodSpecReviewStatus `json:"status,omitempty"`
}

// PodSpecReviewStatus represents the status of PodSpecReview
type PodSpecReviewStatus struct {
	// AllowedServiceAccounts returns the list of service accounts in *this* namespace that have the power to create the PodSpec
	AllowedServiceAccounts map[string]PodSpecReviewResult `json:"allowedSubjects"`
}

// PodSpecReviewResult contains information related the a specifc service account for the requested PodSpec
type PodSpecReviewResult struct {
	// PodSpec is the PodSpec after the defaulting is applied
	PodSpec kapi.PodSpec `json:"spec"`

	// DefaultedBy is a reference to the rule that defaulted this PodSpec
	DefaultedBy kapi.ObjectReference `json:"defaultedBy"`
}
