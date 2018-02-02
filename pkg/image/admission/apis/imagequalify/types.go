package imagequalify

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ImageQualifyConfig struct {
	metav1.TypeMeta

	Rules []ImageQualifyRule
}

type ImageQualifyRule struct {
	Pattern string
	Domain  string
}
