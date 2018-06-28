package service_catalog

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"path"

	"github.com/golang/glog"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	aggregatorclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	rbacv1helpers "k8s.io/kubernetes/pkg/apis/rbac/v1"
	"k8s.io/kubernetes/pkg/registry/rbac/reconciliation"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/oc/clusteradd/componentinstall"
	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/kubeapiserver"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/errors"
	"github.com/openshift/origin/pkg/oc/clusterup/manifests"
)

const (
	catalogNamespace        = "kube-service-catalog"
	ServiceCatalogServiceIP = "172.30.1.2"
)

type ServiceCatalogComponentOptions struct {
	InstallContext componentinstall.Context
}

func (c *ServiceCatalogComponentOptions) Name() string {
	return "openshift-service-catalog"
}

func (c *ServiceCatalogComponentOptions) Install(dockerClient dockerhelper.Interface) error {
	kubeAdminClient, err := kubernetes.NewForConfig(c.InstallContext.ClusterAdminClientConfig())
	if err != nil {
		return err
	}

	for _, role := range getServiceCatalogClusterRoles() {
		if _, err := (&reconciliation.ReconcileRoleOptions{
			Confirm:                true,
			RemoveExtraPermissions: false,
			Role: reconciliation.ClusterRoleRuleOwner{ClusterRole: &role},
			Client: reconciliation.ClusterRoleModifier{
				Client: kubeAdminClient.RbacV1().ClusterRoles(),
			},
		}).Run(); err != nil {
			return errors.NewError("could not reconcile service catalog cluster role %s", role.Name).WithCause(err)
		}
	}

	// Instantiate service catalog
	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = c.InstallContext.ImageFormat()
	imageTemplate.Latest = false

	configBytes, err := ioutil.ReadFile(path.Join(c.InstallContext.BaseDir(), kubeapiserver.KubeAPIServerDirName, "master-config.yaml"))
	if err != nil {
		return err
	}
	configObj, err := runtime.Decode(configapilatest.Codec, configBytes)
	if err != nil {
		return err
	}
	masterConfig, ok := configObj.(*configapi.MasterConfig)
	if !ok {
		return fmt.Errorf("the %#v is not MasterConfig", configObj)
	}
	masterURL, err := url.Parse(masterConfig.MasterPublicURL)
	if err != nil {
		return err
	}

	params := map[string]string{
		"SERVICE_CATALOG_SERVICE_IP": ServiceCatalogServiceIP,
		"CORS_ALLOWED_ORIGIN":        masterURL.Hostname(),
		"SERVICE_CATALOG_IMAGE":      imageTemplate.ExpandOrDie("service-catalog"),
	}
	aggregatorClient, err := aggregatorclient.NewForConfig(c.InstallContext.ClusterAdminClientConfig())
	if err != nil {
		return err
	}
	// Stands up the service catalog apiserver, etcd, and controller manager
	component := componentinstall.Template{
		Name:            "service-catalog",
		Namespace:       catalogNamespace,
		RBACTemplate:    manifests.MustAsset("examples/service-catalog/service-catalog-rbac.yaml"),
		InstallTemplate: manifests.MustAsset("examples/service-catalog/service-catalog.yaml"),

		// Wait for the apiserver endpoint to become available
		WaitCondition: func() (bool, error) {
			glog.V(2).Infof("polling for service catalog api server endpoint availability")
			deployment, err := kubeAdminClient.AppsV1().Deployments(catalogNamespace).Get("apiserver", metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			if deployment.Status.AvailableReplicas == 0 {
				return false, nil
			}
			return componentinstall.CheckForAPIs(aggregatorClient, "v1beta1.servicecatalog.k8s.io")
		},
	}

	err = component.MakeReady(
		c.InstallContext.ClientImage(),
		c.InstallContext.BaseDir(),
		params).Install(dockerClient)

	if err != nil {
		return err
	}

	return nil
}

// TODO move to template
func getServiceCatalogClusterRoles() []rbacv1.ClusterRole {
	return []rbacv1.ClusterRole{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "system:openshift:service-catalog:aggregate-to-admin",
				Labels: map[string]string{"rbac.authorization.k8s.io/aggregate-to-admin": "true"},
			},
			Rules: []rbacv1.PolicyRule{
				rbacv1helpers.NewRule("create", "update", "delete", "get", "list", "watch", "patch").Groups("servicecatalog.k8s.io").Resources("serviceinstances", "servicebindings").RuleOrDie(),
				rbacv1helpers.NewRule("create", "update", "delete", "get", "list", "watch").Groups("settings.k8s.io").Resources("podpresets").RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "system:openshift:service-catalog:aggregate-to-edit",
				Labels: map[string]string{"rbac.authorization.k8s.io/aggregate-to-edit": "true"},
			},
			Rules: []rbacv1.PolicyRule{
				rbacv1helpers.NewRule("create", "update", "delete", "get", "list", "watch", "patch").Groups("servicecatalog.k8s.io").Resources("serviceinstances", "servicebindings").RuleOrDie(),
				rbacv1helpers.NewRule("create", "update", "delete", "get", "list", "watch").Groups("settings.k8s.io").Resources("podpresets").RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "system:openshift:service-catalog:aggregate-to-view",
				Labels: map[string]string{"rbac.authorization.k8s.io/aggregate-to-view": "true"},
			},
			Rules: []rbacv1.PolicyRule{
				rbacv1helpers.NewRule("get", "list", "watch").Groups("servicecatalog.k8s.io").Resources("serviceinstances", "servicebindings").RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "system:openshift:clusterservicebroker-client",
			},
			Rules: []rbacv1.PolicyRule{
				rbacv1helpers.NewRule("create", "update", "delete", "get", "list", "watch", "patch").Groups("servicecatalog.k8s.io").Resources("clusterservicebrokers").RuleOrDie(),
			},
		},
	}
}
