package openshift

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/golang/glog"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/oc/bootstrap"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

const (
	consoleNamespace             = "openshift-web-console"
	consoleAPIServerTemplateName = "openshift-web-console"
	consoleAssetConfigFile       = "install/origin-web-console/console-config.yaml"
)

// InstallWebConsole installs the web console server into the openshift-web-console namespace and waits for it to become ready
func (h *Helper) InstallWebConsole(f *clientcmd.Factory, imageFormat string, serverLogLevel int, publicURL string, masterURL string, loggingURL string, metricsURL string) error {
	kubeClient, err := f.ClientSet()
	if err != nil {
		return errors.NewError("cannot obtain API clients").WithCause(err).WithDetails(h.OriginLog())
	}
	templateClient, err := f.OpenshiftInternalTemplateClient()
	if err != nil {
		return err
	}

	// create the namespace if needed.  This is a reserved namespace, so you can't do it with the create project request
	if _, err := kubeClient.Core().Namespaces().Create(&kapi.Namespace{ObjectMeta: metav1.ObjectMeta{Name: consoleNamespace}}); err != nil && !kapierrors.IsAlreadyExists(err) {
		return errors.NewError("cannot create web console project").WithCause(err)
	}

	// read in the asset config YAML file like the installer
	assetConfigYaml, err := bootstrap.Asset(consoleAssetConfigFile)
	if err != nil {
		return errors.NewError("cannot read web console asset config file").WithCause(err)
	}

	// prase the YAML to edit
	var assetConfig map[string]interface{}
	if err := yaml.Unmarshal(assetConfigYaml, &assetConfig); err != nil {
		return errors.NewError("cannot parse web console asset config as YAML").WithCause(err)
	}

	// update asset config values
	assetConfig["publicURL"] = publicURL
	assetConfig["masterPublicURL"] = masterURL
	if len(loggingURL) > 0 {
		assetConfig["loggingPublicURL"] = loggingURL
	}
	if len(metricsURL) > 0 {
		assetConfig["metricsPublicURL"] = metricsURL
	}

	// serialize it back out as a string to use as a template parameter
	updatedAssetConfig, err := yaml.Marshal(assetConfig)
	if err != nil {
		return errors.NewError("cannot serialize web console asset config").WithCause(err)
	}

	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = imageFormat
	imageTemplate.Latest = false

	params := map[string]string{
		"API_SERVER_CONFIG": string(updatedAssetConfig),
		"IMAGE":             imageTemplate.ExpandOrDie("web-console"),
		"LOGLEVEL":          fmt.Sprint(serverLogLevel),
		"NAMESPACE":         consoleNamespace,
	}
	glog.V(2).Infof("instantiating web console template with parameters %v", params)

	// instantiate the web console template
	if err = instantiateTemplate(templateClient.Template(), f, OpenshiftInfraNamespace, consoleAPIServerTemplateName, consoleNamespace, params, true); err != nil {
		return errors.NewError("cannot instantiate web console template").WithCause(err)
	}

	// wait for the apiserver endpoint to become available
	err = wait.Poll(1*time.Second, 10*time.Minute, func() (bool, error) {
		glog.V(2).Infof("polling for web console server availability")
		ds, err := kubeClient.Extensions().Deployments(consoleNamespace).Get("webconsole", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if ds.Status.ReadyReplicas > 0 {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return errors.NewError(fmt.Sprintf("failed to start the web console server: %v", err))
	}

	return nil
}
