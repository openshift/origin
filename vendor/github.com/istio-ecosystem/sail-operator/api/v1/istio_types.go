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

const IstioKind = "Istio"

type UpdateStrategyType string

const (
	UpdateStrategyTypeInPlace       UpdateStrategyType = "InPlace"
	UpdateStrategyTypeRevisionBased UpdateStrategyType = "RevisionBased"

	DefaultRevisionDeletionGracePeriodSeconds = 30
	MinRevisionDeletionGracePeriodSeconds     = 0
)

// IstioSpec defines the desired state of Istio
// +kubebuilder:validation:XValidation:rule="!has(self.values) || !has(self.values.global) || !has(self.values.global.istioNamespace) || self.values.global.istioNamespace == self.__namespace__",message="spec.values.global.istioNamespace must match spec.namespace"
type IstioSpec struct {
	// +sail:version
	// Defines the version of Istio to install.
	// Must be one of: v1.24-latest, v1.24.3, v1.24.2, v1.24.1, v1.24.0, v1.23-latest, v1.23.5, v1.23.4, v1.23.3, v1.23.2, v1.22-latest, v1.22.8, v1.22.7, v1.22.6, v1.22.5, v1.21.6, master, v1.25-alpha.c2ac935c.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=1,displayName="Istio Version",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:General", "urn:alm:descriptor:com.tectonic.ui:select:v1.24-latest", "urn:alm:descriptor:com.tectonic.ui:select:v1.24.3", "urn:alm:descriptor:com.tectonic.ui:select:v1.24.2", "urn:alm:descriptor:com.tectonic.ui:select:v1.24.1", "urn:alm:descriptor:com.tectonic.ui:select:v1.24.0", "urn:alm:descriptor:com.tectonic.ui:select:v1.23-latest", "urn:alm:descriptor:com.tectonic.ui:select:v1.23.5", "urn:alm:descriptor:com.tectonic.ui:select:v1.23.4", "urn:alm:descriptor:com.tectonic.ui:select:v1.23.3", "urn:alm:descriptor:com.tectonic.ui:select:v1.23.2", "urn:alm:descriptor:com.tectonic.ui:select:v1.22-latest", "urn:alm:descriptor:com.tectonic.ui:select:v1.22.8", "urn:alm:descriptor:com.tectonic.ui:select:v1.22.7", "urn:alm:descriptor:com.tectonic.ui:select:v1.22.6", "urn:alm:descriptor:com.tectonic.ui:select:v1.22.5", "urn:alm:descriptor:com.tectonic.ui:select:v1.21.6", "urn:alm:descriptor:com.tectonic.ui:select:master", "urn:alm:descriptor:com.tectonic.ui:select:v1.25-alpha.c2ac935c"}
	// +kubebuilder:validation:Enum=v1.24-latest;v1.24.3;v1.24.2;v1.24.1;v1.24.0;v1.23-latest;v1.23.5;v1.23.4;v1.23.3;v1.23.2;v1.22-latest;v1.22.8;v1.22.7;v1.22.6;v1.22.5;v1.21.6;master;v1.25-alpha.c2ac935c
	// +kubebuilder:default=v1.24.3
	Version string `json:"version"`

	// Defines the update strategy to use when the version in the Istio CR is updated.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Update Strategy"
	// +kubebuilder:default={type: "InPlace"}
	UpdateStrategy *IstioUpdateStrategy `json:"updateStrategy,omitempty"`

	// +sail:profile
	// The built-in installation configuration profile to use.
	// The 'default' profile is always applied. On OpenShift, the 'openshift' profile is also applied on top of 'default'.
	// Must be one of: ambient, default, demo, empty, external, openshift-ambient, openshift, preview, remote, stable.
	// +++PROFILES-DROPDOWN-HIDDEN-UNTIL-WE-FULLY-IMPLEMENT-THEM+++operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Profile",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:General", "urn:alm:descriptor:com.tectonic.ui:select:ambient", "urn:alm:descriptor:com.tectonic.ui:select:default", "urn:alm:descriptor:com.tectonic.ui:select:demo", "urn:alm:descriptor:com.tectonic.ui:select:empty", "urn:alm:descriptor:com.tectonic.ui:select:external", "urn:alm:descriptor:com.tectonic.ui:select:minimal", "urn:alm:descriptor:com.tectonic.ui:select:preview", "urn:alm:descriptor:com.tectonic.ui:select:remote"}
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	// +kubebuilder:validation:Enum=ambient;default;demo;empty;external;openshift-ambient;openshift;preview;remote;stable
	Profile string `json:"profile,omitempty"`

	// Namespace to which the Istio components should be installed. Note that this field is immutable.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:io.kubernetes:Namespace"}
	// +kubebuilder:default=istio-system
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Namespace string `json:"namespace"`

	// Defines the values to be passed to the Helm charts when installing Istio.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Helm Values"
	Values *Values `json:"values,omitempty"`
}

