package start

import (
	"io"
	_ "net/http/pprof"

	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"

	"github.com/openshift/origin/pkg/cmd/openshift-etcd"
	tsbcmd "github.com/openshift/origin/pkg/templateservicebroker/cmd/server"
)

// NewCommandStart provides a CLI handler for 'start' command
func NewCommandStart(basename string, out, errout io.Writer, stopCh <-chan struct{}) *cobra.Command {

	cmds := &cobra.Command{
		Use:   "start",
		Short: "Launch all-in-one server",
		Long: templates.LongDesc(`
			Start components of OpenShift
		
			This command launches components of OpenShift.

			`),
		Deprecated: "This command will be replaced by the hypershift and hyperkube binaries for starting individual components.",
	}

	startMaster, _ := NewCommandStartMaster(basename, out, errout)
	startNodeNetwork, _ := NewCommandStartNetwork(basename, out, errout)
	startEtcdServer, _ := openshift_etcd.NewCommandStartEtcdServer(openshift_etcd.RecommendedStartEtcdServerName, basename, out, errout)
	startTSBServer := tsbcmd.NewCommandStartTemplateServiceBrokerServer(out, errout, stopCh)
	cmds.AddCommand(startMaster)
	cmds.AddCommand(startNodeNetwork)
	cmds.AddCommand(startEtcdServer)
	cmds.AddCommand(startTSBServer)

	return cmds
}
