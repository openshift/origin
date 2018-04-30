package default_imagestreams

import (
	"github.com/openshift/origin/pkg/oc/bootstrap"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusteradd/componentinstall"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
)

const (
	rhelLocation   = "examples/image-streams/image-streams-rhel7.json"
	centosLocation = "examples/image-streams/image-streams-centos7.json"
)

type RHELImageStreamsComponentOptions struct {
	InstallContext componentinstall.Context
}

func (c *RHELImageStreamsComponentOptions) Name() string {
	return "rhel-imagestreams"
}

func (c *RHELImageStreamsComponentOptions) Install(dockerClient dockerhelper.Interface) error {
	component := componentinstall.List{
		Name:      c.Name(),
		Namespace: "openshift",
		List:      bootstrap.MustAsset(rhelLocation),
	}

	return component.MakeReady(
		c.InstallContext.ClientImage(),
		c.InstallContext.BaseDir()).Install(dockerClient)
}

type CentosImageStreamsComponentOptions struct {
	InstallContext componentinstall.Context
}

func (c *CentosImageStreamsComponentOptions) Name() string {
	return "centos-imagestreams"
}

func (c *CentosImageStreamsComponentOptions) Install(dockerClient dockerhelper.Interface) error {
	component := componentinstall.List{
		Name:      c.Name(),
		Namespace: "openshift",
		List:      bootstrap.MustAsset(centosLocation),
	}

	return component.MakeReady(
		c.InstallContext.ClientImage(),
		c.InstallContext.BaseDir()).Install(dockerClient)
}
