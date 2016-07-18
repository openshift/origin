package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	kapi "k8s.io/kubernetes/pkg/api/v1"
)

// +genclient=true

// PodSecurityPolicySubjectReview checks whether a particular user/SA tuple can create the PodSpec.
type PodSecurityPolicySubjectReview struct {
	unversioned.TypeMeta `json:",inline"`

	// spec defines specification for the PodSecurityPolicySubjectReview.
	Spec PodSecurityPolicySubjectReviewSpec `json:"spec" protobuf:"bytes,1,opt,name=spec"`

	// status represents the current information/status for the PodSecurityPolicySubjectReview.
	Status PodSecurityPolicySubjectReviewStatus `json:"status,omitempty" protobuf:"bytes,2,opt,name=status"`
}

// PodSecurityPolicySubjectReviewSpec defines specification for PodSecurityPolicySubjectReview
type PodSecurityPolicySubjectReviewSpec struct {
	// podSpec is the PodSpec to check. If podSpec.serviceAccountName is empty it will not be defaulted.
	// If its non-empty, it will be checked.
	PodSpec kapi.PodSpec `json:"podSpec" protobuf:"bytes,1,opt,name=podSpec"`

	// user is the user you're testing for.
	// If you specify "user" but not "group", then is it interpreted as "What if user were not a member of any groups.
	// If user and groups are empty, then the check is performed using *only* the serviceAccountName in the podSpec.
	User string `json:"user,omitempty" protobuf:"bytes,2,opt,name=user"`

	// groups is the groups you're testing for.
	Groups []string `json:"groups,omitempty" protobuf:"bytes,3,rep,name=groups"`
}

// PodSecurityPolicySubjectReviewStatus contains information/status for PodSecurityPolicySubjectReview.
type PodSecurityPolicySubjectReviewStatus struct {
	// allowedBy is a reference to the rule that allows the PodSpec.
	// A rule can be a SecurityContextConstraint or a PodSecurityPolicy
	// A `nil`, indicates that it was denied.
	AllowedBy *kapi.ObjectReference `json:"allowedBy,omitempty" protobuf:"bytes,1,opt,name=allowedBy"`

	// A machine-readable description of why this operation is in the
	// "Failure" status. If this value is empty there
	// is no information available.
	Reason string `json:"reason,omitempty" protobuf:"bytes,2,opt,name=reason"`

	// podSpec is the PodSpec after the defaulting is applied.
	PodSpec kapi.PodSpec `json:"podSpec,omitempty" protobuf:"bytes,3,opt,name=podSpec"`
}

// PodSecurityPolicySelfSubjectReview checks whether this user/SA tuple can create the PodSpec
type PodSecurityPolicySelfSubjectReview struct {
	unversioned.TypeMeta `json:",inline"`

	// spec defines specification the PodSecurityPolicySelfSubjectReview.
	Spec PodSecurityPolicySelfSubjectReviewSpec `json:"spec" protobuf:"bytes,1,opt,name=spec"`

	// status represents the current information/status for the PodSecurityPolicySelfSubjectReview.
	Status PodSecurityPolicySubjectReviewStatus `json:"status,omitempty" protobuf:"bytes,2,opt,name=status"`
}

// PodSecurityPolicySelfSubjectReviewSpec contains specification for PodSecurityPolicySelfSubjectReview.
type PodSecurityPolicySelfSubjectReviewSpec struct {
	// podSpec is the PodSpec to check.
	PodSpec kapi.PodSpec `json:"podSpec" protobuf:"bytes,1,opt,name=podSpec"`
}

// PodSecurityPolicyReview checks which service accounts (not users, since that would be cluster-wide) can create the `PodSpec` in question.
type PodSecurityPolicyReview struct {
	unversioned.TypeMeta `json:",inline"`

	// spec is the PodSecurityPolicy to check.
	Spec PodSecurityPolicyReviewSpec `json:"spec" protobuf:"bytes,1,opt,name=spec"`

	// status represents the current information/status for the PodSecurityPolicyReview.
	Status PodSecurityPolicyReviewStatus `json:"status,omitempty" protobuf:"bytes,2,opt,name=status"`
}

// PodSecurityPolicyReviewSpec defines specification for PodSecurityPolicyReview
type PodSecurityPolicyReviewSpec struct {
	// podSpec is the PodSpec to check. The podSpec.serviceAccountName field is used
	// if serviceAccountNames is empty, unless the podSpec.serviceAccountName is empty,
	// in which case "default" is used.
	// If serviceAccountNames is specified, podSpec.serviceAccountName is ignored.
	PodSpec kapi.PodSpec `json:"podSpec" protobuf:"bytes,1,opt,name=podSpec"`

	// serviceAccountNames is an optional set of ServiceAccounts to run the check with.
	// If serviceAccountNames is empty, the podSpec serviceAccountName is used,
	// unless it's empty, in which case "default" is used instead.
	// If serviceAccountNames is specified, podSpec serviceAccountName is ignored.
	ServiceAccountNames []string `json:"serviceAccountNames,omitempty" protobuf:"bytes,2,rep,name=serviceAccountNames"` // TODO: find a way to express 'all service accounts'
}

// PodSecurityPolicyReviewStatus represents the status of PodSecurityPolicyReview.
type PodSecurityPolicyReviewStatus struct {
	// allowedServiceAccounts returns the list of service accounts in *this* namespace that have the power to create the PodSpec.
	AllowedServiceAccounts []ServiceAccountPodSecurityPolicyReviewStatus `json:"allowedServiceAccounts" protobuf:"bytes,1,rep,name=allowedServiceAccounts"`
}

// ServiceAccountPodSecurityPolicyReviewStatus represents ServiceAccount name and related review status
type ServiceAccountPodSecurityPolicyReviewStatus struct {
	PodSecurityPolicySubjectReviewStatus `json:",inline" protobuf:"bytes,1,opt,name=podSecurityPolicySubjectReviewStatus"`

	// name contains the allowed and the denied ServiceAccount name
	Name string `json:"name" protobuf:"bytes,2,opt,name=name"`
}
