package cluster

import (
	"fmt"
	"io"

	"github.com/openshift/origin/pkg/oc/bootstrap/clusteradd"
	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/oc/bootstrap/docker"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

const ClusterRecommendedName = "cluster"

var (
	clusterLong = templates.LongDesc(`
		Manage a local OpenShift cluster

		The OpenShift cluster will run as an all-in-one container on a Docker host. The Docker host
		may be a local VM (ie. using docker-machine on OS X and Windows clients), remote machine, or
		the local Unix host.

		Use the 'up' command to start a new cluster on a docker host.

		To use an existing Docker connection, ensure that Docker commands are working and that you
		can create new containers.

		Default routes are setup using nip.io and the host ip of your cluster. To use a different
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

	clusterAdd := clusteradd.NewCmdAdd(clusteradd.CmdAddRecommendedName, fullName+" "+clusteradd.CmdAddRecommendedName, out, errout)

	cmds.AddCommand(clusterAdd)
	cmds.AddCommand(docker.NewCmdUp(docker.CmdUpRecommendedName, fullName+" "+docker.CmdUpRecommendedName, out, errout, clusterAdd))
	cmds.AddCommand(docker.NewCmdDown(docker.CmdDownRecommendedName, fullName+" "+docker.CmdDownRecommendedName))
	cmds.AddCommand(docker.NewCmdStatus(docker.CmdStatusRecommendedName, fullName+" "+docker.CmdStatusRecommendedName, f, out))
	return cmds
}
