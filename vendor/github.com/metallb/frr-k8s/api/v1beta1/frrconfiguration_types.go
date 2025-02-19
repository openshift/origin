/*
Copyright 2023.

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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FRRConfigurationSpec defines the desired state of FRRConfiguration.
type FRRConfigurationSpec struct {
	// BGP is the configuration related to the BGP protocol.
	// +optional
	BGP BGPConfig `json:"bgp,omitempty"`

	// Raw is a snippet of raw frr configuration that gets appended to the
	// one rendered translating the type safe API.
	// +optional
	Raw RawConfig `json:"raw,omitempty"`
	// NodeSelector limits the nodes that will attempt to apply this config.
	// When specified, the configuration will be considered only on nodes
	// whose labels match the specified selectors.
	// When it is not specified all nodes will attempt to apply this config.
	// +optional
	NodeSelector metav1.LabelSelector `json:"nodeSelector,omitempty"`
}

// RawConfig is a snippet of raw frr configuration that gets appended to the
// rendered configuration.
type RawConfig struct {
	// Priority is the order with this configuration is appended to the
	// bottom of the rendered configuration. A higher value means the
	// raw config is appended later in the configuration file.
	Priority int `json:"priority,omitempty"`

	// Config is a raw FRR configuration to be appended to the configuration
	// rendered via the k8s api.
	Config string `json:"rawConfig,omitempty"`
}

// BGPConfig is the configuration related to the BGP protocol.
type BGPConfig struct {
	// Routers is the list of routers we want FRR to configure (one per VRF).
	// +optional
	Routers []Router `json:"routers"`
	// BFDProfiles is the list of bfd profiles to be used when configuring the neighbors.
	// +optional
	BFDProfiles []BFDProfile `json:"bfdProfiles,omitempty"`
}

// Router represent a neighbor router we want FRR to connect to.
type Router struct {
	// ASN is the AS number to use for the local end of the session.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=4294967295
	ASN uint32 `json:"asn"`
	// ID is the BGP router ID
	// +optional
	ID string `json:"id,omitempty"`
	// VRF is the host vrf used to establish sessions from this router.
	// +optional
	VRF string `json:"vrf,omitempty"`
	// Neighbors is the list of neighbors we want to establish BGP sessions with.
	// +optional
	Neighbors []Neighbor `json:"neighbors,omitempty"`
	// Prefixes is the list of prefixes we want to advertise from this router instance.
	// +optional
	Prefixes []string `json:"prefixes,omitempty"`

	// Imports is the list of imported VRFs we want for this router / vrf.
	// +optional
	Imports []Import `json:"imports,omitempty"`
}

// Import represents the possible imported VRFs to a given router.
type Import struct {
	// Vrf is the vrf we want to import from
	// +optional
	VRF string `json:"vrf,omitempty"`
}

// Neighbor represents a BGP Neighbor we want FRR to connect to.
type Neighbor struct {
	// ASN is the AS number to use for the local end of the session.
	// ASN and DynamicASN are mutually exclusive and one of them must be specified.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=4294967295
	// +optional
	ASN uint32 `json:"asn,omitempty"`

	// DynamicASN detects the AS number to use for the local end of the session
	// without explicitly setting it via the ASN field. Limited to:
	// internal - if the neighbor's ASN is different than the router's the connection is denied.
	// external - if the neighbor's ASN is the same as the router's the connection is denied.
	// ASN and DynamicASN are mutually exclusive and one of them must be specified.
	// +kubebuilder:validation:Enum=internal;external
	// +optional
	DynamicASN DynamicASNMode `json:"dynamicASN,omitempty"`

	// SourceAddress is the IPv4 or IPv6 source address to use for the BGP
	// session to this neighbour, may be specified as either an IP address
	// directly or as an interface name
	// +optional
	SourceAddress string `json:"sourceaddress,omitempty"`

	// Address is the IP address to establish the session with.
	Address string `json:"address"`

	// Port is the port to dial when establishing the session.
	// Defaults to 179.
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=16384
	Port *uint16 `json:"port,omitempty"`

	// Password to be used for establishing the BGP session.
	// Password and PasswordSecret are mutually exclusive.
	// +optional
	Password string `json:"password,omitempty"`

	// PasswordSecret is name of the authentication secret for the neighbor.
	// the secret must be of type "kubernetes.io/basic-auth", and created in the
	// same namespace as the frr-k8s daemon. The password is stored in the
	// secret as the key "password".
	// Password and PasswordSecret are mutually exclusive.
	// +optional
	PasswordSecret SecretReference `json:"passwordSecret,omitempty"`

	// HoldTime is the requested BGP hold time, per RFC4271.
	// Defaults to 180s.
	// +optional
	HoldTime *metav1.Duration `json:"holdTime,omitempty"`

	// KeepaliveTime is the requested BGP keepalive time, per RFC4271.
	// Defaults to 60s.
	// +optional
	KeepaliveTime *metav1.Duration `json:"keepaliveTime,omitempty"`

	// Requested BGP connect time, controls how long BGP waits between connection attempts to a neighbor.
	// +kubebuilder:validation:XValidation:message="connect time should be between 1 seconds to 65535",rule="duration(self).getSeconds() >= 1 && duration(self).getSeconds() <= 65535"
	// +kubebuilder:validation:XValidation:message="connect time should contain a whole number of seconds",rule="duration(self).getMilliseconds() % 1000 == 0"
	// +optional
	ConnectTime *metav1.Duration `json:"connectTime,omitempty"`

	// EBGPMultiHop indicates if the BGPPeer is multi-hops away.
	// +optional
	EBGPMultiHop bool `json:"ebgpMultiHop,omitempty"`

	// BFDProfile is the name of the BFD Profile to be used for the BFD session associated
	// to the BGP session. If not set, the BFD session won't be set up.
	// +optional
	BFDProfile string `json:"bfdProfile,omitempty"`

	// EnableGracefulRestart allows BGP peer to continue to forward data packets along
	// known routes while the routing protocol information is being restored. If
	// the session is already established, the configuration will have effect
	// after reconnecting to the peer
	// +optional
	EnableGracefulRestart bool `json:"enableGracefulRestart,omitempty"`

	// ToAdvertise represents the list of prefixes to advertise to the given neighbor
	// and the associated properties.
	// +optional
	ToAdvertise Advertise `json:"toAdvertise,omitempty"`

	// ToReceive represents the list of prefixes to receive from the given neighbor.
	// +optional
	ToReceive Receive `json:"toReceive,omitempty"`

	// To set if we want to disable MP BGP that will separate IPv4 and IPv6 route exchanges into distinct BGP sessions.
	// +optional
	// +kubebuilder:default:=false
	DisableMP bool `json:"disableMP,omitempty"`
}

// Advertise represents a list of prefixes to advertise to the given neighbor.

type Advertise struct {

	// Allowed is is the list of prefixes allowed to be propagated to
	// this neighbor. They must match the prefixes defined in the router.
	Allowed AllowedOutPrefixes `json:"allowed,omitempty"`

	// PrefixesWithLocalPref is a list of prefixes that are associated to a local
	// preference when being advertised. The prefixes associated to a given local pref
	// must be in the prefixes allowed to be advertised.
	// +optional
	PrefixesWithLocalPref []LocalPrefPrefixes `json:"withLocalPref,omitempty"`

	// PrefixesWithCommunity is a list of prefixes that are associated to a
	// bgp community when being advertised. The prefixes associated to a given local pref
	// must be in the prefixes allowed to be advertised.
	// +optional
	PrefixesWithCommunity []CommunityPrefixes `json:"withCommunity,omitempty"`
}

// Receive represents a list of prefixes to receive from the given neighbor.
type Receive struct {
	// Allowed is the list of prefixes allowed to be received from
	// this neighbor.
	// +optional
	Allowed AllowedInPrefixes `json:"allowed,omitempty"`
}

// PrefixSelector is a filter of prefixes to receive.
type PrefixSelector struct {
	// +kubebuilder:validation:Format="cidr"
	Prefix string `json:"prefix,omitempty"`
	// The prefix length modifier. This selector accepts any matching prefix with length
	// less or equal the given value.
	// +kubebuilder:validation:Maximum:=128
	// +kubebuilder:validation:Minimum:=1
	LE uint32 `json:"le,omitempty"`
	// The prefix length modifier. This selector accepts any matching prefix with length
	// greater or equal the given value.
	// +kubebuilder:validation:Maximum:=128
	// +kubebuilder:validation:Minimum:=1
	GE uint32 `json:"ge,omitempty"`
}

type AllowedInPrefixes struct {
	Prefixes []PrefixSelector `json:"prefixes,omitempty"`
	// Mode is the mode to use when handling the prefixes.
	// When set to "filtered", only the prefixes in the given list will be allowed.
	// When set to "all", all the prefixes configured on the router will be allowed.
	// +kubebuilder:default:=filtered
	Mode AllowMode `json:"mode,omitempty"`
}

type AllowedOutPrefixes struct {
	Prefixes []string `json:"prefixes,omitempty"`
	// Mode is the mode to use when handling the prefixes.
	// When set to "filtered", only the prefixes in the given list will be allowed.
	// When set to "all", all the prefixes configured on the router will be allowed.
	// +kubebuilder:default:=filtered
	Mode AllowMode `json:"mode,omitempty"`
}

// LocalPrefPrefixes is a list of prefixes associated to a local preference.
type LocalPrefPrefixes struct {
	// Prefixes is the list of prefixes associated to the local preference.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Format="cidr"
	Prefixes []string `json:"prefixes,omitempty"`
	// LocalPref is the local preference associated to the prefixes.
	LocalPref uint32 `json:"localPref,omitempty"`
}

// CommunityPrefixes is a list of prefixes associated to a community.
type CommunityPrefixes struct {
	// Prefixes is the list of prefixes associated to the community.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Format="cidr"
	Prefixes []string `json:"prefixes,omitempty"`
	// Community is the community associated to the prefixes.
	Community string `json:"community,omitempty"`
}

// BFDProfile is the configuration related to the BFD protocol associated
// to a BGP session.
type BFDProfile struct {
	// The name of the BFD Profile to be referenced in other parts
	// of the configuration.
	Name string `json:"name"`

	// The minimum interval that this system is capable of
	// receiving control packets in milliseconds.
	// Defaults to 300ms.
	// +kubebuilder:validation:Maximum:=60000
	// +kubebuilder:validation:Minimum:=10
	// +optional
	ReceiveInterval *uint32 `json:"receiveInterval,omitempty"`

	// The minimum transmission interval (less jitter)
	// that this system wants to use to send BFD control packets in
	// milliseconds. Defaults to 300ms
	// +kubebuilder:validation:Maximum:=60000
	// +kubebuilder:validation:Minimum:=10
	// +optional
	TransmitInterval *uint32 `json:"transmitInterval,omitempty"`

	// Configures the detection multiplier to determine
	// packet loss. The remote transmission interval will be multiplied
	// by this value to determine the connection loss detection timer.
	// +kubebuilder:validation:Maximum:=255
	// +kubebuilder:validation:Minimum:=2
	// +optional
	DetectMultiplier *uint32 `json:"detectMultiplier,omitempty"`

	// Configures the minimal echo receive transmission
	// interval that this system is capable of handling in milliseconds.
	// Defaults to 50ms
	// +kubebuilder:validation:Maximum:=60000
	// +kubebuilder:validation:Minimum:=10
	// +optional
	EchoInterval *uint32 `json:"echoInterval,omitempty"`

	// Enables or disables the echo transmission mode.
	// This mode is disabled by default, and not supported on multi
	// hops setups.
	// +optional
	EchoMode *bool `json:"echoMode,omitempty"`

	// Mark session as passive: a passive session will not
	// attempt to start the connection and will wait for control packets
	// from peer before it begins replying.
	// +optional
	PassiveMode *bool `json:"passiveMode,omitempty"`

	// For multi hop sessions only: configure the minimum
	// expected TTL for an incoming BFD control packet.
	// +kubebuilder:validation:Maximum:=254
	// +kubebuilder:validation:Minimum:=1
	// +optional
	MinimumTTL *uint32 `json:"minimumTtl,omitempty"`
}

// FRRConfigurationStatus defines the observed state of FRRConfiguration.
type FRRConfigurationStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//nolint
//+genclient

// FRRConfiguration is a piece of FRR configuration.
type FRRConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FRRConfigurationSpec   `json:"spec,omitempty"`
	Status FRRConfigurationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FRRConfigurationList contains a list of FRRConfiguration.
type FRRConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FRRConfiguration `json:"items"`
}

//nolint
//+structType=atomic

// SecretReference represents a Secret Reference. It has enough information to retrieve secret
// in any namespace.
type SecretReference struct {
	// name is unique within a namespace to reference a secret resource.
	// +optional
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	// namespace defines the space within which the secret name must be unique.
	// +optional
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,2,opt,name=namespace"`
}

func init() {
	SchemeBuilder.Register(&FRRConfiguration{}, &FRRConfigurationList{})
}

// +kubebuilder:validation:Enum=all;filtered
type AllowMode string

const (
	AllowAll        AllowMode = "all"
	AllowRestricted AllowMode = "filtered"
)

type DynamicASNMode string

const (
	InternalASNMode DynamicASNMode = "internal"
	ExternalASNMode DynamicASNMode = "external"
)
