package web_console_operator

import (
	"fmt"

	"github.com/golang/glog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/oc/bootstrap"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusteradd/componentinstall"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
)

const (
	namespace = "openshift-core-operators"
)

type WebConsoleOperatorComponentOptions struct {
	InstallContext componentinstall.Context
}

func (c *WebConsoleOperatorComponentOptions) Name() string {
	return "openshift-web-console-operator"
}

func (c *WebConsoleOperatorComponentOptions) Install(dockerClient dockerhelper.Interface) error {
	kubeAdminClient, err := kubernetes.NewForConfig(c.InstallContext.ClusterAdminClientConfig())
	if err != nil {
		return err
	}

	// create the actual resources required
	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = c.InstallContext.ImageFormat()
	imageTemplate.Latest = false

	params := map[string]string{
		"IMAGE":                 imageTemplate.ExpandOrDie("hypershift"),
		"LOGLEVEL":              fmt.Sprintf("%d", c.InstallContext.ComponentLogLevel()),
		"COMPONENT_IMAGE":       imageTemplate.ExpandOrDie("web-console"),
		"COMPONENT_LOGLEVEL":    fmt.Sprintf("%d", c.InstallContext.ComponentLogLevel()),
		"NAMESPACE":             namespace,
		"OPENSHIFT_PULL_POLICY": c.InstallContext.ImagePullPolicy(),
	}

	glog.V(2).Infof("instantiating webconsole-operator template with parameters %v", params)

	component := componentinstall.Template{
		Name:            "openshift-web-console-operator",
		Namespace:       namespace,
		RBACTemplate:    bootstrap.MustAsset("install/openshift-web-console-operator/install-rbac.yaml"),
		InstallTemplate: bootstrap.MustAsset("install/openshift-web-console-operator/install.yaml"),

		// wait until the webconsole to an available endpoint
		WaitCondition: func() (bool, error) {
			glog.V(2).Infof("polling for web-console availability ...")
			deployment, err := kubeAdminClient.AppsV1().Deployments("openshift-web-console").Get("webconsole", metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			if deployment.Status.AvailableReplicas == 0 {
				return false, nil
			}
			return true, nil
		},
	}

	return component.MakeReady(
		c.InstallContext.ClientImage(),
		c.InstallContext.BaseDir(),
		params).Install(dockerClient)
}
