package observe

import (
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

type ObservationType string

const (
	ObservationTypeAdd    ObservationType = "add"
	ObservationTypeUpdate ObservationType = "update"
	ObservationTypeDelete ObservationType = "delete"
)

type ResourceObservation struct {
	Group    string `json:"group"`
	Version  string `json:"version"`
	Resource string `json:"resource"`

	UID types.UID `json:"uid"`

	Object    *unstructured.Unstructured `json:"object,omitempty"`
	OldObject *unstructured.Unstructured `json:"oldObject,omitempty"`

	ObservationType ObservationType `json:"observationType"`
	ObservationTime time.Time       `json:"observationTime"`
}
