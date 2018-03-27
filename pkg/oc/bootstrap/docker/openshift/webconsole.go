package openshift

import (
	"fmt"

	"gopkg.in/yaml.v2"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/oc/bootstrap"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/componentinstall"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
)

const (
	consoleNamespace  = "openshift-web-console"
	consoleConfigFile = "install/origin-web-console/console-config.yaml"
)

// InstallWebConsole installs the web console server into the openshift-web-console namespace and waits for it to become ready
func (h *Helper) InstallWebConsole(clusterAdminKubeConfigBytes []byte, imageFormat string, serverLogLevel int, publicURL, masterURL, loggingURL, metricsURL, logdir string) error {
	restConfig, err := kclientcmd.RESTConfigFromKubeConfig(clusterAdminKubeConfigBytes)
	if err != nil {
		return err
	}
	kubeClient, err := kclientset.NewForConfig(restConfig)
	if err != nil {
		return errors.NewError("cannot obtain API clients").WithCause(err)
	}

	// parse the YAML to edit
	var consoleConfig map[string]interface{}
	if err := yaml.Unmarshal(bootstrap.MustAsset(consoleConfigFile), &consoleConfig); err != nil {
		return errors.NewError("cannot parse web console config as YAML").WithCause(err)
	}

	// update config values
	clusterInfo, ok := consoleConfig["clusterInfo"].(map[interface{}]interface{})
	if !ok {
		return errors.NewError("cannot read clusterInfo in web console config")
	}

	clusterInfo["consolePublicURL"] = publicURL
	clusterInfo["masterPublicURL"] = masterURL
	if len(loggingURL) > 0 {
		clusterInfo["loggingPublicURL"] = loggingURL
	}
	if len(metricsURL) > 0 {
		clusterInfo["metricsPublicURL"] = metricsURL
	}

	// serialize it back out as a string to use as a template parameter
	updatedConfig, err := yaml.Marshal(consoleConfig)
	if err != nil {
		return errors.NewError("cannot serialize web console config").WithCause(err)
	}

	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = imageFormat
	imageTemplate.Latest = false

	params := map[string]string{
		"API_SERVER_CONFIG": string(updatedConfig),
		"IMAGE":             imageTemplate.ExpandOrDie("web-console"),
		"LOGLEVEL":          fmt.Sprint(serverLogLevel),
		"NAMESPACE":         consoleNamespace,
	}

	component := componentinstall.Template{
		Name:            "webconsole",
		Namespace:       consoleNamespace,
		InstallTemplate: bootstrap.MustAsset("install/origin-web-console/console-template.yaml"),

		// wait until the webconsole is ready
		WaitCondition: func() (bool, error) {
			glog.V(2).Infof("polling for web console server availability")
			ds, err := kubeClient.Extensions().Deployments(consoleNamespace).Get("webconsole", metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			if ds.Status.ReadyReplicas > 0 {
				return true, nil
			}
			return false, nil
		},
	}

	// instantiate the web console template
	return component.MakeReady(
		h.image,
		clusterAdminKubeConfigBytes,
		params).Install(h.dockerHelper.Client(), logdir)
}
