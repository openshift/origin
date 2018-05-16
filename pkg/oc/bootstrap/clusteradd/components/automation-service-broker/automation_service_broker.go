package automation_service_broker

import (
	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/origin/pkg/oc/bootstrap"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusteradd/componentinstall"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/errors"
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
	}
	glog.V(2).Infof("instantiating automation service broker template with parameters %v", params)

	component := componentinstall.Template{
		Name:            "automation-service-broker",
		Namespace:       asbNamespace,
		RBACTemplate:    bootstrap.MustAsset("install/automationservicebroker/install-rbac.yaml"),
		InstallTemplate: bootstrap.MustAsset("install/automationservicebroker/install.yaml"),

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
