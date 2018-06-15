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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type TestConfigV1 struct {
	metav1.TypeMeta `json:",inline"`
	Item1           string   `json:"item1"`
	Item2           []string `json:"item2"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type OtherTestConfig2 struct {
	metav1.TypeMeta
	Thing string `json:"thing"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type OtherTestConfig2V2 struct {
	metav1.TypeMeta
	Thing string `json:"thing"`
}

func (obj *TestConfig) GetObjectKind() schema.ObjectKind         { return &obj.TypeMeta }
func (obj *TestConfigV1) GetObjectKind() schema.ObjectKind       { return &obj.TypeMeta }
func (obj *OtherTestConfig2) GetObjectKind() schema.ObjectKind   { return &obj.TypeMeta }
func (obj *OtherTestConfig2V2) GetObjectKind() schema.ObjectKind { return &obj.TypeMeta }
