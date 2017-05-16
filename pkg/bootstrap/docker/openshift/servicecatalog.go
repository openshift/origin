package openshift

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

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

	err = instantiateTemplate(osClient, clientcmd.ResourceMapper(f), "openshift", catalogTemplate, catalogNamespace, params)
	if err != nil {
		return errors.NewError("cannot instantiate service catalog template").WithCause(err)
	}

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

	insecureCli := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	register := `{
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
			bytes.NewBufferString(register),
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

	return nil
}

func CatalogHost(routingSuffix, serverIP string) string {
	if len(routingSuffix) > 0 {
		return fmt.Sprintf("apiserver-service-catalog.%s", routingSuffix)
	}
	return fmt.Sprintf("apiserver-service-catalog.%s.nip.io", serverIP)
}
