package openshift

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/golang/glog"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	aggregatorapi "k8s.io/kube-aggregator/pkg/apis/apiregistration"
	aggregatorclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/internalclientset/typed/apiregistration/internalversion"
	kapi "k8s.io/kubernetes/pkg/api"

	authzapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
)

const (
	catalogNamespace        = "kube-service-catalog"
	catalogService          = "service-catalog"
	catalogTemplate         = "service-catalog"
	ServiceCatalogServiceIP = "172.30.1.2"
)

// InstallServiceCatalog checks whether the service catalog is installed and installs it if not already installed
func (h *Helper) InstallServiceCatalog(f *clientcmd.Factory, configDir, publicMaster, catalogHost string, imageFormat string) error {
	kubeClient, err := f.ClientSet()
	if err != nil {
		return errors.NewError("cannot obtain API clients").WithCause(err).WithDetails(h.OriginLog())
	}
	authClient, err := f.OpenshiftInternalAuthorizationClient()
	if err != nil {
		return err
	}
	templateClient, err := f.OpenshiftInternalTemplateClient()
	if err != nil {
		return err
	}

	scRule, err := authzapi.NewRule("create", "update", "delete", "get", "list", "watch").Groups("servicecatalog.k8s.io").Resources("serviceinstances", "serviceinstancecredentials").Rule()
	podpresetRule, err := authzapi.NewRule("create", "update", "delete", "get", "list", "watch").Groups("settings.k8s.io").Resources("podpresets").Rule()
	if err != nil {
		return errors.NewError("could not create service catalog resource rule").WithCause(err)
	}

	editRole, err := authClient.Authorization().ClusterRoles().Get("edit", metav1.GetOptions{})
	if err != nil {
		return errors.NewError("could not get cluster edit role for patching").WithCause(err).WithDetails(h.OriginLog())
	}

	// Grant all users with the edit role, the ability to manage podpresets and service catalog instances/bindings
	editRole.Rules = append(editRole.Rules, scRule, podpresetRule)
	_, err = authClient.Authorization().ClusterRoles().Update(editRole)
	if err != nil {
		return errors.NewError("could not update the cluster edit role to add service catalog resource permissions").WithCause(err).WithDetails(h.OriginLog())
	}

	adminRole, err := authClient.Authorization().ClusterRoles().Get("admin", metav1.GetOptions{})
	if err != nil {
		return errors.NewError("could not get cluster admin role for patching").WithCause(err).WithDetails(h.OriginLog())
	}

	// Grant all users with the admin role, the ability to manage podpresets and service catalog instances/bindings
	adminRole.Rules = append(adminRole.Rules, scRule, podpresetRule)
	_, err = authClient.Authorization().ClusterRoles().Update(adminRole)
	if err != nil {
		return errors.NewError("could not update the cluster admin role to add service catalog resource permissions").WithCause(err).WithDetails(h.OriginLog())
	}

	// create the namespace if needed.  This is a reserved namespace, so you can't do it with the create project request
	if _, err := kubeClient.Core().Namespaces().Create(&kapi.Namespace{ObjectMeta: metav1.ObjectMeta{Name: catalogNamespace}}); err != nil && !kapierrors.IsAlreadyExists(err) {
		return errors.NewError("cannot create service catalog project").WithCause(err)
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
	glog.V(2).Infof("instantiating service catalog template with parameters %v", params)

	// Stands up the service catalog apiserver, etcd, and controller manager
	err = instantiateTemplate(templateClient.Template(), clientcmd.ResourceMapper(f), nil, OpenshiftInfraNamespace, catalogTemplate, catalogNamespace, params, true)
	if err != nil {
		return errors.NewError("cannot instantiate service catalog template").WithCause(err)
	}

	// Wait for the apiserver endpoint to become available
	err = wait.Poll(1*time.Second, 600*time.Second, func() (bool, error) {
		glog.V(2).Infof("polling for service catalog api server endpoint availability")
		deployment, err := kubeClient.Extensions().Deployments(catalogNamespace).Get("apiserver", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if deployment.Status.AvailableReplicas > 0 {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return errors.NewError(fmt.Sprintf("failed to start the service catalog apiserver: %v", err))
	}

	// Register the service catalog api server w/ the api aggregator
	glog.V(2).Infof("setting up the api aggregator")
	clientConfig, err := f.OpenShiftClientConfig().ClientConfig()
	if err != nil {
		return errors.NewError(fmt.Sprintf("failed to retrieve client config: %v", err))
	}

	aggregatorclient, err := aggregatorclient.NewForConfig(clientConfig)
	if err != nil {
		return errors.NewError(fmt.Sprintf("failed to create an api aggregation registration client: %v", err))
	}

	serviceCA, err := ioutil.ReadFile(filepath.Join(configDir, "master", "service-signer.crt"))
	if err != nil {
		return errors.NewError(fmt.Sprintf("failed to read the service certificate signer CA bundle: %v", err))
	}

	sc := &aggregatorapi.APIService{
		Spec: aggregatorapi.APIServiceSpec{
			CABundle:             serviceCA,
			Version:              "v1alpha1",
			Group:                "servicecatalog.k8s.io",
			GroupPriorityMinimum: 200,
			VersionPriority:      20,
			Service: &aggregatorapi.ServiceReference{
				Name:      "apiserver",
				Namespace: catalogNamespace,
			},
		},
	}
	sc.Name = "v1alpha1.servicecatalog.k8s.io"

	_, err = aggregatorclient.APIServices().Create(sc)
	if err != nil {
		return errors.NewError(fmt.Sprintf("failed to register service catalog with api aggregator: %v", err))
	}

	return nil
}

func CatalogHost(routingSuffix, serverIP string) string {
	if len(routingSuffix) > 0 {
		return fmt.Sprintf("apiserver-service-catalog.%s", routingSuffix)
	}
	return fmt.Sprintf("apiserver-service-catalog.%s.nip.io", serverIP)
}
