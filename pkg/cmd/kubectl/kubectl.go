package kubectl

import (
	"os"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/cmd/kubectl/cmd"
	"github.com/spf13/cobra"
)

func NewCommandKubectl(name string) *cobra.Command {
	cmds := &cobra.Command{
		Use:   name,
		Short: name + " controls the Kubernetes cluster manager and OpenShift",
		Long: `Controls the Kubernetes cluster manager.
Find more information at https://github.com/GoogleCloudPlatform/kubernetes and
https://github.com/openshift/origin.`,
		Run: func(c *cobra.Command, args []string) {
			c.Help()
		},
	}

	// TODO: Make this as a method in upstream and just consume this method
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

	factory.AddCommands(cmds, os.Stdout)
	return cmds
}
