package openshift

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/golang/glog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/oc/bootstrap"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/componentinstall"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	aggregatorapiv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
)

const (
	catalogNamespace        = "kube-service-catalog"
	catalogTemplate         = "service-catalog"
	ServiceCatalogServiceIP = "172.30.1.2"
)

// InstallServiceCatalog checks whether the service catalog is installed and installs it if not already installed
func (h *Helper) InstallServiceCatalog(clusterAdminKubeConfig []byte, f *clientcmd.Factory, configDir, publicMaster, imageFormat, logdir string) error {
	// Instantiate service catalog
	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = imageFormat
	imageTemplate.Latest = false

	serviceCA, err := ioutil.ReadFile(filepath.Join(configDir, "master", "service-signer.crt"))
	if err != nil {
		return errors.NewError(fmt.Sprintf("failed to read the service certificate signer CA bundle: %v", err))
	}

	params := map[string]string{
		"SERVICE_CATALOG_SERVICE_IP": ServiceCatalogServiceIP,
		"CORS_ALLOWED_ORIGIN":        publicMaster,
		"SERVICE_CATALOG_IMAGE":      imageTemplate.ExpandOrDie("service-catalog"),
		"CA_BUNDLE":                  base64.StdEncoding.EncodeToString(serviceCA),
	}
	glog.V(2).Infof("instantiating service catalog template with parameters %v", params)

	component := componentinstall.Template{
		Name:            "service-catalog",
		Namespace:       catalogNamespace,
		RBACTemplate:    bootstrap.MustAsset("install/service-catalog/rbac-template.yaml"),
		InstallTemplate: bootstrap.MustAsset("install/service-catalog/service-catalog-template.yaml"),
		WaitCondition: func() (bool, error) {
			kubeClient, err := f.ClientSet()
			if err != nil {
				utilruntime.HandleError(err)
				return false, nil
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
				return false, err
			}

			clientConfig, err := f.OpenShiftClientConfig().ClientConfig()
			if err != nil {
				return false, err
			}

			err = componentinstall.WaitForAPI(clientConfig, func(apiService aggregatorapiv1beta1.APIService) bool {
				return apiService.Name == "v1beta1.servicecatalog.k8s.io"
			})
			if err != nil {
				return false, err
			}

			return true, nil
		},
	}

	return component.MakeReady(
		h.image,
		clusterAdminKubeConfig,
		params).Install(h.dockerHelper.Client(), logdir)
}
