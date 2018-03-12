package kubelet

import (
	"fmt"
	"strings"

	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/openshift"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/run"
	"github.com/openshift/origin/pkg/oc/errors"
)

type KubeletRunConfig struct {
	// ContainerBinds is a list of local/path:image/path pairs
	ContainerBinds []string
	// NodeImage is the docker image for openshift start node
	NodeImage   string
	Environment []string

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
func (opt KubeletRunConfig) StartKubelet(dockerClient dockerhelper.Interface, logdir string) (string, error) {
	componentName := "start-kubelet"
	imageRunHelper := run.NewRunHelper(dockerhelper.NewHelper(dockerClient)).New()
	glog.Infof("Running %q", componentName)

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
	env = append(env, opt.Environment...)

	createConfigCmd := []string{
		"kubelet",
	}
	createConfigCmd = append(createConfigCmd, opt.Args...)

	containerID, err := imageRunHelper.Image(opt.NodeImage).
		Name(openshift.ContainerName).
		Privileged().
		DiscardContainer().
		HostNetwork().
		HostPid().
		Bind(opt.ContainerBinds...).
		Env(env...).
		Entrypoint("hyperkube").
		Command(createConfigCmd...).Start()
	if err != nil {
		return "", errors.NewError("could not create OpenShift configuration: %v", err).WithCause(err)
	}

	return containerID, nil
}
