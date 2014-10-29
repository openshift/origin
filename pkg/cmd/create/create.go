package create

import (
	"os"

	"github.com/openshift/origin/pkg/cmd/client"
	"github.com/spf13/cobra"
)

const longDescription = `
Perform bulk operations on groups of resources:
  osc create -c config.json
`

func NewCmdCreate(resource string) *cobra.Command {
	cfg := &client.KubeConfig{}

	cmd := &cobra.Command{
		Use:   resource,
		Short: "Perform bulk operations on groups of resources",
		Long:  longDescription,
		Run: func(c *cobra.Command, args []string) {
			cfg.Args = []string{"apply"}
			cfg.Run()
		},
	}
	flag := cmd.Flags()
	flag.StringVar(&cfg.ClientConfig.Host, "host", "", "The host to connect to.")
	flag.StringVarP(&cfg.Config, "config", "c", "", "Path or URL to the config file, or '-' to read from STDIN")
	flag.StringVar(&cfg.AuthConfig, "auth", os.Getenv("HOME")+"/.kubernetes_auth", "Path to the auth info file.  If missing, prompt the user.  Only used if doing https.")
	flag.BoolVar(&cfg.JSON, "json", false, "If true, print raw JSON for responses")
	flag.BoolVar(&cfg.YAML, "yaml", false, "If true, print raw YAML for responses")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "If true, print extra information")
	flag.StringVar(&cfg.TemplateFile, "template_file", "", "If present, load this file as a golang template and use it for output printing")
	flag.StringVar(&cfg.TemplateStr, "template", "", "If present, parse this string as a golang template and use it for output printing")
	flag.StringVar(&cfg.ClientConfig.CAFile, "certificate_authority", "", "Path to a cert. file for the certificate authority")
	flag.StringVar(&cfg.ClientConfig.CertFile, "client_certificate", "", "Path to a client certificate for TLS.")
	flag.StringVar(&cfg.ClientConfig.KeyFile, "client_key", "", "Path to a client key file for TLS.")
	flag.BoolVar(&cfg.ClientConfig.Insecure, "insecure_skip_tls_verify", false, "If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure.")

	return cmd
}
