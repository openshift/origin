package v1

import "k8s.io/kubernetes/pkg/runtime"

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	RegisterDefaults(scheme)
	return nil
}
