package kubelet

import (
	"path"

	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/run"
	"github.com/openshift/origin/pkg/oc/errors"
)

type KubeletStartFlags struct {
	// ContainerBinds is a list of local/path:image/path pairs
	ContainerBinds []string
	// NodeImage is the docker image for openshift start node and the kubelet
	NodeImage       string
	Environment     []string
	UseSharedVolume bool
}

func NewKubeletStartFlags() *KubeletStartFlags {
	return &KubeletStartFlags{}
}

// MakeKubeletFlags returns the flags to start the kubelet
func (opt KubeletStartFlags) MakeKubeletFlags(dockerClient dockerhelper.Interface, basedir string) (string, error) {
	componentName := "create-kubelet-flags"
	imageRunHelper := run.NewRunHelper(dockerhelper.NewHelper(dockerClient)).New()
	glog.Infof("Running %q", componentName)

	binds := append(opt.ContainerBinds)
	env := append(opt.Environment)
	if opt.UseSharedVolume {
		env = append(env, "OPENSHIFT_CONTAINERIZED=false")
	}

	createFlagsCmd := []string{
		"start", "node",
		"--write-flags",
		"--config=/var/lib/origin/openshift.local.config/node/node-config.yaml",
	}

	_, stdout, _, rc, err := imageRunHelper.Image(opt.NodeImage).
		DiscardContainer().
		Bind(binds...).
		Env(env...).
		SaveContainerLogs(componentName, path.Join(basedir, "logs")).
		Entrypoint("openshift").
		Command(createFlagsCmd...).Output()
	if err != nil {
		return "", errors.NewError("could not run %q: %v", componentName, err).WithCause(err)
	}
	if rc != 0 {
		return "", errors.NewError("could not run %q: rc=%d", componentName, rc)
	}

	return stdout, nil
}
