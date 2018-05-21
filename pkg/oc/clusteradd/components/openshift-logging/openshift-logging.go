package openshift_logging

import (
	"encoding/base64"
	"fmt"
	"github.com/golang/glog"
	"io/ioutil"
	"net/url"
	"path"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	kerrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/origin/pkg/oc/clusteradd/componentinstall"
	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/kubeapiserver"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/clusterup/manifests"
	"github.com/openshift/origin/pkg/oc/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

const (
	olNamespace = "openshift-logging"
)

type OpenshiftLoggingComponentOptions struct {
	InstallContext componentinstall.Context
}

func (c *OpenshiftLoggingComponentOptions) Name() string {
	return "openshift-logging"
}

func (c *OpenshiftLoggingComponentOptions) Install(dockerClient dockerhelper.Interface) error {
	adminKubeConfigBytes, err := ioutil.ReadFile(path.Join(c.InstallContext.BaseDir(), kubeapiserver.KubeAPIServerDirName, "admin.kubeconfig"))
	adminKubeConfigString := base64.StdEncoding.EncodeToString(adminKubeConfigBytes)

	configBytes, err := ioutil.ReadFile(path.Join(c.InstallContext.BaseDir(), kubeapiserver.KubeAPIServerDirName, "master-config.yaml"))
	if err != nil {
		return err
	}
	configObj, err := runtime.Decode(configapilatest.Codec, configBytes)
	if err != nil {
		return err
	}
	masterConfig, ok := configObj.(*configapi.MasterConfig)
	if !ok {
		return fmt.Errorf("the %#v is not MasterConfig", configObj)
	}
	masterUrl, err := url.Parse(masterConfig.MasterPublicURL)
	if err != nil {
		return err
	}

	kubeAdminClient, err := kubernetes.NewForConfig(c.InstallContext.ClusterAdminClientConfig())
	if err != nil {
		return errors.NewError("cannot obtain API clients").WithCause(err)
	}

	params := map[string]string{
		"NAMESPACE":        olNamespace,
		"PUBLIC_HOSTNAME":  masterUrl.Hostname(),
		"ADMIN_KUBECONFIG": adminKubeConfigString,
	}
	glog.V(2).Infof("instantiating openshift logging template with parameters %v", params)

	err = kubeAdminClient.BatchV1().Jobs(olNamespace).Delete("openshift-logging-apb", metav1.NewDeleteOptions(0))
	if err != nil && !kerrs.IsNotFound(err) {
		return err
	}

	component := componentinstall.Template{
		Name:            "openshift-logging",
		Namespace:       olNamespace,
		InstallTemplate: manifests.MustAsset("install/openshift-logging/install.yaml"),

		WaitCondition: func() (bool, error) {
			glog.V(2).Infof("polling for openshift-logging-apb to complete")
			job, err := kubeAdminClient.BatchV1().Jobs(olNamespace).Get("openshift-logging-apb", metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			if job.Status.Succeeded > 0 {
				return true, nil
			}
			return false, nil

		},
	}

	return component.MakeReady(
		c.InstallContext.ClientImage(),
		c.InstallContext.BaseDir(),
		params).Install(dockerClient)

}
