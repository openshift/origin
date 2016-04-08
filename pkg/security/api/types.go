package api

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
)

// PodSpecSubjectReview checks whether a particular user/SA tuple can create the PodSpec
type PodSpecSubjectReview struct {
	unversioned.TypeMeta `json:",inline"`

	Spec PodSpecSubjectReviewSpec `json:"spec"`

	Status PodSpecSubjectReviewStatus `json:"status,omitempty"`
}

type PodSpecSubjectReviewSpec struct {
	PodSpec kapi.PodSpec

	User string

	Groups []string
}

type PodSpecSubjectReviewStatus struct {
	AllowedBy *kapi.ObjectReference `json:"allowedSubjects"`

	Reason string

	// PodSpec is the PodSpec after the defaulting is applied
	PodSpec kapi.PodSpec
}

// PodSpecSelfSubjectReview checks whether this user/SA tuple can create the PodSpec
type PodSpecSelfSubjectReview struct {
	unversioned.TypeMeta `json:",inline"`

	Spec PodSpecSelfSubjectReviewSpec `json:"spec"`

	Status PodSpecSubjectReviewStatus `json:"status,omitempty"`
}

type PodSpecSelfSubjectReviewSpec struct {
	PodSpec kapi.PodSpec

	ExcludeSelf bool
}

// PodSpecReview checks which service accounts (not users, since that would be cluster-wide) can create the `PodSpec` in question
type PodSpecReview struct {
	unversioned.TypeMeta `json:",inline"`

	Spec kapi.PodSpec `json:"spec"`

	Status PodSpecReviewStatus `json:"status,omitempty"`
}

type PodSpecReviewStatus struct {
	AllowedServiceAccounts map[string]PodSpecReviewResult `json:"allowedSubjects"`
}

type PodSpecReviewResult struct {
	PodSpec kapi.PodSpec

	DefaultedBy kapi.ObjectReference
}
