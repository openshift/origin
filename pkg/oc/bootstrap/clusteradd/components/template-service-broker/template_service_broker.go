package template_service_broker

import (
	"fmt"
	"path"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusteradd/components/register-template-service-broker"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/kubeapiserver"
	"k8s.io/client-go/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/oc/bootstrap"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusteradd/componentinstall"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/errors"
)

const (
	tsbNamespace = "openshift-template-service-broker"
)

type TemplateServiceBrokerComponentOptions struct {
	InstallContext componentinstall.Context
}

func (c *TemplateServiceBrokerComponentOptions) Name() string {
	return "openshift-template-service-broker"
}

func (c *TemplateServiceBrokerComponentOptions) Install(dockerClient dockerhelper.Interface) error {
	kubeAdminClient, err := kubernetes.NewForConfig(c.InstallContext.ClusterAdminClientConfig())
	if err != nil {
		return errors.NewError("cannot obtain API clients").WithCause(err)
	}
	// create the actual resources required
	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = c.InstallContext.ImageFormat()
	imageTemplate.Latest = false

	params := map[string]string{
		"OPENSHIFT_TSB_IMAGE": imageTemplate.ExpandOrDie("template-service-broker"),
		"LOGLEVEL":            fmt.Sprintf("%d", c.InstallContext.ComponentLogLevel()),
		"NAMESPACE":           tsbNamespace,
	}
	glog.V(2).Infof("instantiating template service broker template with parameters %v", params)

	component := componentinstall.Template{
		Name:            "template-service-broker-apiserver",
		Namespace:       tsbNamespace,
		RBACTemplate:    bootstrap.MustAsset("install/templateservicebroker/rbac-template.yaml"),
		InstallTemplate: bootstrap.MustAsset("install/templateservicebroker/apiserver-template.yaml"),

		// wait until the apiservice is ready
		WaitCondition: func() (bool, error) {
			glog.V(2).Infof("polling for template service broker api server endpoint availability")
			ds, err := kubeAdminClient.AppsV1().DaemonSets(tsbNamespace).Get("apiserver", metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			if ds.Status.NumberAvailable > 0 {
				return true, nil
			}
			return false, nil

		},
	}

	err = component.MakeReady(
		c.InstallContext.ClientImage(),
		c.InstallContext.BaseDir(),
		params).Install(dockerClient)

	if err != nil {
		return err
	}

	masterConfigDir := path.Join(c.InstallContext.BaseDir(), kubeapiserver.KubeAPIServerDirName)
	// the service catalog may not be here, but as a best effort try to register
	register_template_service_broker.RegisterTemplateServiceBroker(
		dockerClient,
		c.InstallContext.ImageFormat(),
		c.InstallContext.BaseDir(),
		masterConfigDir,
	)
	return nil
}
