package openshift

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	aggregatorapi "k8s.io/kube-aggregator/pkg/apis/apiregistration"
	aggregatorclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/internalclientset/typed/apiregistration/internalversion"

	authzapi "github.com/openshift/origin/pkg/authorization/apis/authorization"

	"github.com/openshift/origin/pkg/bootstrap/docker/errors"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	catalogNamespace        = "service-catalog"
	catalogService          = "service-catalog"
	catalogTemplate         = "service-catalog"
	ServiceCatalogServiceIP = "172.30.1.2"
)

// InstallServiceCatalog checks whether the service catalog is installed and installs it if not already installed
func (h *Helper) InstallServiceCatalog(f *clientcmd.Factory, configDir, publicMaster, catalogHost string, tag string) error {
	osClient, kubeClient, err := f.Clients()
	if err != nil {
		return errors.NewError("cannot obtain API clients").WithCause(err).WithDetails(h.OriginLog())
	}

	// Grant all users with the edit role, the ability to manage service catalog instance/binding resources.
	scRule, err := authzapi.NewRule("create", "update", "delete", "get", "list", "watch").Groups("servicecatalog.k8s.io").Resources("instances", "bindings").Rule()
	podpresetRule, err := authzapi.NewRule("create", "update", "delete", "get", "list", "watch").Groups("settings.k8s.io").Resources("podpresets").Rule()
	if err != nil {
		return errors.NewError("could not create service catalog resource rule").WithCause(err)
	}

	editRole, err := osClient.ClusterRoles().Get("edit", metav1.GetOptions{})
	if err != nil {
		return errors.NewError("could not get cluster edit role for patching").WithCause(err).WithDetails(h.OriginLog())
	}

	editRole.Rules = append(editRole.Rules, scRule, podpresetRule)
	_, err = osClient.ClusterRoles().Update(editRole)
	if err != nil {
		return errors.NewError("could not update the cluster edit role to add service catalog resource permissions").WithCause(err).WithDetails(h.OriginLog())
	}

	adminRole, err := osClient.ClusterRoles().Get("admin", metav1.GetOptions{})
	if err != nil {
		return errors.NewError("could not get cluster admin role for patching").WithCause(err).WithDetails(h.OriginLog())
	}

	adminRole.Rules = append(adminRole.Rules, scRule, podpresetRule)
	_, err = osClient.ClusterRoles().Update(adminRole)
	if err != nil {
		return errors.NewError("could not update the cluster admin role to add service catalog resource permissions").WithCause(err).WithDetails(h.OriginLog())
	}

	_, err = kubeClient.Core().Namespaces().Get(catalogNamespace, metav1.GetOptions{})
	if err == nil {
		// If there's no error, the catalog namespace already exists and we won't initialize it
		return nil
	}

	// Create catalog namespace
	out := &bytes.Buffer{}
	err = CreateProject(f, catalogNamespace, "", "", "oc", out)
	if err != nil {
		return errors.NewError("cannot create service catalog project").WithCause(err).WithDetails(out.String())
	}

	// Instantiate service catalog
	params := map[string]string{
		"SERVICE_CATALOG_SERVICE_IP": ServiceCatalogServiceIP,
		"CORS_ALLOWED_ORIGIN":        publicMaster,
		"SERVICE_CATALOG_TAG":        tag,
	}
	glog.V(2).Infof("instantiating service catalog template with parameters %v", params)

	// Stands up the service catalog apiserver, etcd, and controller manager
	err = instantiateTemplate(osClient, clientcmd.ResourceMapper(f), OpenshiftInfraNamespace, catalogTemplate, catalogNamespace, params)
	if err != nil {
		return errors.NewError("cannot instantiate service catalog template").WithCause(err)
	}

	// Wait for the apiserver endpoint to become available
	err = wait.Poll(5*time.Second, 600*time.Second, func() (bool, error) {
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
				Namespace: "service-catalog",
			},
		},
	}
	sc.Name = "v1alpha1.servicecatalog.k8s.io"

	_, err = aggregatorclient.APIServices().Create(sc)
	if err != nil {
		return errors.NewError(fmt.Sprintf("failed to register service catalog with api aggregator: %v", err))
	}

	// Register the template broker with the service catalog
	glog.V(2).Infof("registering the template broker with the service catalog")
	pool := dynamic.NewDynamicClientPool(clientConfig)
	dclient, err := pool.ClientForGroupVersionResource(schema.GroupVersionResource{
		Group:    "servicecatalog.k8s.io",
		Version:  "v1alpha1",
		Resource: "broker",
	})
	if err != nil {
		return errors.NewError(fmt.Sprintf("failed to create a broker resource client: %v", err))
	}

	brokerResource := &metav1.APIResource{
		Name:       "brokers",
		Namespaced: false,
		Kind:       "Broker",
		Verbs:      []string{"create"},
	}
	brokerClient := dclient.Resource(brokerResource, "")

	broker := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "servicecatalog.k8s.io/v1alpha1",
			"kind":       "Broker",
			"metadata": map[string]interface{}{
				"name": "template-broker",
			},
			"spec": map[string]interface{}{
				"url": "https://kubernetes.default.svc:443/brokers/template.openshift.io",
			},
		},
	}

	err = wait.Poll(1*time.Second, 30*time.Second, func() (bool, error) {
		_, err = brokerClient.Create(broker)

		if err != nil {
			glog.V(2).Infof("retrying registration after error %v", err)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return errors.NewError(fmt.Sprintf("failed to register broker with service catalog: %v", err))
	}

	return nil
}

func CatalogHost(routingSuffix, serverIP string) string {
	if len(routingSuffix) > 0 {
		return fmt.Sprintf("apiserver-service-catalog.%s", routingSuffix)
	}
	return fmt.Sprintf("apiserver-service-catalog.%s.nip.io", serverIP)
}
