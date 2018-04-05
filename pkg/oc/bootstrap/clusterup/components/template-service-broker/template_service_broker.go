package template_service_broker

import (
	"fmt"
	"io/ioutil"
	"path"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kclientcmd "k8s.io/client-go/tools/clientcmd"

	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/oc/bootstrap"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/componentinstall"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/components/register-template-service-broker"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/errors"
)

const (
	TSBNamespace = "openshift-template-service-broker"
)

type TemplateServiceBrokerComponentOptions struct {
	OCImage         string
	MasterConfigDir string
	ImageFormat     string
	ServerLogLevel  int
}

func (c *TemplateServiceBrokerComponentOptions) Name() string {
	return "openshift-template-service-broker"
}

func (c *TemplateServiceBrokerComponentOptions) Install(dockerClient dockerhelper.Interface, logdir string) error {
	clusterAdminKubeConfigBytes, err := ioutil.ReadFile(path.Join(c.MasterConfigDir, "admin.kubeconfig"))
	if err != nil {
		return err
	}
	restConfig, err := kclientcmd.RESTConfigFromKubeConfig(clusterAdminKubeConfigBytes)
	if err != nil {
		return err
	}
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return errors.NewError("cannot obtain API clients").WithCause(err)
	}
	// create the actual resources required
	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = c.ImageFormat
	imageTemplate.Latest = false

	params := map[string]string{
		"IMAGE":     imageTemplate.ExpandOrDie("template-service-broker"),
		"LOGLEVEL":  fmt.Sprintf("%d", c.ServerLogLevel),
		"NAMESPACE": TSBNamespace,
	}
	glog.V(2).Infof("instantiating template service broker template with parameters %v", params)

	component := componentinstall.Template{
		Name:            "template-service-broker-apiserver",
		Namespace:       TSBNamespace,
		RBACTemplate:    bootstrap.MustAsset("install/templateservicebroker/rbac-template.yaml"),
		InstallTemplate: bootstrap.MustAsset("install/templateservicebroker/apiserver-template.yaml"),

		// wait until the apiservice is ready
		WaitCondition: func() (bool, error) {
			glog.V(2).Infof("polling for template service broker api server endpoint availability")
			ds, err := kubeClient.AppsV1().DaemonSets(TSBNamespace).Get("apiserver", metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			if ds.Status.NumberReady > 0 {
				return true, nil
			}
			return false, nil

		},
	}

	err = component.MakeReady(
		c.OCImage,
		clusterAdminKubeConfigBytes,
		params).Install(dockerClient, logdir)
	if err != nil {
		return err
	}

	// the service catalog may not be here, but as a best effort try to register
	register_template_service_broker.RegisterTemplateServiceBroker(dockerClient, c.OCImage, clusterAdminKubeConfigBytes, c.MasterConfigDir, logdir)
	return nil
}
