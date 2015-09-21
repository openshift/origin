package v1beta3

import (
	"k8s.io/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1beta3",
		&PodSecurityPolicy{},
		&PodSecurityPolicyList{},
		&SecurityContextConstraints{},
		&SecurityContextConstraintsList{},
	)
}

func (*PodSecurityPolicy) IsAnAPIObject()              {}
func (*PodSecurityPolicyList) IsAnAPIObject()          {}
func (*SecurityContextConstraints) IsAnAPIObject()     {}
func (*SecurityContextConstraintsList) IsAnAPIObject() {}
