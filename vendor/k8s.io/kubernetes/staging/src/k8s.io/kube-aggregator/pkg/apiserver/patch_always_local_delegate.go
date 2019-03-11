package apiserver

import (
	"k8s.io/apimachinery/pkg/util/sets"
)

// alwaysLocalDelegatePrefixes specify a list of API paths that we want to delegate to Kubernetes API server
// instead of handling with OpenShift API server.
var alwaysLocalDelegatePathPrefixes = sets.NewString()

// AddAlwaysLocalDelegateForPrefix will cause the given URL prefix always be served by local API server (kube apiserver).
// This allows to move some resources from aggregated API server into CRD.
func AddAlwaysLocalDelegateForPrefix(prefix string) {
	if alwaysLocalDelegatePathPrefixes.Has(prefix) {
		return
	}
	alwaysLocalDelegatePathPrefixes.Insert(prefix)
}
