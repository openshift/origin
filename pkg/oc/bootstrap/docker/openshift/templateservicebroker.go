package openshift

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
)

const (
	tsbNamespace             = "openshift-template-service-broker"
	tsbRBACTemplateName      = "template-service-broker-rbac"
	tsbAPIServerTemplateName = "template-service-broker-apiserver"
)

// InstallServiceCatalog checks whether the template service broker is installed and installs it if not already installed
func (h *Helper) InstallTemplateServiceBroker(f *clientcmd.Factory, imageFormat string, serverLogLevel int) error {
	osClient, kubeClient, err := f.Clients()
	if err != nil {
		return errors.NewError("cannot obtain API clients").WithCause(err).WithDetails(h.OriginLog())
	}

	// create the namespace if needed.  This is a reserved namespace, so you can't do it with the create project request
	if _, err := kubeClient.Core().Namespaces().Create(&kapi.Namespace{ObjectMeta: metav1.ObjectMeta{Name: tsbNamespace}}); err != nil && !kapierrors.IsAlreadyExists(err) {
		return errors.NewError("cannot create template service broker project").WithCause(err)
	}

	if err = instantiateTemplate(osClient, clientcmd.ResourceMapper(f), OpenshiftInfraNamespace, tsbRBACTemplateName, tsbNamespace, map[string]string{}, true); err != nil {
		return errors.NewError("cannot instantiate template service broker permissions").WithCause(err)
	}

	// create the actual resources required
	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = imageFormat
	imageTemplate.Latest = false

	if err = instantiateTemplate(osClient, clientcmd.ResourceMapper(f), OpenshiftInfraNamespace, tsbAPIServerTemplateName, tsbNamespace, map[string]string{
		"IMAGE":     imageTemplate.ExpandOrDie(""),
		"LOGLEVEL":  fmt.Sprint(serverLogLevel),
		"NAMESPACE": tsbNamespace,
	}, true); err != nil {
		return errors.NewError("cannot instantiate template service broker resources").WithCause(err)
	}

	// Wait for the apiserver endpoint to become available
	err = wait.Poll(1*time.Second, 10*time.Minute, func() (bool, error) {
		glog.V(2).Infof("polling for template service broker api server endpoint availability")
		ds, err := kubeClient.Extensions().DaemonSets(tsbNamespace).Get("apiserver", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if ds.Status.NumberReady > 0 {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return errors.NewError(fmt.Sprintf("failed to start the template service broker apiserver: %v", err))
	}

	// Register the template broker with the service catalog
	glog.V(2).Infof("registering the template broker with the service catalog")
	clientConfig, err := f.OpenShiftClientConfig().ClientConfig()
	if err != nil {
		return errors.NewError(fmt.Sprintf("failed to retrieve client config: %v", err))
	}
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
				"url": "https://apiserver.openshift-template-service-broker.svc:443/brokers/template.openshift.io",
				"insecureSkipTLSVerify": true,
				"authInfo": map[string]interface{}{
					"bearer": map[string]interface{}{
						"secretRef": map[string]interface{}{
							"kind":      "Secret",
							"name":      "templateservicebroker-client",
							"namespace": tsbNamespace,
						},
					},
				},
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
