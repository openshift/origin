package api

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
)

// IngressAdmissionConfig is the configuration for the the ingress
// controller limiter plugin. It changes the behavior of ingress
// objects to behave better with openshift routes and routers.
// *NOTE* This has security implications in the router when handling
// ingress objects
type IngressAdmissionConfig struct {
	unversioned.TypeMeta

	// AllowHostnameChanges when false or unset openshift does not
	// allow changing or adding hostnames to ingress objects. If set
	// to true then hostnames can be added or modified which has
	// security implications in the router.
	AllowHostnameChanges bool
}
