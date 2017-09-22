package origin

import (
	genericapiserver "k8s.io/apiserver/pkg/server"
)

// ensureOpenShiftSharedResourcesNamespace is called as part of global policy initialization to ensure shared namespace exists
func (c *MasterConfig) ensureOpenShiftSharedResourcesNamespace(context genericapiserver.PostStartHookContext) error {
	ensureNamespaceServiceAccountRoleBindings(context, c.Options.PolicyConfig.OpenShiftSharedResourcesNamespace)
	return nil
}
