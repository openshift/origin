package api

import (
	"k8s.io/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("",
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
