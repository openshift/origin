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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ImageFormat enumerates the allowed image formats
// +kubebuilder:validation:Enum=iso;initrd
type ImageFormat string

const PreprovisioningImageFinalizer = "preprovisioningimage.metal3.io"

const (
	ImageFormatISO    ImageFormat = "iso"
	ImageFormatInitRD ImageFormat = "initrd"
)

// PreprovisioningImageSpec defines the desired state of PreprovisioningImage.
type PreprovisioningImageSpec struct {
	// networkDataName is the name of a Secret in the local namespace that
	// contains network data to build in to the image.
	// +optional
	NetworkDataName string `json:"networkDataName,omitempty"`

	// architecture is the processor architecture for which to build the image.
	// +optional
	Architecture string `json:"architecture,omitempty"`

	// acceptFormats is a list of acceptable image formats.
	// +optional
	AcceptFormats []ImageFormat `json:"acceptFormats,omitempty"`
}

type SecretStatus struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

type ImageStatusConditionType string

const (
	// Ready indicates that the Image is available and ready to be downloaded.
	ConditionImageReady ImageStatusConditionType = "Ready"

	// Error indicates that the operator was unable to build an image.
	ConditionImageError ImageStatusConditionType = "Error"
)

// PreprovisioningImageStatus defines the observed state of PreprovisioningImage.
type PreprovisioningImageStatus struct {
	// imageUrl is the URL from which the built image can be downloaded.
	//nolint:tagliatelle
	ImageUrl string `json:"imageUrl,omitempty"` //nolint:revive,stylecheck

	// kernelUrl is the URL from which the kernel of the image can be downloaded.
	// Only makes sense for initrd images.
	// +optional
	//nolint:tagliatelle
	KernelUrl string `json:"kernelUrl,omitempty"` //nolint:revive,stylecheck

	// extraKernelParams is a string with extra parameters to pass to the
	// kernel when booting the image over network. Only makes sense for initrd images.
	// +optional
	ExtraKernelParams string `json:"extraKernelParams,omitempty"`

	// format is the type of image that is available at the download url:
	// either iso or initrd.
	// +optional
	Format ImageFormat `json:"format,omitempty"`

	// networkData is a reference to the version of the Secret containing the
	// network data used to build the image.
	// +optional
	NetworkData SecretStatus `json:"networkData,omitempty"`

	// architecture is the processor architecture for which the image is built
	Architecture string `json:"architecture,omitempty"`

	// conditions describe the state of the built image
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=ppimg
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status",description="Whether the image is ready"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason",description="The reason for the image readiness status"
// +kubebuilder:subresource:status

// PreprovisioningImage is the Schema for the preprovisioningimages API.
type PreprovisioningImage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PreprovisioningImageSpec   `json:"spec,omitempty"`
	Status PreprovisioningImageStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PreprovisioningImageList contains a list of PreprovisioningImage.
type PreprovisioningImageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PreprovisioningImage `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PreprovisioningImage{}, &PreprovisioningImageList{})
}
