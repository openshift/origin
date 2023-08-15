package disruption

import (
	poll_service "github.com/openshift/origin/pkg/cmd/openshift-tests/disruption/poll-service"
	watch_endpointslice "github.com/openshift/origin/pkg/cmd/openshift-tests/disruption/watch-endpointslice"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func NewDisruptionCommand(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "disruption",
		SilenceErrors: true,
	}
	cmd.AddCommand(
		watch_endpointslice.NewWatchEndpointSlice(streams),
		poll_service.NewPollService(streams),
	)
	return cmd
}
