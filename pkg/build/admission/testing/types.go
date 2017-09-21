package testing

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	_ "github.com/openshift/origin/pkg/api/install"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:deepcopy-gen=true
type TestConfig struct {
	metav1.TypeMeta

	Item1 string   `json:"item1"`
	Item2 []string `json:"item2"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:deepcopy-gen=true
type TestConfigV1 struct {
	metav1.TypeMeta

	Item1 string   `json:"item1"`
	Item2 []string `json:"item2"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:deepcopy-gen=true
type OtherTestConfig2 struct {
	metav1.TypeMeta
	Thing string `json:"thing"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:deepcopy-gen=true
type OtherTestConfig2V2 struct {
	metav1.TypeMeta
	Thing string `json:"thing"`
}

func (obj *TestConfig) GetObjectKind() schema.ObjectKind         { return &obj.TypeMeta }
func (obj *TestConfigV1) GetObjectKind() schema.ObjectKind       { return &obj.TypeMeta }
func (obj *OtherTestConfig2) GetObjectKind() schema.ObjectKind   { return &obj.TypeMeta }
func (obj *OtherTestConfig2V2) GetObjectKind() schema.ObjectKind { return &obj.TypeMeta }