// IstioUpdateStrategy defines how the control plane should be updated when the version in
// the Istio CR is updated.
type IstioUpdateStrategy struct {
	// Type of strategy to use. Can be "InPlace" or "RevisionBased". When the "InPlace" strategy
	// is used, the existing Istio control plane is updated in-place. The workloads therefore
	// don't need to be moved from one control plane instance to another. When the "RevisionBased"
	// strategy is used, a new Istio control plane instance is created for every change to the
	// Istio.spec.version field. The old control plane remains in place until all workloads have
	// been moved to the new control plane instance.
	//
	// The "InPlace" strategy is the default.	TODO: change default to "RevisionBased"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=1,displayName="Type",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:select:InPlace", "urn:alm:descriptor:com.tectonic.ui:select:RevisionBased"}
	// +kubebuilder:validation:Enum=InPlace;RevisionBased
	// +kubebuilder:default=InPlace
	Type UpdateStrategyType `json:"type,omitempty"`

	// Defines how many seconds the operator should wait before removing a non-active revision after all
	// the workloads have stopped using it. You may want to set this value on the order of minutes.
	// The minimum is 0 and the default value is 30.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=2,displayName="Inactive Revision Deletion Grace Period (seconds)",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number"}
	// +kubebuilder:validation:Minimum=0
	InactiveRevisionDeletionGracePeriodSeconds *int64 `json:"inactiveRevisionDeletionGracePeriodSeconds,omitempty"`

	// Defines whether the workloads should be moved from one control plane instance to another
	// automatically. If updateWorkloads is true, the operator moves the workloads from the old
	// control plane instance to the new one after the new control plane is ready.
	// If updateWorkloads is false, the user must move the workloads manually by updating the
	// istio.io/rev labels on the namespace and/or the pods.
	// Defaults to false.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=3,displayName="Update Workloads Automatically",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	UpdateWorkloads bool `json:"updateWorkloads,omitempty"`
}

// IstioStatus defines the observed state of Istio
type IstioStatus struct {
	// ObservedGeneration is the most recent generation observed for this
	// Istio object. It corresponds to the object's generation, which is
	// updated on mutation by the API Server. The information in the status
	// pertains to this particular generation of the object.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Represents the latest available observations of the object's current state.
	Conditions []IstioCondition `json:"conditions,omitempty"`

	// Reports the current state of the object.
	State IstioConditionReason `json:"state,omitempty"`

	// The name of the active revision.
	ActiveRevisionName string `json:"activeRevisionName,omitempty"`

	// Reports information about the underlying IstioRevisions.
	Revisions RevisionSummary `json:"revisions,omitempty"`
}

// RevisionSummary contains information on the number of IstioRevisions associated with this Istio.
type RevisionSummary struct {
	// Total number of IstioRevisions currently associated with this Istio.
	Total int32 `json:"total"`

	// Number of IstioRevisions that are Ready.
	Ready int32 `json:"ready"`

	// Number of IstioRevisions that are currently in use.
	InUse int32 `json:"inUse"`
}

// GetCondition returns the condition of the specified type
func (s *IstioStatus) GetCondition(conditionType IstioConditionType) IstioCondition {
	if s != nil {
		for i := range s.Conditions {
			if s.Conditions[i].Type == conditionType {
				return s.Conditions[i]
			}
		}
	}
	return IstioCondition{Type: conditionType, Status: metav1.ConditionUnknown}
}

