package origin

import (
	"io/ioutil"
	"regexp"
	"time"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierror "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/cmd/admin/policy"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	clusterpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
	clusterpolicystorage "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy/etcd"
	"github.com/openshift/origin/pkg/cmd/server/admin"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

// ensureOpenShiftSharedResourcesNamespace is called as part of global policy initialization to ensure shared namespace exists
func (c *MasterConfig) ensureOpenShiftSharedResourcesNamespace() {
	if _, err := c.KubeClient().Namespaces().Get(c.Options.PolicyConfig.OpenShiftSharedResourcesNamespace); kapierror.IsNotFound(err) {
		namespace, createErr := c.KubeClient().Namespaces().Create(&kapi.Namespace{ObjectMeta: kapi.ObjectMeta{Name: c.Options.PolicyConfig.OpenShiftSharedResourcesNamespace}})
		if createErr != nil {
			glog.Errorf("Error creating namespace: %v due to %v\n", c.Options.PolicyConfig.OpenShiftSharedResourcesNamespace, createErr)
			return
		}

		c.ensureNamespaceServiceAccountRoleBindings(namespace)
	}
}

// ensureOpenShiftInfraNamespace is called as part of global policy initialization to ensure infra namespace exists
func (c *MasterConfig) ensureOpenShiftInfraNamespace() {
	ns := c.Options.PolicyConfig.OpenShiftInfrastructureNamespace

	// Ensure namespace exists
	namespace, err := c.KubeClient().Namespaces().Create(&kapi.Namespace{ObjectMeta: kapi.ObjectMeta{Name: ns}})
	if kapierror.IsAlreadyExists(err) {
		// Get the persisted namespace
		namespace, err = c.KubeClient().Namespaces().Get(ns)
		if err != nil {
			glog.Errorf("Error getting namespace %s: %v", ns, err)
			return
		}
	} else if err != nil {
		glog.Errorf("Error creating namespace %s: %v", ns, err)
		return
	}

	// Ensure service accounts exist
	serviceAccounts := []string{
		c.BuildControllerServiceAccount, c.DeploymentControllerServiceAccount, c.ReplicationControllerServiceAccount,
		c.JobControllerServiceAccount, c.HPAControllerServiceAccount, c.PersistentVolumeControllerServiceAccount,
	}
	for _, serviceAccountName := range serviceAccounts {
		_, err := c.KubeClient().ServiceAccounts(ns).Create(&kapi.ServiceAccount{ObjectMeta: kapi.ObjectMeta{Name: serviceAccountName}})
		if err != nil && !kapierror.IsAlreadyExists(err) {
			glog.Errorf("Error creating service account %s/%s: %v", ns, serviceAccountName, err)
		}
	}

	// Ensure service account cluster role bindings exist
	clusterRolesToSubjects := map[string][]kapi.ObjectReference{
		bootstrappolicy.BuildControllerRoleName:            {{Namespace: ns, Name: c.BuildControllerServiceAccount, Kind: "ServiceAccount"}},
		bootstrappolicy.DeploymentControllerRoleName:       {{Namespace: ns, Name: c.DeploymentControllerServiceAccount, Kind: "ServiceAccount"}},
		bootstrappolicy.ReplicationControllerRoleName:      {{Namespace: ns, Name: c.ReplicationControllerServiceAccount, Kind: "ServiceAccount"}},
		bootstrappolicy.JobControllerRoleName:              {{Namespace: ns, Name: c.JobControllerServiceAccount, Kind: "ServiceAccount"}},
		bootstrappolicy.HPAControllerRoleName:              {{Namespace: ns, Name: c.HPAControllerServiceAccount, Kind: "ServiceAccount"}},
		bootstrappolicy.PersistentVolumeControllerRoleName: {{Namespace: ns, Name: c.PersistentVolumeControllerServiceAccount, Kind: "ServiceAccount"}},
	}
	roleAccessor := policy.NewClusterRoleBindingAccessor(c.ServiceAccountRoleBindingClient())
	for clusterRole, subjects := range clusterRolesToSubjects {
		addRole := &policy.RoleModificationOptions{
			RoleName:            clusterRole,
			RoleBindingAccessor: roleAccessor,
			Subjects:            subjects,
		}
		if err := addRole.AddRole(); err != nil {
			glog.Errorf("Could not add %v subjects to the %v cluster role: %v\n", subjects, clusterRole, err)
		} else {
			glog.V(2).Infof("Added %v subjects to the %v cluster role: %v\n", subjects, clusterRole, err)
		}
	}

	c.ensureNamespaceServiceAccountRoleBindings(namespace)
}

// ensureDefaultNamespaceServiceAccountRoles initializes roles for service accounts in the default namespace
func (c *MasterConfig) ensureDefaultNamespaceServiceAccountRoles() {
	// Wait for the default namespace
	var namespace *kapi.Namespace
	for i := 0; i < 30; i++ {
		ns, err := c.KubeClient().Namespaces().Get(kapi.NamespaceDefault)
		if err == nil {
			namespace = ns
			break
		}
		if kapierror.IsNotFound(err) {
			time.Sleep(time.Second)
			continue
		}
		glog.Errorf("Error adding service account roles to %q namespace: %v", kapi.NamespaceDefault, err)
		return
	}
	if namespace == nil {
		glog.Errorf("Namespace %q not found, could not initialize the %q namespace", kapi.NamespaceDefault, kapi.NamespaceDefault)
		return
	}

	c.ensureNamespaceServiceAccountRoleBindings(namespace)
}

// ensureNamespaceServiceAccountRoleBindings initializes roles for service accounts in the namespace
func (c *MasterConfig) ensureNamespaceServiceAccountRoleBindings(namespace *kapi.Namespace) {
	const ServiceAccountRolesInitializedAnnotation = "openshift.io/sa.initialized-roles"

	// Short-circuit if we're already initialized
	if namespace.Annotations[ServiceAccountRolesInitializedAnnotation] == "true" {
		return
	}

	hasErrors := false
	for _, binding := range bootstrappolicy.GetBootstrapServiceAccountProjectRoleBindings(namespace.Name) {
		addRole := &policy.RoleModificationOptions{
			RoleName:            binding.RoleRef.Name,
			RoleNamespace:       binding.RoleRef.Namespace,
			RoleBindingAccessor: policy.NewLocalRoleBindingAccessor(namespace.Name, c.ServiceAccountRoleBindingClient()),
			Subjects:            binding.Subjects,
		}
		if err := addRole.AddRole(); err != nil {
			glog.Errorf("Could not add service accounts to the %v role in the %q namespace: %v\n", binding.RoleRef.Name, namespace.Name, err)
			hasErrors = true
		}
	}

	// If we had errors, don't register initialization so we can try again
	if hasErrors {
		return
	}

	if namespace.Annotations == nil {
		namespace.Annotations = map[string]string{}
	}
	namespace.Annotations[ServiceAccountRolesInitializedAnnotation] = "true"
	if _, err := c.KubeClient().Namespaces().Update(namespace); err != nil {
		glog.Errorf("Error recording adding service account roles to %q namespace: %v", namespace.Name, err)
	}
}

func (c *MasterConfig) ensureDefaultSecurityContextConstraints() {
	ns := c.Options.PolicyConfig.OpenShiftInfrastructureNamespace
	bootstrapSCCGroups, bootstrapSCCUsers := bootstrappolicy.GetBoostrapSCCAccess(ns)

	for _, scc := range bootstrappolicy.GetBootstrapSecurityContextConstraints(bootstrapSCCGroups, bootstrapSCCUsers) {
		_, err := c.KubeClient().SecurityContextConstraints().Create(&scc)
		if kapierror.IsAlreadyExists(err) {
			continue
		}
		if err != nil {
			glog.Errorf("Unable to create default security context constraint %s.  Got error: %v", scc.Name, err)
			continue
		}
		glog.Infof("Created default security context constraint %s", scc.Name)
	}
}

// ensureComponentAuthorizationRules initializes the cluster policies
func (c *MasterConfig) ensureComponentAuthorizationRules() {
	clusterPolicyRegistry := clusterpolicyregistry.NewRegistry(clusterpolicystorage.NewStorage(c.EtcdHelper))
	ctx := kapi.WithNamespace(kapi.NewContext(), "")

	if _, err := clusterPolicyRegistry.GetClusterPolicy(ctx, authorizationapi.PolicyName); kapierror.IsNotFound(err) {
		glog.Infof("No cluster policy found.  Creating bootstrap policy based on: %v", c.Options.PolicyConfig.BootstrapPolicyFile)

		if err := admin.OverwriteBootstrapPolicy(c.EtcdHelper, c.Options.PolicyConfig.BootstrapPolicyFile, admin.CreateBootstrapPolicyFileFullCommand, true, ioutil.Discard); err != nil {
			glog.Errorf("Error creating bootstrap policy: %v", err)
		}

	} else {
		glog.V(2).Infof("Ignoring bootstrap policy file because cluster policy found")
	}
}

// ensureCORSAllowedOrigins takes a string list of origins and attempts to covert them to CORS origin
// regexes, or exits if it cannot.
func (c *MasterConfig) ensureCORSAllowedOrigins() []*regexp.Regexp {
	if len(c.Options.CORSAllowedOrigins) == 0 {
		return []*regexp.Regexp{}
	}
	allowedOriginRegexps, err := util.CompileRegexps(util.StringList(c.Options.CORSAllowedOrigins))
	if err != nil {
		glog.Fatalf("Invalid --cors-allowed-origins: %v", err)
	}
	return allowedOriginRegexps
}
