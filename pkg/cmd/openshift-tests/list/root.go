package list

import (
	"github.com/openshift-eng/openshift-tests-extension/pkg/cmd/cmdlist"
	"github.com/openshift-eng/openshift-tests-extension/pkg/extension"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func NewListCommand(streams genericclioptions.IOStreams, extensionRegistry *extension.Registry) *cobra.Command {

	oteListCmd := cmdlist.NewListCommand(extensionRegistry)

	// Remove OTE's own suites command (maybe put it back later, if we can register all the
	// extension suites here too).
	for _, c := range oteListCmd.Commands() {
		if c.Use == "suites" {
			oteListCmd.RemoveCommand(c)
		}
	}

	oteListCmd.AddCommand(
		NewListSuitesCommand(streams),
		NewListExtensionsCommand(streams),
	)

	return oteListCmd
}
