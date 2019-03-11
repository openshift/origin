package apiserver

import (
	"k8s.io/apimachinery/pkg/util/sets"
)

// alwaysDelegatePrefixes specify a list of API paths that we want to delegate to Kubernetes API server
// instead of handling with OpenShift API server.
// Usually this means that these resources were moved from OpenShift API into CRD.
var alwaysDelegatePathPrefixes = sets.NewString(
	"/apis/quota.openshift.io/v1/clusterresourcequotas",
	)
