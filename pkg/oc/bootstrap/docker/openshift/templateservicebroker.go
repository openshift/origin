package openshift

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/oc/bootstrap"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/componentinstall"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

const (
	tsbNamespace = "openshift-template-service-broker"
)

// InstallTemplateServiceBroker checks whether the template service broker is installed and installs it if not already installed
func (h *Helper) InstallTemplateServiceBroker(clusterAdminKubeConfig []byte, f *clientcmd.Factory, imageFormat string, serverLogLevel int, logdir string) error {
	// create the actual resources required
	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = imageFormat
	imageTemplate.Latest = false

	params := map[string]string{
		"IMAGE":     imageTemplate.ExpandOrDie("template-service-broker"),
		"LOGLEVEL":  fmt.Sprint(serverLogLevel),
		"NAMESPACE": tsbNamespace,
	}
	glog.V(2).Infof("instantiating template service broker template with parameters %v", params)

	component := componentinstall.Template{
		Name:            "tsb-apiserver",
		Namespace:       tsbNamespace,
		RBACTemplate:    bootstrap.MustAsset("install/templateservicebroker/rbac-template.yaml"),
		InstallTemplate: bootstrap.MustAsset("install/templateservicebroker/apiserver-template.yaml"),

		// wait until the apiservice is ready
		WaitCondition: func() (bool, error) {
			kubeClient, err := f.ClientSet()
			if err != nil {
				utilruntime.HandleError(err)
				return false, nil
			}

			glog.V(2).Infof("polling for template service broker api server endpoint availability")
			ds, err := kubeClient.Extensions().DaemonSets(tsbNamespace).Get("apiserver", metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			if ds.Status.NumberReady > 0 {
				return true, nil
			}
			return false, nil

		},
	}

	return component.MakeReady(
		h.image,
		clusterAdminKubeConfig,
		params).Install(h.dockerHelper.Client(), logdir)
}

// RegisterTemplateServiceBroker registers the TSB with the SC by creating the broker resource
func (h *Helper) RegisterTemplateServiceBroker(clusterAdminKubeConfig []byte, configDir string, logdir string) error {
	// Register the template broker with the service catalog
	glog.V(2).Infof("registering the template broker with the service catalog")

	serviceCABytes, err := ioutil.ReadFile(filepath.Join(configDir, "master", "service-signer.crt"))
	serviceCAString := base64.StdEncoding.EncodeToString(serviceCABytes)
	if err != nil {
		return errors.NewError("unable to read service signer cert").WithCause(err)
	}
	params := map[string]string{
		"TSB_NAMESPACE": tsbNamespace,
		"CA_BUNDLE":     serviceCAString,
	}

	component := componentinstall.Template{
		Name:            "tsb-registration",
		Namespace:       tsbNamespace,
		InstallTemplate: bootstrap.MustAsset("install/service-catalog-broker-resources/template-service-broker-registration.yaml"),
	}
	return component.MakeReady(
		h.image,
		clusterAdminKubeConfig,
		params).Install(h.dockerHelper.Client(), logdir)
}
