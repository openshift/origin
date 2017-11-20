package origin

import (
	genericapiserver "k8s.io/apiserver/pkg/server"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

// ensureOpenShiftSharedResourcesNamespace is called as part of global policy initialization to ensure shared namespace exists
func (c *MasterConfig) ensureOpenShiftSharedResourcesNamespace(context genericapiserver.PostStartHookContext) error {
	ensureNamespaceServiceAccountRoleBindings(context, c.Options.PolicyConfig.OpenShiftSharedResourcesNamespace)
	return nil
}

// ensureOpenShiftMasterServiceAccounts creates service accounts that are required for the master API to function
func (c *MasterConfig) ensureOpenShiftMasterServiceAccounts(context genericapiserver.PostStartHookContext) error {
	return ensureNamespaceServiceAccounts(context, c.Options.PolicyConfig.OpenShiftInfrastructureNamespace, bootstrappolicy.InfraRegistryManagementControllerServiceAccountName)
}
