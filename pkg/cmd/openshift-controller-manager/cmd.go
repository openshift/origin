package openshift_controller_manager

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/coreos/go-systemd/daemon"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/validation"
	"github.com/openshift/origin/pkg/cmd/server/origin"
)

const RecommendedStartControllerManagerName = "openshift-controller-manager"

type OpenShiftControllerManager struct {
	ConfigFile string
	Output     io.Writer
}

var longDescription = templates.LongDesc(`
	Start the OpenShift controllers`)

func NewOpenShiftControllerManagerCommand(name, basename string, out, errout io.Writer) *cobra.Command {
	options := &OpenShiftControllerManager{Output: out}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Start the OpenShift controllers",
		Long:  longDescription,
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Validate())

			origin.StartProfiler()

			if err := options.StartControllerManager(); err != nil {
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
	flags.StringVar(&options.ConfigFile, "config", "", "Location of the master configuration file to run from.")
	cmd.MarkFlagFilename("config", "yaml", "yml")
	cmd.MarkFlagRequired("config")

	return cmd
}

func (o *OpenShiftControllerManager) Validate() error {
	if len(o.ConfigFile) == 0 {
		return errors.New("--config is required for this command")
	}

	return nil
}

// StartAPIServer calls RunAPIServer and then waits forever
func (o *OpenShiftControllerManager) StartControllerManager() error {
	if err := o.RunControllerManager(); err != nil {
		return err
	}

	go daemon.SdNotify(false, "READY=1")
	select {}
}

// RunAPIServer takes the options and starts the etcd server
func (o *OpenShiftControllerManager) RunControllerManager() error {
	masterConfig, err := configapilatest.ReadAndResolveMasterConfig(o.ConfigFile)
	if err != nil {
		return err
	}

	validationResults := validation.ValidateMasterConfig(masterConfig, nil)
	if len(validationResults.Warnings) != 0 {
		for _, warning := range validationResults.Warnings {
			glog.Warningf("%v", warning)
		}
	}
	if len(validationResults.Errors) != 0 {
		return kerrors.NewInvalid(configapi.Kind("MasterConfig"), "master-config.yaml", validationResults.Errors)
	}

	return RunOpenShiftControllerManager(masterConfig)
}
