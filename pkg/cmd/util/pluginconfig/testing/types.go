package testing

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type TestConfig struct {
	metav1.TypeMeta `json:",inline"`
	Item1           string   `json:"item1"`
	Item2           []string `json:"item2"`
}

func (obj *TestConfig) GetObjectKind() schema.ObjectKind { return &obj.TypeMeta }

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type TestConfigV1 struct {
	metav1.TypeMeta `json:",inline"`
	Item1           string   `json:"item1"`
	Item2           []string `json:"item2"`
}

func (obj *TestConfigV1) GetObjectKind() schema.ObjectKind { return &obj.TypeMeta }
