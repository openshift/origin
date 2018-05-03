package docker

import (
	"fmt"
	"io"
	"runtime"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/openshift"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/run"
)

const CmdDownRecommendedName = "down"

var (
	cmdDownLong = templates.LongDesc(`
		Stops the container running OpenShift on Docker and associated containers.`)

	cmdDownExample = templates.Examples(`
	  # Stop local OpenShift cluster
	  %[1]s`)
)

type ClientStopConfig struct {
}

// NewCmdDown creates a command that stops OpenShift
func NewCmdDown(name, fullName string, out io.Writer) *cobra.Command {
	config := &ClientStopConfig{}
	cmd := &cobra.Command{
		Use:     name,
		Short:   "Stop OpenShift on Docker",
		Long:    cmdDownLong,
		Example: fmt.Sprintf(cmdDownExample, fullName),
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(config.Stop())
		},
	}
	return cmd
}

// Stop stops the currently running origin container and any
// containers started by the node.
func (c *ClientStopConfig) Stop() error {
	client, err := GetDockerClient()
	if err != nil {
		return err
	}
	helper := dockerhelper.NewHelper(client)
	glog.V(4).Infof("Killing previous socat tunnel")
	err = openshift.KillExistingSocat()
	if err != nil {
		glog.V(2).Infof("error: cannot kill socat: %v", err)
	}
	originContainer, err := client.ContainerInspect(openshift.ContainerName)
	if err != nil {
		glog.V(2).Infof("Unable to inspect origin container: %v", err)
	}
	originContainerImage := originContainer.Config.Image
	glog.V(4).Infof("Stopping and removing origin container")
	if err = helper.StopAndRemoveContainer(openshift.ContainerName); err != nil {
		glog.V(2).Infof("Error stopping origin container: %v", err)
	}

	names, err := helper.ListContainerNames()
	if err != nil {
		return err
	}
	for _, name := range names {
		if _, err = parseDockerName(name); err != nil {
			continue
		}
		name = strings.TrimLeft(name, "/")
		glog.V(4).Infof("Stopping container %s", name)
		if err = client.ContainerStop(name, 0); err != nil {
			glog.V(2).Infof("Error stopping container %s: %v", name, err)
		}
		glog.V(4).Infof("Removing container %s", name)
		if err = helper.RemoveContainer(name); err != nil {
			glog.V(2).Infof("Error removing container %s: %v", name, err)
		}
	}
	// FIXME: Docker For Mac snowflake
	// The Docker For Mac does not stop the containers properly and does not report them as running via Docker API.
	// However the k8s_POD and hypershift containers (static pod containers) are still running in the VM, hidden...
	// That is causing issues when you want to run cluster up again as you have to restart the entire VM to get rid
	// of these containers.
	// See: https://github.com/docker/for-mac/issues/2844
	if runtime.GOOS == "darwin" {
		runner := run.NewRunHelper(helper).New()
		cleanPodsCmd := "nsenter -t 1 -m -u -i -n /bin/sh -c '/containers/services/docker-ce/rootfs/usr/local/bin/docker ps --filter name=k8s_* -q" +
			" | xargs /containers/services/docker-ce/rootfs/usr/local/bin/docker kill'"
		out, rc, err := runner.Image(originContainerImage).
			DiscardContainer().
			Privileged().
			HostPid().
			Entrypoint("/bin/bash").
			Command("-c", cleanPodsCmd).Run()
		if rc != 0 || err != nil {
			glog.V(5).Infof("Docker For Mac container cleanup failed: %s (%v)", out, err)
		}
	}
	return nil
}

// Unpacks a container name, returning the pod full name and container name we would have used to
// construct the docker name. If we are unable to parse the name, an error is returned.
func parseDockerName(name string) (hash uint64, err error) {
	const containerNamePrefix = "k8s"
	// For some reason docker appears to be appending '/' to names.
	// If it's there, strip it.
	name = strings.TrimPrefix(name, "/")
	parts := strings.Split(name, "_")
	if len(parts) == 0 || parts[0] != containerNamePrefix {
		err = fmt.Errorf("failed to parse Docker container name %q into parts", name)
		return 0, err
	}
	if len(parts) < 6 {
		// We have at least 5 fields.  We may have more in the future.
		// Anything with less fields than this is not something we can
		// manage.
		glog.Warningf("found a container with the %q prefix, but too few fields (%d): %q", containerNamePrefix, len(parts), name)
		err = fmt.Errorf("Docker container name %q has less parts than expected %v", name, parts)
		return 0, err
	}

	nameParts := strings.Split(parts[1], ".")
	if len(nameParts) > 1 {
		hash, err = strconv.ParseUint(nameParts[1], 16, 32)
		if err != nil {
			glog.Warningf("invalid container hash %q in container %q", nameParts[1], name)
		}
	}

	return hash, nil
}
