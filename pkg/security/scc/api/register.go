package api

import (
	"k8s.io/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("",
		&SecurityContextConstraints{},
		&SecurityContextConstraintsList{},
	)
}

func (*SecurityContextConstraints) IsAnAPIObject()     {}
func (*SecurityContextConstraintsList) IsAnAPIObject() {}
