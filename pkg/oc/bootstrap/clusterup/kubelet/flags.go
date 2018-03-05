package kubelet

import (
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/run"
	"github.com/openshift/origin/pkg/oc/errors"
)

type KubeletStartFlags struct {
	// ContainerBinds is a list of local/path:image/path pairs
	ContainerBinds []string
	// NodeImage is the docker image for openshift start node
	NodeImage       string
	Environment     []string
	UseSharedVolume bool
}

func NewKubeletStartFlags() *KubeletStartFlags {
	return &KubeletStartFlags{}
}

// MakeKubeletFlags returns the flags to start the kubelet
func (opt KubeletStartFlags) MakeKubeletFlags(dockerClient dockerhelper.Interface) (string, error) {
	imageRunHelper := run.NewRunHelper(dockerhelper.NewHelper(dockerClient)).New()

	binds := append(opt.ContainerBinds)
	env := append(opt.Environment)
	if opt.UseSharedVolume {
		env = append(env, "OPENSHIFT_CONTAINERIZED=false")
	}

	glog.Infof("Creating initial kubelet flags\n")
	createFlagsCmd := []string{
		"start", "node",
		"--write-flags",
		"--config=/var/lib/origin/openshift.local.config/node/node-config.yaml",
	}

	_, stdout, _, _, err := imageRunHelper.Image(opt.NodeImage).
		Privileged().
		DiscardContainer().
		HostNetwork().
		HostPid().
		Bind(binds...).
		Env(env...).
		Entrypoint("openshift").
		Command(createFlagsCmd...).Output()
	if err != nil {
		return "", errors.NewError("could not create OpenShift configuration: %v", err).WithCause(err)
	}

	return stdout, nil
}
