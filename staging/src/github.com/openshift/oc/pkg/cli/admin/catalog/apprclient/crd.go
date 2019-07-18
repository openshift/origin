package apprclient

import (
	"fmt"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
)

// CRDKey contains metadata needed to uniquely identify a CRD.
//
// OLM uses CRDKey to uniquely identify a CustomResourceDefinition object. We
// are following the same pattern to be consistent.
type CRDKey struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

// String returns a string representation of this CRDKey object with Kind, Name
// and Version concatenated.
//
// CRDKey is used as the key to map of CustomResourceDefinition object(s). This
// function ensures that Kind, Name and Version are taken into account
// to compute the key associated with a CustomResourceDefinition object.
func (k CRDKey) String() string {
	return fmt.Sprintf("%s/%s/%s", k.Kind, k.Name, k.Version)
}

// CustomResourceDefinition is a structured representation of custom resource
// definition(s) specified in `customResourceDefinitions` section of an
// operator manifest.
type CustomResourceDefinition struct {
	v1beta1.CustomResourceDefinition `json:",inline"`
}

// Key returns an instance of CRDKey which uniquely identifies a given
// CustomResourceDefinition object.
func (crd *CustomResourceDefinition) Key() CRDKey {
	return CRDKey{
		Kind:    crd.Spec.Names.Kind,
		Name:    crd.GetName(),
		Version: crd.Spec.Version,
	}
}
