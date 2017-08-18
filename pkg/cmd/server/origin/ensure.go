package origin

import (
	"github.com/golang/glog"

	kapierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	genericapiserver "k8s.io/apiserver/pkg/server"
	kapi "k8s.io/kubernetes/pkg/api"
)

// ensureOpenShiftSharedResourcesNamespace is called as part of global policy initialization to ensure shared namespace exists
func (c *MasterConfig) ensureOpenShiftSharedResourcesNamespace(context genericapiserver.PostStartHookContext) error {
	if _, err := c.PrivilegedLoopbackKubernetesClientsetInternal.Core().Namespaces().Get(c.Options.PolicyConfig.OpenShiftSharedResourcesNamespace, metav1.GetOptions{}); kapierror.IsNotFound(err) {
		namespace, createErr := c.PrivilegedLoopbackKubernetesClientsetInternal.Core().Namespaces().Create(&kapi.Namespace{ObjectMeta: metav1.ObjectMeta{Name: c.Options.PolicyConfig.OpenShiftSharedResourcesNamespace}})
		if createErr != nil {
			glog.Errorf("Error creating namespace: %v due to %v\n", c.Options.PolicyConfig.OpenShiftSharedResourcesNamespace, createErr)
			return nil
		}

		EnsureNamespaceServiceAccountRoleBindings(c.PrivilegedLoopbackKubernetesClientsetInternal, c.PrivilegedLoopbackOpenShiftClient, namespace)
	}
	return nil
}
