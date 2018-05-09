package web_console

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"path"

	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/oc/bootstrap"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusteradd/componentinstall"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/kubeapiserver"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/errors"
)

const (
	consoleNamespace = "openshift-web-console"
)

type WebConsoleComponentOptions struct {
	InstallContext componentinstall.Context
}

func (c *WebConsoleComponentOptions) Name() string {
	return "openshift-web-console"
}

func (c *WebConsoleComponentOptions) Install(dockerClient dockerhelper.Interface) error {
	kubeAdminClient, err := kubernetes.NewForConfig(c.InstallContext.ClusterAdminClientConfig())
	if err != nil {
		return errors.NewError("cannot obtain API clients").WithCause(err)
	}

	// parse the YAML to edit
	var consoleConfig map[string]interface{}
	if err := yaml.Unmarshal(bootstrap.MustAsset("install/origin-web-console/console-config.yaml"), &consoleConfig); err != nil {
		return errors.NewError("cannot parse web console config as YAML").WithCause(err)
	}

	// update config values
	clusterInfo, ok := consoleConfig["clusterInfo"].(map[interface{}]interface{})
	if !ok {
		return errors.NewError("cannot read clusterInfo in web console config")
	}

	masterPublicHostPort, err := getMasterPublicHostPort(c.InstallContext.BaseDir())
	if err != nil {
		return err
	}
	clusterInfo["consolePublicURL"] = "https://" + masterPublicHostPort + "/console/"

	clusterInfo["masterPublicURL"], err = getMasterPublicURL(c.InstallContext.BaseDir())
	if err != nil {
		return err
	}

	// serialize it back out as a string to use as a template parameter
	updatedConfig, err := yaml.Marshal(consoleConfig)
	if err != nil {
		return errors.NewError("cannot serialize web console config").WithCause(err)
	}

	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = c.InstallContext.ImageFormat()
	imageTemplate.Latest = false

	params := map[string]string{
		"API_SERVER_CONFIG":          string(updatedConfig),
		"OPENSHIFT_WEBCONSOLE_IMAGE": imageTemplate.ExpandOrDie("web-console"),
		"LOGLEVEL":                   fmt.Sprintf("%d", c.InstallContext.ComponentLogLevel()),
		"NAMESPACE":                  consoleNamespace,
	}

	component := componentinstall.Template{
		Name:            "webconsole",
		Namespace:       consoleNamespace,
		InstallTemplate: bootstrap.MustAsset("install/origin-web-console/console-template.yaml"),

		// wait until the webconsole is ready
		WaitCondition: func() (bool, error) {
			glog.V(2).Infof("polling for web console server availability")
			deployment, err := kubeAdminClient.AppsV1().Deployments(consoleNamespace).Get("webconsole", metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			if deployment.Status.AvailableReplicas > 0 {
				return true, nil
			}
			return false, nil
		},
	}

	// instantiate the web console template
	return component.MakeReady(
		c.InstallContext.ClientImage(),
		c.InstallContext.BaseDir(),
		params).Install(dockerClient)
}

func getMasterPublicHostPort(basedir string) (string, error) {
	masterPublicURL, err := getMasterPublicURL(basedir)
	if err != nil {
		return "", err
	}
	masterURL, err := url.Parse(masterPublicURL)
	if err != nil {
		return "", err
	}
	return masterURL.Host, nil
}

func getMasterPublicURL(basedir string) (string, error) {
	masterConfig, err := getMasterConfig(basedir)
	if err != nil {
		return "", err
	}
	return masterConfig.MasterPublicURL, nil
}

func getMasterConfig(basedir string) (*configapi.MasterConfig, error) {
	configBytes, err := ioutil.ReadFile(path.Join(basedir, kubeapiserver.KubeAPIServerDirName, "master-config.yaml"))
	if err != nil {
		return nil, err
	}
	configObj, err := runtime.Decode(configapilatest.Codec, configBytes)
	if err != nil {
		return nil, err
	}
	masterConfig, ok := configObj.(*configapi.MasterConfig)
	if !ok {
		return nil, fmt.Errorf("the %#v is not MasterConfig", configObj)
	}
	return masterConfig, nil
}
