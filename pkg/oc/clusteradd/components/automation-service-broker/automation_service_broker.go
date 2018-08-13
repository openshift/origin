package automation_service_broker

import (
	"strings"

	"github.com/golang/glog"

	kerrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/origin/pkg/oc/clusteradd/componentinstall"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/clusterup/manifests"
	"github.com/openshift/origin/pkg/oc/lib/errors"
	"github.com/openshift/origin/pkg/version"
	"k8s.io/client-go/kubernetes"
)

const (
	asbNamespace = "openshift-automation-service-broker"
)

type AutomationServiceBrokerComponentOptions struct {
	InstallContext componentinstall.Context
}

func (c *AutomationServiceBrokerComponentOptions) Name() string {
	return "automation-service-broker"
}

func (c *AutomationServiceBrokerComponentOptions) Install(dockerClient dockerhelper.Interface) error {
	kubeAdminClient, err := kubernetes.NewForConfig(c.InstallContext.ClusterAdminClientConfig())
	if err != nil {
		return errors.NewError("cannot obtain API clients").WithCause(err)
	}

	params := map[string]string{
		"NAMESPACE": asbNamespace,
		"TAG":       strings.TrimRight("v"+version.Get().Major+"."+version.Get().Minor, "+"),
	}
	glog.V(2).Infof("instantiating automation service broker template with parameters %v", params)

	err = kubeAdminClient.BatchV1().Jobs(asbNamespace).Delete("automation-broker-apb", metav1.NewDeleteOptions(0))
	if err != nil && !kerrs.IsNotFound(err) {
		return err
	}

	component := componentinstall.Template{
		Name:            "automation-service-broker",
		Namespace:       asbNamespace,
		RBACTemplate:    manifests.MustAsset("install/automationservicebroker/install-rbac.yaml"),
		InstallTemplate: manifests.MustAsset("install/automationservicebroker/install.yaml"),

		WaitCondition: func() (bool, error) {
			glog.V(2).Infof("polling for automation broker apb to complete")
			job, err := kubeAdminClient.BatchV1().Jobs(asbNamespace).Get("automation-broker-apb", metav1.GetOptions{})
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
