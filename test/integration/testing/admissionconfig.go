package testing

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type TestPluginConfig struct {
	metav1.TypeMeta `json:",inline"`
	Data            string `json:"data"`
}

func (obj *TestPluginConfig) GetObjectKind() schema.ObjectKind { return &obj.TypeMeta }
