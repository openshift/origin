package docker

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	units "github.com/docker/go-units"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/bootstrap/docker/errors"
	"github.com/openshift/origin/pkg/bootstrap/docker/openshift"
	"github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

// CmdStatusRecommendedName is the recommended command name
const CmdStatusRecommendedName = "status"

var (
	cmdStatusLong = templates.LongDesc(`
		Show the status of the local OpenShift cluster.

		If you started your OpenShift with a specific docker-machine, you need to specify the
		same machine using the --docker-machine argument.`)

	cmdStatusExample = templates.Examples(`
		# See status of local OpenShift cluster
		%[1]s

		# See status of OpenShift cluster running on Docker machine 'mymachine'
		%[1]s --docker-machine=mymachine`)
)

// NewCmdStatus implements the OpenShift cluster status command.
func NewCmdStatus(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	config := &ClientStatusConfig{}
	cmd := &cobra.Command{
		Use:     name,
		Short:   "Show OpenShift on Docker status",
		Long:    cmdStatusLong,
		Example: fmt.Sprintf(cmdStatusExample, fullName),
		Run: func(c *cobra.Command, args []string) {
			err := config.Status(f, out)
			if err != nil {
				PrintError(err, out)
				os.Exit(1)
			}
		},
	}
	cmd.Flags().StringVar(&config.DockerMachine, "docker-machine", "", "Specify the Docker machine to use")
	return cmd
}

// ClientStatusConfig is the configuration for the client status command
type ClientStatusConfig struct {
	DockerMachine string
}

// Status prints the OpenShift cluster status
func (c *ClientStatusConfig) Status(f *clientcmd.Factory, out io.Writer) error {
	dockerClient, _, err := getDockerClient(out, c.DockerMachine, false)
	if err != nil {
		return errors.ErrNoDockerClient(err)
	}
	helper := dockerhelper.NewHelper(dockerClient, nil)

	container, running, err := helper.GetContainerState(openshift.OpenShiftContainer)
	if err != nil {
		return errors.NewError("cannot get state of OpenShift container %s", openshift.OpenShiftContainer).WithCause(err)
	}

	if !running {
		return errors.NewError("OpenShift cluster is not running")
	}

	healthy, err := isHealthy(f)
	if err != nil {
		return err
	}
	if !healthy {
		return errors.NewError("OpenShift cluster health check failed")
	}

	config, err := openshift.GetConfigFromContainer(dockerClient)
	if err != nil {
		return err
	}

	fmt.Print(status(container, config))

	return nil
}

func isHealthy(f *clientcmd.Factory) (bool, error) {
	osClient, _, err := f.Clients()
	if err != nil {
		return false, err
	}

	var statusCode int
	osClient.Client.Timeout = 10 * time.Second
	osClient.Get().AbsPath("/healthz").Do().StatusCode(&statusCode)
	return statusCode == 200, nil
}

func status(container *docker.Container, config *api.MasterConfig) string {
	mountMap := make(map[string]string)
	for _, mount := range container.Mounts {
		mountMap[mount.Destination] = mount.Source
	}

	duration := strings.ToLower(units.HumanDuration(time.Now().Sub(container.State.StartedAt)))

	status := fmt.Sprintf("The OpenShift cluster was started %s ago\n\n", duration)

	status = status + fmt.Sprintf("Web console URL: %s\n", config.AssetConfig.MasterPublicURL)
	if config.AssetConfig.MetricsPublicURL != "" {
		status = status + fmt.Sprintf("Metrics URL:     %s\n", config.AssetConfig.MetricsPublicURL)
	}
	if config.AssetConfig.LoggingPublicURL != "" {
		status = status + fmt.Sprintf("Logging URL:     %s\n", config.AssetConfig.LoggingPublicURL)
	}
	status = status + fmt.Sprintf("\n")

	status = status + fmt.Sprintf("Config is at host directory %s\n", mountMap["/var/lib/origin/openshift.local.config"])
	status = status + fmt.Sprintf("Volumes are at host directory %s\n", mountMap["/var/lib/origin/openshift.local.volumes"])
	if _, hasKey := mountMap["/var/lib/origin/openshift.local.etcd"]; hasKey {
		status = status + fmt.Sprintf("Data is at host directory %s\n", mountMap["/var/lib/origin/openshift.local.etcd"])
	} else {
		status = status + fmt.Sprintf("Data will be discarded when cluster is destroyed\n")
	}

	return status
}
