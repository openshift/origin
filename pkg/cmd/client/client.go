package client

import (
	"os"
	"time"

	"github.com/openshift/origin/pkg/api/latest"
	kubectl "github.com/openshift/origin/pkg/kubectl/cmd"
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

func NewCommandKubectl(name string) *cobra.Command {
	cmds := &cobra.Command{
		Use:   name,
		Short: "kubectl controls the Kubernetes cluster manager",
		Long: `kubectl controls the Kubernetes cluster manager.

Find more information at https://github.com/GoogleCloudPlatform/kubernetes.`,
		Run: func(c *cobra.Command, args []string) {
			c.Help()
		},
	}

	// Globally persistent flags across all subcommands.
	// TODO Change flag names to consts to allow safer lookup from subcommands.
	// TODO Add a verbose flag that turns on glog logging. Probably need a way
	// to do that automatically for every subcommand.
	cmds.PersistentFlags().StringP("server", "s", "", "Kubernetes apiserver to connect to")
	cmds.PersistentFlags().StringP("auth-path", "a", os.Getenv("HOME")+"/.kubernetes_auth", "Path to the auth info file. If missing, prompt the user. Only used if using https.")
	cmds.PersistentFlags().Bool("match-server-version", false, "Require server version to match client version")
	cmds.PersistentFlags().String("api-version", latest.Version, "The version of the API to use against the server (used for viewing resources only)")
	cmds.PersistentFlags().String("certificate-authority", "", "Path to a certificate file for the certificate authority")
	cmds.PersistentFlags().String("client-certificate", "", "Path to a client certificate for TLS.")
	cmds.PersistentFlags().String("client-key", "", "Path to a client key file for TLS.")
	cmds.PersistentFlags().Bool("insecure-skip-tls-verify", false, "If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure.")

	f := kubectl.NewFactory()
	out := os.Stdout
	//cmds.AddCommand(NewCmdVersion(out))
	//cmds.AddCommand(NewCmdProxy(out))
	cmds.AddCommand(f.NewCmdGet(out))
	cmds.AddCommand(f.NewCmdDescribe(out))
	cmds.AddCommand(f.NewCmdCreate(out))
	cmds.AddCommand(f.NewCmdUpdate(out))
	cmds.AddCommand(f.NewCmdDelete(out))

	return cmds
}
