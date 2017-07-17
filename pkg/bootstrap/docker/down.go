package docker

import (
	"fmt"
	"io"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	dockertools "k8s.io/kubernetes/pkg/kubelet/dockershim/libdocker"

	"github.com/openshift/origin/pkg/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/bootstrap/docker/openshift"
	"github.com/openshift/origin/pkg/cmd/templates"
	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const CmdDownRecommendedName = "down"

var (
	cmdDownLong = templates.LongDesc(`
		Stops the container running OpenShift on Docker and associated containers.

		If you started your OpenShift with a specific docker-machine, you need to specify the
		same machine using the --docker-machine argument.`)

	cmdDownExample = templates.Examples(`
	  # Stop local OpenShift cluster
	  %[1]s

	  # Stop cluster running on Docker machine 'mymachine'
	  %[1]s --docker-machine=mymachine`)
)

type ClientStopConfig struct {
	DockerMachine string
}

// NewCmdDown creates a command that stops OpenShift
func NewCmdDown(name, fullName string, f *osclientcmd.Factory, out io.Writer) *cobra.Command {
	config := &ClientStopConfig{}
	cmd := &cobra.Command{
		Use:     name,
		Short:   "Stop OpenShift on Docker",
		Long:    cmdDownLong,
		Example: fmt.Sprintf(cmdDownExample, fullName),
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(config.Stop(out))
		},
	}
	cmd.Flags().StringVar(&config.DockerMachine, "docker-machine", "", "Specify the Docker machine to use")
	return cmd
}

// Stop stops the currently running origin container and any
// containers started by the node.
func (c *ClientStopConfig) Stop(out io.Writer) error {
	client, err := getDockerClient(out, c.DockerMachine, false)
	if err != nil {
		return err
	}
	helper := dockerhelper.NewHelper(client)
	glog.V(4).Infof("Killing previous socat tunnel")
	err = openshift.KillExistingSocat()
	if err != nil {
		glog.V(1).Infof("error: cannot kill socat: %v", err)
	}
	glog.V(4).Infof("Stopping and removing origin container")
	if err = helper.StopAndRemoveContainer("origin"); err != nil {
		glog.V(1).Infof("Error stopping origin container: %v", err)
	}
	names, err := helper.ListContainerNames()
	if err != nil {
		return err
	}
	for _, name := range names {
		if _, _, err = dockertools.ParseDockerName(name); err != nil {
			continue
		}
		name = strings.TrimLeft(name, "/")
		glog.V(4).Infof("Stopping container %s", name)
		if err = client.ContainerStop(name, 0); err != nil {
			glog.V(1).Infof("Error stopping container %s: %v", name, err)
		}
		glog.V(4).Infof("Removing container %s", name)
		if err = helper.RemoveContainer(name); err != nil {
			glog.V(1).Infof("Error removing container %s: %v", name, err)
		}
	}
	return nil
}
