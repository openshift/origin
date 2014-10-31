package client

import (
	"os"
	"time"

	"github.com/spf13/cobra"
)

const longDescription = `
Kubernetes Command Line - kubecfg

OpenShift currently embeds the kubecfg command line for prototyping and debugging.
`

func NewCommandKubecfg(name string) *cobra.Command {
	cfg := &KubeConfig{}
	cmd := &cobra.Command{
		Use:   name,
		Short: "The Kubernetes command line client",
		Long:  longDescription + usage(name),
		Run: func(c *cobra.Command, args []string) {
			if len(args) < 1 {
				c.Help()
				os.Exit(1)
			}
			cfg.Args = args
			cfg.Run()
		},
	}
	flag := cmd.Flags()
	flag.BoolVar(&cfg.ServerVersion, "server_version", false, "Print the server's version number.")
	flag.BoolVar(&cfg.PreventSkew, "expect_version_match", false, "Fail if server's version doesn't match own version.")
	flag.StringVar(&cfg.ClientConfig.Host, "host", "", "The host to connect to.")
	flag.StringVarP(&cfg.Config, "config", "c", "", "Path or URL to the config file, or '-' to read from STDIN")
	flag.StringVarP(&cfg.Selector, "label", "l", "", "Selector (label query) to use for listing")
	flag.DurationVarP(&cfg.UpdatePeriod, "update", "u", 60*time.Second, "Update interval period")
	flag.StringVarP(&cfg.PortSpec, "port", "p", "", "The port spec, comma-separated list of <external>:<internal>,...")
	flag.IntVarP(&cfg.ServicePort, "service", "s", -1, "If positive, create and run a corresponding service on this port, only used with 'run'")
	flag.StringVar(&cfg.AuthConfig, "auth", os.Getenv("HOME")+"/.kubernetes_auth", "Path to the auth info file.  If missing, prompt the user.  Only used if doing https.")
	flag.BoolVar(&cfg.JSON, "json", false, "If true, print raw JSON for responses")
	flag.BoolVar(&cfg.YAML, "yaml", false, "If true, print raw YAML for responses")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "If true, print extra information")
	flag.BoolVar(&cfg.Proxy, "proxy", false, "If true, run a proxy to the api server")
	flag.StringVar(&cfg.WWW, "www", "", "If -proxy is true, use this directory to serve static files")
	flag.StringVar(&cfg.TemplateFile, "template_file", "", "If present, load this file as a golang template and use it for output printing")
	flag.StringVar(&cfg.TemplateStr, "template", "", "If present, parse this string as a golang template and use it for output printing")
	flag.StringVar(&cfg.ClientConfig.CAFile, "certificate_authority", "", "Path to a cert. file for the certificate authority")
	flag.StringVar(&cfg.ClientConfig.CertFile, "client_certificate", "", "Path to a client certificate for TLS.")
	flag.StringVar(&cfg.ClientConfig.KeyFile, "client_key", "", "Path to a client key file for TLS.")
	flag.BoolVar(&cfg.ClientConfig.Insecure, "insecure_skip_tls_verify", false, "If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure.")
	flag.StringVar(&cfg.ImageName, "image", "", "Image used when updating a replicationController.  Will apply to the first container in the pod template.")
	flag.StringVar(&cfg.ID, "id", "", "Specifies ID of requested resource.")
	flag.StringVar(&cfg.ns, "ns", "", "If present, the namespace scope for this request.")
	flag.StringVar(&cfg.nsFile, "ns_file", os.Getenv("HOME")+"/.kubernetes_ns", "Path to the namespace file")

	return cmd
}
