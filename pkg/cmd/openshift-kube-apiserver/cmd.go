package openshift_kube_apiserver

import (
	"io"

	"k8s.io/kubernetes/cmd/kube-apiserver/app"

	"github.com/spf13/cobra"
	"k8s.io/klog"

	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"
)

const RecommendedStartAPIServerName = "openshift-kube-apiserver"

type OpenShiftKubeAPIServerServer struct {
	ConfigFile string
	Output     io.Writer
}

var longDescription = templates.LongDesc(`
	Start the extended kube-apiserver with OpenShift security extensions`)

func NewOpenShiftKubeAPIServerServerCommand(name, basename string, out, errout io.Writer, stopCh <-chan struct{}) *cobra.Command {
	options := &OpenShiftKubeAPIServerServer{Output: out}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Start the OpenShift kube-apiserver",
		Long:  longDescription,
		Run: func(c *cobra.Command, args []string) {
			rest.CommandNameOverride = name

			kubecmd := app.NewAPIServerCommand(stopCh)
			newArgs := append(args, "--openshift-config="+options.ConfigFile)
			if err := kubecmd.ParseFlags(newArgs); err != nil {
				klog.Fatal(err)
			}
			klog.Infof("`kube-apiserver %v`", args)
			if err := kubecmd.RunE(kubecmd, nil); err != nil {
				klog.Fatal(err)
			}

			// When no error is returned, always return with zero exit code.
			// This is here to make sure the container that run apiserver won't get accidentally restarted
			// when the pod runs with restart on failure.
		},
	}

	flags := cmd.Flags()
	// This command only supports reading from config
	flags.StringVar(&options.ConfigFile, "config", "", "Location of the master configuration file to run from.")
	cmd.MarkFlagFilename("config", "yaml", "yml")
	cmd.MarkFlagRequired("config")

	return cmd
}
