package web_console_operator

import (
	"fmt"

	"github.com/golang/glog"

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
	//kubeAdminClient, err := kubernetes.NewForConfig(c.InstallContext.ClusterAdminClientConfig())
	//if err != nil {
	//	return errors.NewError("cannot obtain API clients").WithCause(err)
	//}
	// create the actual resources required
	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = c.InstallContext.ImageFormat()
	imageTemplate.Latest = false

	params := map[string]string{
		"IMAGE":              c.InstallContext.ClientImage(),
		"LOGLEVEL":           fmt.Sprintf("%d", c.InstallContext.ComponentLogLevel()),
		"COMPONENT_IMAGE":    imageTemplate.ExpandOrDie("web-console"),
		"COMPONENT_LOGLEVEL": fmt.Sprintf("%d", c.InstallContext.ComponentLogLevel()),
		"NAMESPACE":          namespace,
	}
	glog.V(2).Infof("instantiating template service broker template with parameters %v", params)

	component := componentinstall.Template{
		Name:            "openshift-web-console-operator",
		Namespace:       namespace,
		RBACTemplate:    bootstrap.MustAsset("install/openshift-web-console-operator/install-rbac.yaml"),
		InstallTemplate: bootstrap.MustAsset("install/openshift-web-console-operator/install.yaml"),

		// TODO wait until the webconsole is up
	}

	return component.MakeReady(
		c.InstallContext.ClientImage(),
		c.InstallContext.BaseDir(),
		params).Install(dockerClient)
}
