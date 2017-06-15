package bootstrappolicy

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/authorization"
	"k8s.io/kubernetes/pkg/apis/certificates"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationapiv1 "github.com/openshift/origin/pkg/authorization/apis/authorization/v1"
	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"

	// we need the conversions registered for our init block
	_ "github.com/openshift/origin/pkg/authorization/apis/authorization/install"
)

const (
	// The controllers below were converted to new controller initialization and use RBAC
	// rules:
	InfraOriginNamespaceServiceAccountName                      = "origin-namespace-controller"
	InfraServiceAccountControllerServiceAccountName             = "serviceaccount-controller"
	InfraServiceAccountPullSecretsControllerServiceAccountName  = "serviceaccount-pull-secrets-controller"
	InfraServiceAccountTokensControllerServiceAccountName       = "serviceaccount-tokens-controller"
	InfraServiceServingCertServiceAccountName                   = "service-serving-cert-controller"
	InfraBuildControllerServiceAccountName                      = "build-controller"
	InfraBuildConfigChangeControllerServiceAccountName          = "build-config-change-controller"
	InfraDeploymentConfigControllerServiceAccountName           = "deploymentconfig-controller"
	InfraDeploymentTriggerControllerServiceAccountName          = "deployment-trigger-controller"
	InfraDeployerControllerServiceAccountName                   = "deployer-controller"
	InfraImageTriggerControllerServiceAccountName               = "image-trigger-controller"
	InfraImageImportControllerServiceAccountName                = "image-import-controller"
	InfraSDNControllerServiceAccountName                        = "sdn-controller"
	InfraClusterQuotaReconciliationControllerServiceAccountName = "cluster-quota-reconciliation-controller"
	InfraUnidlingControllerServiceAccountName                   = "unidling-controller"
	InfraServiceIngressIPControllerServiceAccountName           = "service-ingress-ip-controller"
	InfraPersistentVolumeRecyclerControllerServiceAccountName   = "pv-recycler-controller"
	InfraResourceQuotaControllerServiceAccountName              = "resourcequota-controller"

	InfraNodeBootstrapServiceAccountName = "node-bootstrapper"
	NodeBootstrapRoleName                = "system:node-bootstrapper"

	// template instance controller watches for TemplateInstance object creation
	// and instantiates templates as a result.
	InfraTemplateInstanceControllerServiceAccountName = "template-instance-controller"

	// template service broker is an open service broker-compliant API
	// implementation which serves up OpenShift templates.  It uses the
	// TemplateInstance backend for most of the heavy lifting.
	InfraTemplateServiceBrokerServiceAccountName = "template-service-broker"
	TemplateServiceBrokerControllerRoleName      = "system:openshift:template-service-broker"

	// This is a special constant which maps to the service account name used by the underlying
	// Kubernetes code, so that we can build out the extra policy required to scale OpenShift resources.
	InfraHorizontalPodAutoscalerControllerServiceAccountName = "horizontal-pod-autoscaler"
)

type InfraServiceAccounts struct {
	serviceAccounts sets.String
	saToRole        map[string]authorizationapi.ClusterRole
}

var InfraSAs = &InfraServiceAccounts{}

func (r *InfraServiceAccounts) addServiceAccount(saName string, role authorizationapi.ClusterRole) error {
	if _, exists := r.serviceAccounts[saName]; exists {
		return fmt.Errorf("%s already registered", saName)
	}

	for existingSAName, existingRole := range r.saToRole {
		if existingRole.Name == role.Name {
			return fmt.Errorf("clusterrole/%s is already registered for %s", existingRole.Name, existingSAName)
		}
	}

	if role.Annotations == nil {
		role.Annotations = map[string]string{}
	}
	role.Annotations[roleSystemOnly] = roleIsSystemOnly

	// TODO make this unnecessary
	// we don't want to expose the resourcegroups externally because it makes it very difficult for customers to learn from
	// our default roles and hard for them to reason about what power they are granting their users
	for j := range role.Rules {
		role.Rules[j].Resources = authorizationapi.NormalizeResources(role.Rules[j].Resources)
	}

	// TODO roundtrip roles to pick up defaulting for API groups.  Without this, the covers check in reconcile-cluster-roles will fail.
	// we can remove this again once everything gets group qualified and we have unit tests enforcing that.  other pulls are in
	// progress to do that.
	// we only want to roundtrip the sa roles now.  We'll remove this once we convert the SA roles
	versionedRole := &authorizationapiv1.ClusterRole{}
	if err := kapi.Scheme.Convert(&role, versionedRole, nil); err != nil {
		return err
	}
	defaultedInternalRole := &authorizationapi.ClusterRole{}
	if err := kapi.Scheme.Convert(versionedRole, defaultedInternalRole, nil); err != nil {
		return err
	}

	r.saToRole[saName] = *defaultedInternalRole
	r.serviceAccounts.Insert(saName)
	return nil
}

func (r *InfraServiceAccounts) GetServiceAccounts() []string {
	return r.serviceAccounts.List()
}

func (r *InfraServiceAccounts) RoleFor(saName string) (authorizationapi.ClusterRole, bool) {
	ret, exists := r.saToRole[saName]
	return ret, exists
}

func (r *InfraServiceAccounts) AllRoles() []authorizationapi.ClusterRole {
	saRoles := []authorizationapi.ClusterRole{}
	for _, saName := range r.serviceAccounts.List() {
		saRoles = append(saRoles, r.saToRole[saName])
	}

	return saRoles
}

func init() {
	var err error

	InfraSAs.serviceAccounts = sets.String{}
	InfraSAs.saToRole = map[string]authorizationapi.ClusterRole{}

	err = InfraSAs.addServiceAccount(
		InfraNodeBootstrapServiceAccountName,
		authorizationapi.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: NodeBootstrapRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					APIGroups: []string{certificates.GroupName},
					// match the upstream role for now
					// TODO sort out how to deconflict this with upstream
					Verbs:     sets.NewString("create", "get", "list", "watch"),
					Resources: sets.NewString("certificatesigningrequests"),
				},
			},
		},
	)
	if err != nil {
		panic(err)
	}

	err = InfraSAs.addServiceAccount(
		InfraTemplateServiceBrokerServiceAccountName,
		authorizationapi.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: TemplateServiceBrokerControllerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					APIGroups: []string{authorization.GroupName},
					Verbs:     sets.NewString("create"),
					Resources: sets.NewString("subjectaccessreviews"),
				},
				{
					APIGroups: []string{templateapi.GroupName},
					Verbs:     sets.NewString("get", "create", "update", "delete"),
					Resources: sets.NewString("brokertemplateinstances"),
				},
				{
					APIGroups: []string{templateapi.GroupName},
					// "assign" is required for the API server to accept creation of
					// TemplateInstance objects with the requester username set to an
					// identity which is not the API caller.
					Verbs:     sets.NewString("get", "create", "delete", "assign"),
					Resources: sets.NewString("templateinstances"),
				},
				{
					APIGroups: []string{kapi.GroupName},
					Verbs:     sets.NewString("get", "list", "create", "delete"),
					Resources: sets.NewString("secrets"),
				},
				{
					APIGroups: []string{kapi.GroupName},
					Verbs:     sets.NewString("list"),
					Resources: sets.NewString("services"),
				},
				{
					APIGroups: []string{kapi.GroupName},
					Verbs:     sets.NewString("list"),
					Resources: sets.NewString("configmaps"),
				},
				{
					APIGroups: []string{routeapi.GroupName},
					Verbs:     sets.NewString("list"),
					Resources: sets.NewString("routes"),
				},
			},
		},
	)
	if err != nil {
		panic(err)
	}
}
