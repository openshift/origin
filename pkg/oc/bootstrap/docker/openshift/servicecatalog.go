package openshift

import (
	"fmt"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/registry/rbac/reconciliation"

	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/oc/bootstrap"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/componentinstall"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
)

const (
	catalogNamespace        = "kube-service-catalog"
	ServiceCatalogServiceIP = "172.30.1.2"
)

// InstallServiceCatalog checks whether the service catalog is installed and installs it if not already installed
func (h *Helper) InstallServiceCatalog(clusterAdminKubeConfigBytes []byte, configDir, publicMaster, catalogHost, imageFormat, logdir string) error {
	restConfig, err := kclientcmd.RESTConfigFromKubeConfig(clusterAdminKubeConfigBytes)
	if err != nil {
		return err
	}
	kubeClient, err := kclientset.NewForConfig(restConfig)
	if err != nil {
		return errors.NewError("cannot obtain API clients").WithCause(err)
	}

	for _, role := range GetServiceCatalogClusterRoles() {
		if _, err := (&reconciliation.ReconcileRoleOptions{
			Confirm:                true,
			RemoveExtraPermissions: false,
			Role: reconciliation.ClusterRoleRuleOwner{ClusterRole: &role},
			Client: reconciliation.ClusterRoleModifier{
				Client: kubeClient.Rbac().ClusterRoles(),
			},
		}).Run(); err != nil {
			return errors.NewError("could not reconcile service catalog cluster role %s", role.Name).WithCause(err)
		}
	}

	// Instantiate service catalog
	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = imageFormat
	imageTemplate.Latest = false

	params := map[string]string{
		"SERVICE_CATALOG_SERVICE_IP": ServiceCatalogServiceIP,
		"CORS_ALLOWED_ORIGIN":        publicMaster,
		"SERVICE_CATALOG_IMAGE":      imageTemplate.ExpandOrDie("service-catalog"),
	}

	// Stands up the service catalog apiserver, etcd, and controller manager
	component := componentinstall.Template{
		Name:            "service-catalog",
		Namespace:       catalogNamespace,
		RBACTemplate:    bootstrap.MustAsset("examples/service-catalog/service-catalog-rbac.yaml"),
		InstallTemplate: bootstrap.MustAsset("examples/service-catalog/service-catalog.yaml"),

		// Wait for the apiserver endpoint to become available
		WaitCondition: func() (bool, error) {
			glog.V(2).Infof("polling for service catalog api server endpoint availability")
			deployment, err := kubeClient.Extensions().Deployments(catalogNamespace).Get("apiserver", metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			if deployment.Status.AvailableReplicas == 0 {
				return false, nil
			}
			return componentinstall.CheckForAPIs(restConfig, "v1beta1.servicecatalog.k8s.io")
		},
	}

	// instantiate the web console template
	err = component.MakeReady(
		h.image,
		clusterAdminKubeConfigBytes,
		params).Install(h.dockerHelper.Client(), logdir)
	if err != nil {
		return errors.NewError(fmt.Sprintf("failed to start the service catalog apiserver: %v", err))
	}

	return nil
}

func CatalogHost(routingSuffix string) string {
	return fmt.Sprintf("apiserver-service-catalog.%s", routingSuffix)
}