// SetCondition sets a specific condition in the list of conditions
func (s *IstioStatus) SetCondition(condition IstioCondition) {
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

// IstioCondition represents a specific observation of the IstioCondition object's state.
type IstioCondition struct {
	// The type of this condition.
	Type IstioConditionType `json:"type,omitempty"`

	// The status of this condition. Can be True, False or Unknown.
	Status metav1.ConditionStatus `json:"status,omitempty"`

	// Unique, single-word, CamelCase reason for the condition's last transition.
	Reason IstioConditionReason `json:"reason,omitempty"`

	// Human-readable message indicating details about the last transition.
	Message string `json:"message,omitempty"`

	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

// IstioConditionType represents the type of the condition.  Condition stages are:
// Installed, Reconciled, Ready
type IstioConditionType string

// IstioConditionReason represents a short message indicating how the condition came
// to be in its present state.
type IstioConditionReason string

const (
	// IstioConditionReconciled signifies whether the controller has
	// successfully reconciled the resources defined through the CR.
	IstioConditionReconciled IstioConditionType = "Reconciled"

	// IstioReasonReconcileError indicates that the reconciliation of the resource has failed, but will be retried.
	IstioReasonReconcileError IstioConditionReason = "ReconcileError"
)

const (
	// IstioConditionReady signifies whether any Deployment, StatefulSet,
	// etc. resources are Ready.
	IstioConditionReady IstioConditionType = "Ready"

	// IstioReasonRevisionNotFound indicates that the active IstioRevision is not found.
	IstioReasonRevisionNotFound IstioConditionReason = "ActiveRevisionNotFound"

	// IstioReasonFailedToGetActiveRevision indicates that a failure occurred when getting the active IstioRevision
	IstioReasonFailedToGetActiveRevision IstioConditionReason = "FailedToGetActiveRevision"

	// IstioReasonIstiodNotReady indicates that the control plane is fully reconciled, but istiod is not ready.
	IstioReasonIstiodNotReady IstioConditionReason = "IstiodNotReady"

	// IstioReasonRemoteIstiodNotReady indicates that the control plane is fully reconciled, but the remote istiod is not ready.
	IstioReasonRemoteIstiodNotReady IstioConditionReason = "RemoteIstiodNotReady"

	// IstioReasonReadinessCheckFailed indicates that readiness could not be ascertained.
	IstioReasonReadinessCheckFailed IstioConditionReason = "ReadinessCheckFailed"
)

const (
	// IstioReasonHealthy indicates that the control plane is fully reconciled and that all components are ready.
	IstioReasonHealthy IstioConditionReason = "Healthy"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,categories=istio-io
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Revisions",type="string",JSONPath=".status.revisions.total",description="Total number of IstioRevision objects currently associated with this object."
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.revisions.ready",description="Number of revisions that are ready."
// +kubebuilder:printcolumn:name="In use",type="string",JSONPath=".status.revisions.inUse",description="Number of revisions that are currently being used by workloads."
// +kubebuilder:printcolumn:name="Active Revision",type="string",JSONPath=".status.activeRevisionName",description="The name of the currently active revision."
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state",description="The current state of the active revision."
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.version",description="The version of the control plane installation."
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="The age of the object"

// Istio represents an Istio Service Mesh deployment consisting of one or more
// control plane instances (represented by one or more IstioRevision objects).
// To deploy an Istio Service Mesh, a user creates an Istio object with the
// desired Istio version and configuration. The operator then creates
// an IstioRevision object, which in turn creates the underlying Deployment
// objects for istiod and other control plane components, similar to how a
// Deployment object in Kubernetes creates ReplicaSets that create the Pods.
type Istio struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:default={version: "v1.24.3", namespace: "istio-system", updateStrategy: {type:"InPlace"}}
	Spec IstioSpec `json:"spec,omitempty"`

	Status IstioStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IstioList contains a list of Istio
type IstioList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Istio `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Istio{}, &IstioList{})
}
