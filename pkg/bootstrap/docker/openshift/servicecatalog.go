package openshift

import (
	"bytes"
	//	"crypto/tls"
	"fmt"
	//	"net/http"
	"time"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"

	"k8s.io/client-go/dynamic"

	aggregatorapi "k8s.io/kube-aggregator/pkg/apis/apiregistration"
	aggregatorclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/internalclientset/typed/apiregistration/internalversion"

	//	scapi "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	//	scclient "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/internalclientset/typed/servicecatalog/internalversion"

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
func (h *Helper) InstallServiceCatalog(f *clientcmd.Factory, publicMaster, catalogHost string) error {
	osClient, kubeClient, err := f.Clients()
	if err != nil {
		return errors.NewError("cannot obtain API clients").WithCause(err).WithDetails(h.OriginLog())
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
		"SERVICE_CATALOG_SERVICE_IP":     ServiceCatalogServiceIP,
		"SERVICE_CATALOG_ROUTE_HOSTNAME": catalogHost,
		"CORS_ALLOWED_ORIGIN":            publicMaster,
	}
	glog.V(2).Infof("instantiating service catalog template")

	// Stands up the service catalog apiserver, etcd, and controller manager
	err = instantiateTemplate(osClient, clientcmd.ResourceMapper(f), "openshift", catalogTemplate, catalogNamespace, params)
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
	//apiclient := apiregistrationclient.New(osClient)
	clientConfig, err := f.OpenShiftClientConfig().ClientConfig()
	if err != nil {
		return errors.NewError(fmt.Sprintf("failed to retrieve client config: %v", err))
	}

	aggregatorclient, err := aggregatorclient.NewForConfig(clientConfig)
	if err != nil {
		return errors.NewError(fmt.Sprintf("failed to create an api aggregation registration client: %v", err))
	}

	sc := &aggregatorapi.APIService{
		Spec: aggregatorapi.APIServiceSpec{
			InsecureSkipTLSVerify: true,
			Version:               "v1alpha1",
			Group:                 "servicecatalog.k8s.io",
			Priority:              200,
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
	pool := dynamic.NewDynamicClientPool(clientConfig)
	dclient, err := pool.ClientForGroupVersionResource(schema.GroupVersionResource{
		Group:    "servicecatalog.k8s.io",
		Version:  "v1alpha1",
		Resource: "broker",
	})
	if err != nil {
		return errors.NewError(fmt.Sprintf("failed to create a broker resource client: %v", err))
	}

	/*
		dclient, err := dynamic.NewClient(clientConfig)
		if err != nil {
			return errors.NewError(fmt.Sprintf("failed to create a dynamic resource client: %v", err))
		}
	*/
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

	/*
		brokerResourceDefinition := &apiextensionsv1beta1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: "broker.servicecatalog.k8s.io"},
			Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
				Group:   "servicecatalog.k8s.io",
				Version: "v1alpha1",
				Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
					Plural:   "brokers",
					Singular: "broker",
					Kind:     "Broker",
				},
				Scope: scope,
			},
		}

		apiExtensionsClient, err := clientset.NewForConfig(clientConfig)
		dynamicClient := client.Resource(&metav1.APIResource{
			Name:       definition.Spec.Names.Plural,
			Namespaced: definition.Spec.Scope == apiextensionsv1beta1.NamespaceScoped,
		}, ns)
	*/

	/*
		scclient, err := scclient.NewForConfig(clientConfig)
		if err != nil {
			return errors.NewError(fmt.Sprintf("failed to create a service catalog client: %v", err))
		}

		broker := &scclient.Broker{
			Spec: scapi.BrokerSpec{
				URL:      "https://kubernetes.default.svc:443/brokers/template.openshift.io",
				AuthInfo: nil,
			},
		}
		broker.Name = "template-broker"

		err = wait.Poll(1*time.Second, 30*time.Second, func() (bool, error) {
			_, err = scclient.Brokers().Create(broker)
			if err != nil {
				glog.V(2).Infof("retrying registration after error %v", err)
				return false, nil
			}
			return true, nil
		})
		if err != nil {
			return errors.NewError(fmt.Sprintf("failed to register broker with service catalog: %v", err))
		}
	*/

	//aggregatorclient.RESTClient().Post().AbsPath("/apis/servicecatalog.k8s.io/v1alpha1/brokers").
	/*
		insecureCli := http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}
		registerBroker := `{
		     "apiVersion": "servicecatalog.k8s.io/v1alpha1",
		     "kind": "Broker",
		     "metadata": {
		       "name": "template-broker"
		     },
		     "spec": {
		       "url": "https://kubernetes.default.svc:443/brokers/template.openshift.io"
		     }
		   }`

		glog.V(2).Infof("registering template broker with service catalog")
		err = wait.Poll(1*time.Second, 30*time.Second, func() (bool, error) {
			resp, err := insecureCli.Post(
				"https://"+catalogHost+"/apis/servicecatalog.k8s.io/v1alpha1/brokers",
				"application/json",
				bytes.NewBufferString(registerBroker),
			)
			if err != nil {
				glog.V(2).Infof("retrying registration after error %v", err)
				return false, nil
			}
			if resp == nil || (resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated) {
				glog.V(2).Infof("retrying registration after bad response %v", resp)
				return false, nil
			}
			return true, nil
		})
		if err != nil {
			return errors.NewError(fmt.Sprintf("failed to register the template broker with the service catalog: %v", err))
		}
	*/

	return nil
}

func CatalogHost(routingSuffix, serverIP string) string {
	if len(routingSuffix) > 0 {
		return fmt.Sprintf("apiserver-service-catalog.%s", routingSuffix)
	}
	return fmt.Sprintf("apiserver-service-catalog.%s.nip.io", serverIP)
}
