package observe

import (
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

type ObservationType string

const (
	ObservationTypeAdd    ObservationType = "add"
	ObservationTypeUpdate ObservationType = "update"
	ObservationTypeDelete ObservationType = "delete"
)

type ResourceObservation struct {
	schema.GroupVersionResource

	UID       types.UID
	Namespace string
	Name      string

	Object    *unstructured.Unstructured
	OldObject *unstructured.Unstructured

	ObservationType ObservationType
	ObservationTime time.Time
}
