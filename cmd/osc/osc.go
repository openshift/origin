package main

import (
	"fmt"
	"os"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	kubectl "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"
	"github.com/openshift/origin/pkg/cmd/auth"
	"github.com/openshift/origin/pkg/cmd/setup"
	"github.com/openshift/origin/pkg/version"
	"github.com/spf13/cobra"
)

const longDescription = `
End-user client tool for OpenShift, the hybrid Platform as a Service by the open source leader Red Hat.

Note: This is an alpha release of OpenShift and will change significantly.  See

    https://github.com/openshift/origin

for the latest information on OpenShift.

`

func main() {
	// Root command
	oscCmd := NewCmdOpenShiftClient("osc")

	// Version
	oscCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: fmt.Sprintf("Command '%s' (main)", "version"),
		Long:  fmt.Sprintf("Command '%s' (main)", "version"),
		Run: func(c *cobra.Command, args []string) {
			fmt.Println("OpenShift", version.Get().String())
		},
	})

	// Root command execution path
	if err := oscCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err)
		os.Exit(1)
	}
}

func NewCmdOpenShiftClient(name string) *cobra.Command {
	// Main command
	cmd := &cobra.Command{
		Use:   name,
		Short: "Client tools for OpenShift",
		Long:  longDescription,
		Run: func(c *cobra.Command, args []string) {
			c.Help()
		},
	}

	// Subcommands
	//cmd.AddCommand(create.NewCmdCreate("create"))
	//cmd.AddCommand(deployment.NewCmdDeployment("deployment"))
	//cmd.AddCommand(pod.NewCmdPod("pod"))
	cmd.AddCommand(setup.NewCmdSetup("setup"))
	cmd.AddCommand(auth.NewCmdLogin("login"))
	cmd.AddCommand(auth.NewCmdLogout("logout"))

	// Exposes kubectl subcommands
	cmd.PersistentFlags().StringP("server", "s", "", "Kubernetes apiserver to connect to")
	cmd.PersistentFlags().StringP("auth-path", "a", os.Getenv("HOME")+"/.kubernetes_auth", "Path to the auth info file. If missing, prompt the user. Only used if using https.")
	cmd.PersistentFlags().Bool("match-server-version", false, "Require server version to match client version")
	cmd.PersistentFlags().String("api-version", latest.Version, "The version of the API to use against the server (used for viewing resources only)")
	cmd.PersistentFlags().String("certificate-authority", "", "Path to a certificate file for the certificate authority")
	cmd.PersistentFlags().String("client-certificate", "", "Path to a client certificate for TLS.")
	cmd.PersistentFlags().String("client-key", "", "Path to a client key file for TLS.")
	cmd.PersistentFlags().Bool("insecure-skip-tls-verify", false, "If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure.")

	f := kubectl.NewFactory()
	out := os.Stdout
	//cmds.AddCommand(NewCmdVersion(out))
	//cmds.AddCommand(NewCmdProxy(out))
	cmd.AddCommand(f.NewCmdGet(out))
	cmd.AddCommand(f.NewCmdDescribe(out))
	cmd.AddCommand(f.NewCmdCreate(out))
	cmd.AddCommand(f.NewCmdUpdate(out))
	cmd.AddCommand(f.NewCmdDelete(out))

	return cmd
}
