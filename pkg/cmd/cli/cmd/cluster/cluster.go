package cluster

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/bootstrap/docker"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const ClusterRecommendedName = "cluster"

var (
	clusterLong = templates.LongDesc(`
		Manage a local OpenShift cluster

		The OpenShift cluster will run as an all-in-one container on a Docker host. The Docker host
		may be a local VM (ie. using docker-machine on OS X and Windows clients), remote machine, or
		the local Unix host.

		To use an existing Docker connection, ensure that Docker commands are working and that you
		can create new containers. For OS X and Windows clients, a docker-machine with the VirtualBox
		driver can be created for you using the --create-machine option.

		By default, etcd data will not be preserved between container restarts. If you wish to
		preserve your data, specify a value for --host-data-dir and the --use-existing-config flag.

		Default routes are setup using xip.io and the host ip of your cluster. To use a different
		routing suffix, use the --routing-suffix flag.`)
)

func NewCmdCluster(name, fullName string, f *clientcmd.Factory, out, errout io.Writer) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   fmt.Sprintf("%s ACTION", name),
		Short: "Start and stop OpenShift cluster",
		Long:  clusterLong,
		Run:   cmdutil.DefaultSubCommandRun(errout),
	}

	cmds.AddCommand(docker.NewCmdUp(docker.CmdUpRecommendedName, fullName+" "+docker.CmdUpRecommendedName, f, out, errout))
	cmds.AddCommand(docker.NewCmdDown(docker.CmdDownRecommendedName, fullName+" "+docker.CmdDownRecommendedName, f, out))
	cmds.AddCommand(docker.NewCmdStatus(docker.CmdStatusRecommendedName, fullName+" "+docker.CmdStatusRecommendedName, f, out))
	return cmds
}
