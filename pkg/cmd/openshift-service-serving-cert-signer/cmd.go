package openshift_service_serving_cert_signer

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/wait"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/server/origin"
)

type OpenShiftSSCS struct {
	ConfigFile     string
	KubeConfigFile string
	Output         io.Writer
}

var longDescription = templates.LongDesc(`
	Start the OpenShift controllers`)

func NewOpenShiftServiceServingCertSignerCommand(name string, out, errout io.Writer) *cobra.Command {
	options := &OpenShiftSSCS{Output: out}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Start the OpenShift controllers",
		Long:  longDescription,
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Validate())

			origin.StartProfiler()

			if err := options.StartSSCS(); err != nil {
				if kerrors.IsInvalid(err) {
					if details := err.(*kerrors.StatusError).ErrStatus.Details; details != nil {
						fmt.Fprintf(errout, "Invalid %s %s\n", details.Kind, details.Name)
						for _, cause := range details.Causes {
							fmt.Fprintf(errout, "  %s: %s\n", cause.Field, cause.Message)
						}
						os.Exit(255)
					}
				}
				glog.Fatal(err)
			}
		},
	}

	flags := cmd.Flags()
	// This command only supports reading from config
	flags.StringVar(&options.ConfigFile, "config", options.ConfigFile, "Location of the  configuration file to run from.")
	cmd.MarkFlagFilename("config", "yaml", "yml")
	cmd.MarkFlagRequired("config")
	flags.StringVar(&options.KubeConfigFile, "kubeconfig", options.KubeConfigFile, "Location of the  configuration file to run from.")
	cmd.MarkFlagFilename("kubeconfig", "kubeconfig")

	return cmd
}

func (o *OpenShiftSSCS) Validate() error {
	if len(o.ConfigFile) == 0 {
		return errors.New("--config is required for this command")
	}

	return nil
}

// StartAPIServer calls RunAPIServer and then waits forever
func (o *OpenShiftSSCS) StartSSCS() error {
	if err := o.RunSSCS(); err != nil {
		return err
	}

	select {}
}

// RunAPIServer takes the options and starts the etcd server
func (o *OpenShiftSSCS) RunSSCS() error {
	config, err := readAndResolveConfig(o.ConfigFile)
	if err != nil {
		return err
	}

	// TODO validate config
	//validationResults := validation.ValidateMasterConfig(Config, nil)
	//if len(validationResults.Warnings) != 0 {
	//	for _, warning := range validationResults.Warnings {
	//		glog.Warningf("%v", warning)
	//	}
	//}
	//if len(validationResults.Errors) != 0 {
	//	return kerrors.NewInvalid(configapi.Kind("MasterConfig"), "config.yaml", validationResults.Errors)
	//}

	clientConfig, err := getKubeConfigOrInClusterConfig(o.KubeConfigFile, defaultConnectionOverrides)
	if err != nil {
		return err
	}

	return RunOpenShiftSSCS(config, clientConfig, wait.NeverStop)
}
