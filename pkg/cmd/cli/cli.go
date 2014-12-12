package cli

import (
	"fmt"
	"os"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"

	"github.com/openshift/origin/pkg/cmd/cli/cmd"
	"github.com/spf13/cobra"
)

func NewCommandCLI(name string) *cobra.Command {
	// Main command
	cmds := &cobra.Command{
		Use:     name,
		Aliases: []string{"kubectl"},
		Short:   "Client tools for OpenShift",
		Long: `
End-user client tool for OpenShift, the hybrid Platform as a Service by the open source leader Red Hat.
Note: This is an alpha release of OpenShift and will change significantly.  See
    https://github.com/openshift/origin
for the latest information on OpenShift.
`,
		Run: func(c *cobra.Command, args []string) {
			c.Help()
		},
	}

	clientBuilder := clientcmd.NewBuilder(clientcmd.NewPromptingAuthLoader(os.Stdin))

	clientBuilder.BindFlags(cmds.PersistentFlags())

	// TODO reuse
	cmds.PersistentFlags().String("ns-path", os.Getenv("HOME")+"/.kubernetes_ns", "Path to the namespace info file that holds the name space context to use for CLI requests.")
	cmds.PersistentFlags().StringP("namespace", "n", "", "If present, the namespace scope for this CLI request.")

	factory := cmd.NewOriginFactory(clientBuilder)

	factory.Factory.Printer = func(cmd *cobra.Command, mapping *meta.RESTMapping, noHeaders bool) (kubectl.ResourcePrinter, error) {
		return NewHumanReadablePrinter(noHeaders), nil
	}

	// Initialize describer for Origin objects
	factory.OriginDescriber = func(cmd *cobra.Command, mapping *meta.RESTMapping) (kubectl.Describer, error) {
		if c, err := factory.OriginClient(cmd, mapping); err == nil {
			config, err := factory.ClientBuilder.Config()
			if err != nil {
				return nil, err
			}
			if describer, ok := DescriberFor(mapping.Kind, c, config.Host); ok == true {
				return describer, nil
			}
		}
		return nil, fmt.Errorf("unable to describe %s type", mapping.Kind)
	}

	factory.AddCommands(cmds, os.Stdout)

	return cmds
}
