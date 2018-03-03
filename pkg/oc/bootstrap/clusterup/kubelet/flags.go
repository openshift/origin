package kubelet

import (
	"fmt"
	"io"

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
func (opt KubeletStartFlags) MakeKubeletFlags(imageRunHelper *run.Runner, out io.Writer) (string, error) {
	binds := append(opt.ContainerBinds)
	env := append(opt.Environment)
	if opt.UseSharedVolume {
		env = append(env, "OPENSHIFT_CONTAINERIZED=false")
	}

	fmt.Fprintf(out, "Creating initial OpenShift node configuration\n")
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
