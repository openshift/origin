package register_template_service_broker

import (
	"encoding/base64"
	"io/ioutil"
	"path/filepath"

	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/oc/bootstrap"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusteradd/componentinstall"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
)

const (
	tsbNamespace = "openshift-template-service-broker"
)

// TODO this should be a controller based on the actual cluster state
// RegisterTemplateServiceBroker registers the TSB with the SC by creating the broker resource
func RegisterTemplateServiceBroker(dockerClient dockerhelper.Interface, ocImage, baseDir, configDir string) error {
	// Register the template broker with the service catalog
	glog.V(2).Infof("registering the template broker with the service catalog")

	serviceCABytes, err := ioutil.ReadFile(filepath.Join(configDir, "service-signer.crt"))
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
		ocImage,
		baseDir,
		params).Install(dockerClient)

}
