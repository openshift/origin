package oci

import (
	"fmt"
)

// KubernetesContainerTypeAnnotation is the annotation used by CRI to define the `ContainerType`.
const KubernetesContainerTypeAnnotation = "io.kubernetes.cri.container-type"

// KubernetesSandboxIDAnnotation is the annotation used by CRI to define the
// KubernetesContainerTypeAnnotation == "sandbox"` ID.
const KubernetesSandboxIDAnnotation = "io.kubernetes.cri.sandbox-id"

// KubernetesContainerType defines the valid types of the
// `KubernetesContainerTypeAnnotation` annotation.
type KubernetesContainerType string

const (
	// KubernetesContainerTypeNone is only valid when
	// `KubernetesContainerTypeAnnotation` is not set.
	KubernetesContainerTypeNone KubernetesContainerType = ""
	// KubernetesContainerTypeContainer is valid when
	// `KubernetesContainerTypeAnnotation == "container"`.
	KubernetesContainerTypeContainer KubernetesContainerType = "container"
	// KubernetesContainerTypeSandbox is valid when
	// `KubernetesContainerTypeAnnotation == "sandbox"`.
	KubernetesContainerTypeSandbox KubernetesContainerType = "sandbox"
)

// GetSandboxTypeAndID parses `specAnnotations` searching for the
// `KubernetesContainerTypeAnnotation` and `KubernetesSandboxIDAnnotation`
// annotations and if found validates the set before returning.
func GetSandboxTypeAndID(specAnnotations map[string]string) (KubernetesContainerType, string, error) {
	var ct KubernetesContainerType
	if t, ok := specAnnotations[KubernetesContainerTypeAnnotation]; ok {
		switch t {
		case string(KubernetesContainerTypeContainer):
			ct = KubernetesContainerTypeContainer
		case string(KubernetesContainerTypeSandbox):
			ct = KubernetesContainerTypeSandbox
		default:
			return KubernetesContainerTypeNone, "", fmt.Errorf("invalid '%s': '%s'", KubernetesContainerTypeAnnotation, t)
		}
	}

	id := specAnnotations[KubernetesSandboxIDAnnotation]

	switch ct {
	case KubernetesContainerTypeContainer, KubernetesContainerTypeSandbox:
		if id == "" {
			return KubernetesContainerTypeNone, "", fmt.Errorf("cannot specify '%s' without '%s'", KubernetesContainerTypeAnnotation, KubernetesSandboxIDAnnotation)
		}
	default:
		if id != "" {
			return KubernetesContainerTypeNone, "", fmt.Errorf("cannot specify '%s' without '%s'", KubernetesSandboxIDAnnotation, KubernetesContainerTypeAnnotation)
		}
	}
	return ct, id, nil
}
