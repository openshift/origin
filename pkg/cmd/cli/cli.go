package cli

import (
	"fmt"
	"os"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	"github.com/openshift/origin/pkg/api/latest"
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

	// TODO reuse
	cmds.PersistentFlags().StringP("server", "s", "", "Kubernetes apiserver to connect to")
	cmds.PersistentFlags().StringP("auth-path", "a", os.Getenv("HOME")+"/.kubernetes_auth", "Path to the auth info file. If missing, p rompt the user. Only used if using https.")
	cmds.PersistentFlags().Bool("match-server-version", false, "Require server version to match client version")
	cmds.PersistentFlags().String("api-version", latest.Version, "The version of the API to use against the server")
	cmds.PersistentFlags().String("certificate-authority", "", "Path to a certificate file for the certificate authority")
	cmds.PersistentFlags().String("client-certificate", "", "Path to a client certificate for TLS.")
	cmds.PersistentFlags().String("client-key", "", "Path to a client key file for TLS.")
	cmds.PersistentFlags().Bool("insecure-skip-tls-verify", false, "If true, the server's certificate will not be checked for validity . This will make your HTTPS connections insecure.")
	cmds.PersistentFlags().String("ns-path", os.Getenv("HOME")+"/.kubernetes_ns", "Path to the namespace info file that holds the name space context to use for CLI requests.")
	cmds.PersistentFlags().StringP("namespace", "n", "", "If present, the namespace scope for this CLI request.")

	factory := cmd.NewOriginFactory()

	factory.Factory.Printer = func(cmd *cobra.Command, mapping *meta.RESTMapping, noHeaders bool) (kubectl.ResourcePrinter, error) {
		return NewHumanReadablePrinter(noHeaders), nil
	}

	// Initialize describer for Origin objects
	factory.OriginDescriber = func(cmd *cobra.Command, mapping *meta.RESTMapping) (kubectl.Describer, error) {
		if c, err := factory.OriginClient(cmd, mapping); err == nil {
			if describer, ok := DescriberFor(mapping.Kind, c, cmd); ok == true {
				return describer, nil
			}
		}
		return nil, fmt.Errorf("unable to describe %s type", mapping.Kind)
	}

	factory.AddCommands(cmds, os.Stdout)

	return cmds
}
