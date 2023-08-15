package network_check

import (
	watch_endpointslice "github.com/openshift/origin/pkg/cmd/openshift-tests/network-check/watch-endpointslice"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func NewNetworkCheckCommand(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "run-network-check",
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	cmd.AddCommand(
		watch_endpointslice.NewWatchEndpointSlice(streams),
	)
	return cmd

}
