package openshift_network_controller

import (
	"fmt"
	"io"
	"os"

	"k8s.io/client-go/rest"

	"github.com/coreos/go-systemd/daemon"
	"github.com/spf13/cobra"
	"k8s.io/klog"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"

	"github.com/openshift/library-go/pkg/serviceability"
)

const RecommendedStartNetworkControllerName = "openshift-network-controller"

type OpenShiftNetworkController struct {
	ConfigFilePath string
	Output         io.Writer
}

var longDescription = templates.LongDesc(`
	Start the OpenShift SDN controller`)

func NewOpenShiftNetworkControllerCommand(name, basename string, out, errout io.Writer) *cobra.Command {
	options := &OpenShiftNetworkController{Output: out}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Start the OpenShift SDN controller",
		Long:  longDescription,
		Run: func(c *cobra.Command, args []string) {
			rest.CommandNameOverride = name
			kcmdutil.CheckErr(options.Validate())

			serviceability.StartProfiler()

			if err := options.StartNetworkController(); err != nil {
				if kerrors.IsInvalid(err) {
					if details := err.(*kerrors.StatusError).ErrStatus.Details; details != nil {
						fmt.Fprintf(errout, "Invalid %s %s\n", details.Kind, details.Name)
						for _, cause := range details.Causes {
							fmt.Fprintf(errout, "  %s: %s\n", cause.Field, cause.Message)
						}
						os.Exit(255)
					}
				}
				klog.Fatal(err)
			}
		},
	}

	flags := cmd.Flags()
	// This command only supports reading from config
	flags.StringVar(&options.ConfigFilePath, "config", options.ConfigFilePath, "Location of the master configuration file to run from.")
	cmd.MarkFlagFilename("config", "yaml", "yml")

	return cmd
}

func (o *OpenShiftNetworkController) Validate() error {
	return nil
}

// StartNetworkController calls RunOpenShiftNetworkController and then waits forever
func (o *OpenShiftNetworkController) StartNetworkController() error {
	if err := RunOpenShiftNetworkController(); err != nil {
		return err
	}

	go daemon.SdNotify(false, "READY=1")
	select {}
}
