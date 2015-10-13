package api

import (
	kapi "k8s.io/kubernetes/pkg/api"
)

const (
	// These are internal finalizer values to Origin
	FinalizerOrigin kapi.FinalizerName = "openshift.io/origin"
)
