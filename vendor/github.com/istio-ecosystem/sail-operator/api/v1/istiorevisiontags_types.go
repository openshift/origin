// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	IstioRevisionTagKind = "IstioRevisionTag"
	DefaultRevisionTag   = "default"
)

// IstioRevisionTagSpec defines the desired state of IstioRevisionTag
type IstioRevisionTagSpec struct {
	// +kubebuilder:validation:Required
	TargetRef IstioRevisionTagTargetReference `json:"targetRef"`
}

// IstioRevisionTagTargetReference can reference either Istio or IstioRevision objects in the cluster. In the case of referencing an Istio object, the Sail Operator will automatically update the reference to the Istio object's Active Revision.
type IstioRevisionTagTargetReference struct {
	// Kind is the kind of the target resource.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`

	// Name is the name of the target resource.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

// IstioRevisionStatus defines the observed state of IstioRevision
type IstioRevisionTagStatus struct {
	// ObservedGeneration is the most recent generation observed for this
	// IstioRevisionTag object. It corresponds to the object's generation, which is
	// updated on mutation by the API Server. The information in the status
	// pertains to this particular generation of the object.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Represents the latest available observations of the object's current state.
	Conditions []IstioRevisionTagCondition `json:"conditions,omitempty"`

	// Reports the current state of the object.
	State IstioRevisionTagConditionReason `json:"state,omitempty"`

	// IstiodNamespace stores the namespace of the corresponding Istiod instance
	IstiodNamespace string `json:"istiodNamespace"`

	// IstioRevision stores the name of the referenced IstioRevision
	IstioRevision string `json:"istioRevision"`
}

// GetCondition returns the condition of the specified type
func (s *IstioRevisionTagStatus) GetCondition(conditionType IstioRevisionTagConditionType) IstioRevisionTagCondition {
	if s != nil {
		for i := range s.Conditions {
			if s.Conditions[i].Type == conditionType {
				return s.Conditions[i]
			}
		}
	}
	return IstioRevisionTagCondition{Type: conditionType, Status: metav1.ConditionUnknown}
}

// SetCondition sets a specific condition in the list of conditions
func (s *IstioRevisionTagStatus) SetCondition(condition IstioRevisionTagCondition) {
	var now time.Time
	if testTime == nil {
		now = time.Now()
	} else {
		now = *testTime
	}

	// The lastTransitionTime only gets serialized out to the second.  This can
	// break update skipping, as the time in the resource returned from the client
	// may not match the time in our cached status during a reconcile.  We truncate
	// here to save any problems down the line.
	lastTransitionTime := metav1.NewTime(now.Truncate(time.Second))

	for i, prevCondition := range s.Conditions {
		if prevCondition.Type == condition.Type {
			if prevCondition.Status != condition.Status {
				condition.LastTransitionTime = lastTransitionTime
			} else {
				condition.LastTransitionTime = prevCondition.LastTransitionTime
			}
			s.Conditions[i] = condition
			return
		}
	}

	// If the condition does not exist, initialize the lastTransitionTime
	condition.LastTransitionTime = lastTransitionTime
	s.Conditions = append(s.Conditions, condition)
}

// IstioRevisionCondition represents a specific observation of the IstioRevision object's state.
type IstioRevisionTagCondition struct {
	// The type of this condition.
	Type IstioRevisionTagConditionType `json:"type,omitempty"`

	// The status of this condition. Can be True, False or Unknown.
	Status metav1.ConditionStatus `json:"status,omitempty"`

	// Unique, single-word, CamelCase reason for the condition's last transition.
	Reason IstioRevisionTagConditionReason `json:"reason,omitempty"`

	// Human-readable message indicating details about the last transition.
	Message string `json:"message,omitempty"`

	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

// IstioRevisionConditionType represents the type of the condition.  Condition stages are:
// Installed, Reconciled, Ready
type IstioRevisionTagConditionType string

// IstioRevisionConditionReason represents a short message indicating how the condition came
// to be in its present state.
type IstioRevisionTagConditionReason string

const (
	// IstioRevisionConditionReconciled signifies whether the controller has
	// successfully reconciled the resources defined through the CR.
	IstioRevisionTagConditionReconciled IstioRevisionTagConditionType = "Reconciled"

	// IstioRevisionTagNameAlreadyExists indicates that the a revision with the same name as the IstioRevisionTag already exists.
	IstioRevisionTagReasonNameAlreadyExists IstioRevisionTagConditionReason = "NameAlreadyExists"

	// IstioRevisionTagReasonReferenceNotFound indicates that the resource referenced by the tag's TargetRef was not found
	IstioRevisionTagReasonReferenceNotFound IstioRevisionTagConditionReason = "RefNotFound"

	// IstioRevisionReasonReconcileError indicates that the reconciliation of the resource has failed, but will be retried.
	IstioRevisionTagReasonReconcileError IstioRevisionTagConditionReason = "ReconcileError"
)

const (
	// IstioRevisionConditionInUse signifies whether any workload is configured to use the revision.
	IstioRevisionTagConditionInUse IstioRevisionTagConditionType = "InUse"

	// IstioRevisionReasonReferencedByWorkloads indicates that the revision is referenced by at least one pod or namespace.
	IstioRevisionTagReasonReferencedByWorkloads IstioRevisionTagConditionReason = "ReferencedByWorkloads"

	// IstioRevisionReasonNotReferenced indicates that the revision is not referenced by any pod or namespace.
	IstioRevisionTagReasonNotReferenced IstioRevisionTagConditionReason = "NotReferencedByAnything"

	// IstioRevisionReasonUsageCheckFailed indicates that the operator could not check whether any workloads use the revision.
	IstioRevisionTagReasonUsageCheckFailed IstioRevisionTagConditionReason = "UsageCheckFailed"
)

const (
	// IstioRevisionTagReasonHealthy indicates that the revision tag has been successfully reconciled and is in use.
	IstioRevisionTagReasonHealthy IstioRevisionTagConditionReason = "Healthy"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=istiorevtag,categories=istio-io
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state",description="The current state of this object."
// +kubebuilder:printcolumn:name="In use",type="string",JSONPath=".status.conditions[?(@.type==\"InUse\")].status",description="Whether the tag is being used by workloads."
// +kubebuilder:printcolumn:name="Revision",type="string",JSONPath=".status.istioRevision",description="The IstioRevision this object is referencing."
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="The age of the object"

// IstioRevisionTag references an Istio or IstioRevision object and serves as an alias for sidecar injection. It can be used to manage stable revision tags without having to use istioctl or helm directly. See https://istio.io/latest/docs/setup/upgrade/canary/#stable-revision-labels for more information on the concept.
type IstioRevisionTag struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IstioRevisionTagSpec   `json:"spec,omitempty"`
	Status IstioRevisionTagStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IstioRevisionTagList contains a list of IstioRevisionTags
type IstioRevisionTagList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IstioRevisionTag `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IstioRevisionTag{}, &IstioRevisionTagList{})
}
