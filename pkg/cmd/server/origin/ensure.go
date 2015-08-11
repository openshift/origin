package origin

import (
	"io/ioutil"
	"regexp"
	"time"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierror "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/serviceaccount"
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
		namespace := &kapi.Namespace{
			ObjectMeta: kapi.ObjectMeta{Name: c.Options.PolicyConfig.OpenShiftSharedResourcesNamespace},
		}
		_, err = c.KubeClient().Namespaces().Create(namespace)
		if err != nil {
			glog.Errorf("Error creating namespace: %v due to %v\n", namespace, err)
		}
	}
}

// ensureOpenShiftInfraNamespace is called as part of global policy initialization to ensure infra namespace exists
func (c *MasterConfig) ensureOpenShiftInfraNamespace() {
	ns := c.Options.PolicyConfig.OpenShiftInfrastructureNamespace

	// Ensure namespace exists
	_, err := c.KubeClient().Namespaces().Create(&kapi.Namespace{ObjectMeta: kapi.ObjectMeta{Name: ns}})
	if err != nil && !kapierror.IsAlreadyExists(err) {
		glog.Errorf("Error creating namespace %s: %v", ns, err)
	}

	// Ensure service accounts exist
	serviceAccounts := []string{c.BuildControllerServiceAccount, c.DeploymentControllerServiceAccount, c.ReplicationControllerServiceAccount}
	for _, serviceAccountName := range serviceAccounts {
		_, err := c.KubeClient().ServiceAccounts(ns).Create(&kapi.ServiceAccount{ObjectMeta: kapi.ObjectMeta{Name: serviceAccountName}})
		if err != nil && !kapierror.IsAlreadyExists(err) {
			glog.Errorf("Error creating service account %s/%s: %v", ns, serviceAccountName, err)
		}
	}

	// Ensure service account cluster role bindings exist
	clusterRolesToUsernames := map[string][]string{
		bootstrappolicy.BuildControllerRoleName:       {serviceaccount.MakeUsername(ns, c.BuildControllerServiceAccount)},
		bootstrappolicy.DeploymentControllerRoleName:  {serviceaccount.MakeUsername(ns, c.DeploymentControllerServiceAccount)},
		bootstrappolicy.ReplicationControllerRoleName: {serviceaccount.MakeUsername(ns, c.ReplicationControllerServiceAccount)},
	}
	roleAccessor := policy.NewClusterRoleBindingAccessor(c.ServiceAccountRoleBindingClient())
	for clusterRole, usernames := range clusterRolesToUsernames {
		addRole := &policy.RoleModificationOptions{
			RoleName:            clusterRole,
			RoleBindingAccessor: roleAccessor,
			Users:               usernames,
		}
		if err := addRole.AddRole(); err != nil {
			glog.Errorf("Could not add %v users to the %v cluster role: %v\n", usernames, clusterRole, err)
		} else {
			glog.V(2).Infof("Added %v users to the %v cluster role: %v\n", usernames, clusterRole, err)
		}
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

// ensureDefaultNamespaceServiceAccountRoles initializes roles for service accounts in the default namespace
func (c *MasterConfig) ensureDefaultNamespaceServiceAccountRoles() {
	const ServiceAccountRolesInitializedAnnotation = "openshift.io/sa.initialized-roles"

	// Wait for the default namespace
	var defaultNamespace *kapi.Namespace
	for i := 0; i < 30; i++ {
		ns, err := c.KubeClient().Namespaces().Get(kapi.NamespaceDefault)
		if err == nil {
			defaultNamespace = ns
			break
		}
		if kapierror.IsNotFound(err) {
			time.Sleep(time.Second)
			continue
		}
		glog.Errorf("Error adding service account roles to default namespace: %v", err)
		return
	}
	if defaultNamespace == nil {
		glog.Errorf("Default namespace not found, could not initialize default service account roles")
		return
	}

	// Short-circuit if we're already initialized
	if defaultNamespace.Annotations[ServiceAccountRolesInitializedAnnotation] == "true" {
		return
	}

	hasErrors := false
	for _, binding := range bootstrappolicy.GetBootstrapServiceAccountProjectRoleBindings(kapi.NamespaceDefault) {
		addRole := &policy.RoleModificationOptions{
			RoleName:            binding.RoleRef.Name,
			RoleNamespace:       binding.RoleRef.Namespace,
			RoleBindingAccessor: policy.NewLocalRoleBindingAccessor(kapi.NamespaceDefault, c.ServiceAccountRoleBindingClient()),
			Users:               binding.Users.List(),
			Groups:              binding.Groups.List(),
		}
		if err := addRole.AddRole(); err != nil {
			glog.Errorf("Could not add service accounts to the %v role in the %v namespace: %v\n", binding.RoleRef.Name, kapi.NamespaceDefault, err)
			hasErrors = true
		}
	}

	// If we had errors, don't register initialization so we can try again
	if !hasErrors {
		if defaultNamespace.Annotations == nil {
			defaultNamespace.Annotations = map[string]string{}
		}
		defaultNamespace.Annotations[ServiceAccountRolesInitializedAnnotation] = "true"
		if _, err := c.KubeClient().Namespaces().Update(defaultNamespace); err != nil {
			glog.Errorf("Error recording adding service account roles to default namespace: %v", err)
		}
	}
}

func (c *MasterConfig) ensureDefaultSecurityContextConstraints() {
	sccList, err := c.KubeClient().SecurityContextConstraints().List(labels.Everything(), fields.Everything())
	if err != nil {
		glog.Errorf("Unable to initialize security context constraints: %v.  This may prevent the creation of pods", err)
		return
	}
	if len(sccList.Items) > 0 {
		return
	}

	glog.Infof("No security context constraints detected, adding defaults")
	ns := c.Options.PolicyConfig.OpenShiftInfrastructureNamespace
	buildControllerUsername := serviceaccount.MakeUsername(ns, c.BuildControllerServiceAccount)
	for _, scc := range bootstrappolicy.GetBootstrapSecurityContextConstraints(buildControllerUsername) {
		_, err = c.KubeClient().SecurityContextConstraints().Create(&scc)
		if err != nil {
			glog.Errorf("Unable to create default security context constraint %s.  Got error: %v", scc.Name, err)
		}
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
