## Problem
It is impossible for a user to know whether the `PodSpec` he's describing will actually be allowed by the current SCC rules.
This becomes even more problematic when trying to reason about whether a `DeploymentConfig` or `ReplicationController` or other resource containing a `PodSpec` 
that a system component will eventually create will work.

## Types
These types are namespace scoped and only return information about the namespace they're posted to.

```go
// PodSpecSubjectReview checks whether a particular user/SA tuple can create the PodSpec
type PodSpecSubjectReview struct {
	unversioned.TypeMeta `json:",inline"`
	kapi.ObjectMeta

	Spec PodSpecSubjectReviewSpec `json:"spec"`

	Status PodSpecReviewStatus `json:"status,omitempty"`
}

type PodSpecSubjectReviewSpec struct {
	// Spec is the PodSpec to check.
	PodSpec kapi.PodSpec

	// User is the user you're testing for.
	// If you specify "User" but not "Group", then is it interpreted as "What if User were not a member of any groups
	// If User and Groups are empty, then the check is performed using *only* the ServiceAccountName in the PodSpec
	User string
	// Groups is the groups you're testing for.
	Groups []string
}

type PodSpecReviewStatus struct {
	// AllowedBy is a reference to the rule that allows the PodSpec.  A `nil`, indicates that it was denied
	AllowedBy *kapi.ObjectReference `json:"allowedSubjects"`

	// Reason is optional.  It indicates why a request was allowed or denied.
	Reason string

	// PodSpec is the PodSpec after the defaulting is applied
	PodSpec kapi.PodSpec
}

// PodSpecSelfSubjectReview checks whether this user/SA tuple can create the PodSpec
type PodSpecSelfSubjectReview struct {
	unversioned.TypeMeta `json:",inline"`
	kapi.ObjectMeta

	Spec PodSpecSelfSubjectReviewSpec `json:"spec"`

	Status PodSpecReviewStatus `json:"status,omitempty"`
}

type PodSpecSubjectReviewSpec struct {
	// Spec is the PodSpec to check.
	PodSpec kapi.PodSpec

	// ExcludeSelf indicates that the check is performed using *only* the ServiceAccountName in the PodSpec
	ExcludeSelf bool
}

// PodSpecReview checks which service accounts (not users, since that would be cluster-wide) can create the `PodSpec` in question
type PodSpecReview struct {
	unversioned.TypeMeta `json:",inline"`
	kapi.ObjectMeta

	// Spec is the PodSpec to check.  The ServiceAccountName field is ignored for this check.
	Spec PodSpec `json:"spec"`

	Status PodSpecReviewStatus `json:"status,omitempty"`
}

type PodSpecReviewStatus struct {
	// AllowedServiceAccounts returns the list of service accounts in *this* namespace that have the power to create the PodSpec
	AllowedServiceAccounts map[string]PodSpecReviewResult `json:"allowedSubjects"`
}

type PodSpecReviewResult struct{
	// PodSpec is the PodSpec after the defaulting is applied
	PodSpec kapi.PodSpec

	// DefaultedBy is a reference to the rule that defaulted this PodSpec
	DefaultedBy kapi.ObjectReference
}

```

## Client commands
We need to add client commands to make retrieving the `PodSpec` easy.  It should be something like `oc policy scc-subject-check -f <file>`
of `oc policy scc-subject-check resourceArgString`.  The client code should then navigate the object, locate a PodSpec, and submit the request.
