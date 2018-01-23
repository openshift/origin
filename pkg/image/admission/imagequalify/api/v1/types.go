package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ImageQualifyConfig struct {
	metav1.TypeMeta `json:",inline"`

	Rules []ImageQualifyRule `json:"rules"`
}

type ImageQualifyRule struct {
	Pattern string `json:"pattern"`
	Domain  string `json:"domain"`
}
