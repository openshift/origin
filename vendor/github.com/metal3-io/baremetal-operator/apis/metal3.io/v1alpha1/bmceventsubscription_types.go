/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (

	// BMCEventSubscriptionFinalizer is the name of the finalizer added to
	// subscriptions to block delete operations until the subscription is removed
	// from the BMC.
	BMCEventSubscriptionFinalizer string = "bmceventsubscription.metal3.io"
)

type BMCEventSubscriptionSpec struct {
	// A reference to a BareMetalHost
	HostName string `json:"hostName,omitempty"`

	// A webhook URL to send events to
	Destination string `json:"destination,omitempty"`

	// Arbitrary user-provided context for the event
	Context string `json:"context,omitempty"`

	// A secret containing HTTP headers which should be passed along to the Destination
	// when making a request
	HTTPHeadersRef *corev1.SecretReference `json:"httpHeadersRef,omitempty"`
}

type BMCEventSubscriptionStatus struct {
	SubscriptionID string `json:"subscriptionID,omitempty"`
	Error          string `json:"error,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//
// BMCEventSubscription is the Schema for the fast eventing API
// +k8s:openapi-gen=true
// +kubebuilder:resource:shortName=bes;bmcevent
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Error",type="string",JSONPath=".status.error",description="The most recent error message"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of BMCEventSubscription"
// +kubebuilder:object:root=true
type BMCEventSubscription struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BMCEventSubscriptionSpec   `json:"spec,omitempty"`
	Status BMCEventSubscriptionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BMCEventSubscriptionList contains a list of BMCEventSubscriptions.
type BMCEventSubscriptionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BMCEventSubscription `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BMCEventSubscription{}, &BMCEventSubscriptionList{})
}
