package v1

import (
	v1 "github.com/openshift/api/security/v1"
	"github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/customresourcevalidation/securitycontextconstraints"
	"k8s.io/apimachinery/pkg/runtime"
)

func AddDefaultingFuncs(scheme *runtime.Scheme) error {
	RegisterDefaults(scheme)
	scheme.AddTypeDefaultingFunc(&v1.SecurityContextConstraints{}, func(obj interface{}) {
		securitycontextconstraints.SetDefaults_SCC(obj.(*v1.SecurityContextConstraints))
	})

	return nil
}
