package kubelet

import (
	"fmt"
	"io"
	"strings"

	"github.com/openshift/origin/pkg/oc/bootstrap/docker/run"
	"github.com/openshift/origin/pkg/oc/errors"
)

type KubeletRunConfig struct {
	// ContainerBinds is a list of local/path:image/path pairs
	ContainerBinds []string
	// NodeImage is the docker image for openshift start node
	NodeImage       string
	Environment     []string
	DockerRoot      string
	UseSharedVolume bool
	HostVolumesDir  string

	HTTPProxy  string
	HTTPSProxy string
	NoProxy    []string

	Args []string
}

func NewKubeletRunConfig() *KubeletRunConfig {
	return &KubeletRunConfig{
		ContainerBinds: []string{
			"/var/log:/var/log:rw",
			"/var/run:/var/run:rw",
			"/sys:/sys:rw",
			"/sys/fs/cgroup:/sys/fs/cgroup:rw",
			"/dev:/dev",
		},
	}

}

// Start starts the OpenShift master as a Docker container
// and returns a directory in the local file system where
// the OpenShift configuration has been copied
func (opt KubeletRunConfig) MakeNodeConfig(imageRunHelper *run.Runner, out io.Writer) (string, error) {
	binds := append(opt.ContainerBinds)
	env := []string{}
	if len(opt.HTTPProxy) > 0 {
		env = append(env, fmt.Sprintf("HTTP_PROXY=%s", opt.HTTPProxy))
	}
	if len(opt.HTTPSProxy) > 0 {
		env = append(env, fmt.Sprintf("HTTPS_PROXY=%s", opt.HTTPSProxy))
	}
	if len(opt.NoProxy) > 0 {
		env = append(env, fmt.Sprintf("NO_PROXY=%s", strings.Join(opt.NoProxy, ",")))
	}
	if opt.UseSharedVolume {
		binds = append(binds, fmt.Sprintf("%[1]s:%[1]s:shared", opt.HostVolumesDir))
		env = append(env, "OPENSHIFT_CONTAINERIZED=false")
	} else {
		binds = append(binds, "/:/rootfs:ro")
		binds = append(binds, fmt.Sprintf("%[1]s:%[1]s:rslave", opt.HostVolumesDir))
	}
	env = append(env, opt.Environment...)
	binds = append(binds, fmt.Sprintf("%[1]s:%[1]s", opt.DockerRoot))

	// Kubelet needs to be able to write to
	// /sys/devices/virtual/net/vethXXX/brport/hairpin_mode, so make this rw, not ro.
	binds = append(binds, "/sys/devices/virtual/net:/sys/devices/virtual/net:rw")

	fmt.Fprintf(out, "Running kubelet\n")
	createConfigCmd := []string{
		"kubelet",
	}
	createConfigCmd = append(createConfigCmd, opt.Args...)

	_, err := imageRunHelper.Image(opt.NodeImage).
		Name("origin"). // TODO figure out why the rest of cluster-up relies on this name
		Privileged().
		DiscardContainer().
		HostNetwork().
		HostPid().
		Bind(binds...).
		Env(env...).
		Entrypoint("hyperkube").
		Command(createConfigCmd...).Start()
	if err != nil {
		return "", errors.NewError("could not create OpenShift configuration: %v", err).WithCause(err)
	}

	return "", nil
}
