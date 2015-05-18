package diagnostics

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	diagnosticflags "github.com/openshift/origin/pkg/cmd/experimental/diagnostics/options"
	"github.com/openshift/origin/pkg/diagnostics/log"
	masterdiagnostics "github.com/openshift/origin/pkg/diagnostics/master"
	systemddiagnostics "github.com/openshift/origin/pkg/diagnostics/systemd"
	diagnostictypes "github.com/openshift/origin/pkg/diagnostics/types/diagnostic"
)

const (
	MasterDiagnosticsRecommendedName = "master"

	StandardMasterConfigPath string = "/etc/openshift/master/master-config.yaml"
)

var (
	AvailableMasterDiagnostics = util.NewStringSet("AnalyzeLogs", "UnitStatus", "MasterConfigCheck")
)

// user options for openshift-diagnostics client command
type MasterDiagnosticsOptions struct {
	RequestedDiagnostics util.StringList

	MasterConfigLocation string

	LogOptions *log.LoggerOptions
	Logger     *log.Logger
}

const longMasterDescription = `
OpenShift Diagnostics

This command helps you understand and troubleshoot a running OpenShift
master. It is intended to be run from the same context as the master
(where "openshift start" or "openshift start master" is run, possibly from
systemd or inside a container) and with the same configuration options.

    $ %s
`

func NewMasterCommand(name string, fullName string, out io.Writer) *cobra.Command {
	o := &MasterDiagnosticsOptions{
		RequestedDiagnostics: AvailableMasterDiagnostics.List(),
		LogOptions:           &log.LoggerOptions{Out: out},
	}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Troubleshoot an OpenShift v3 master.",
		Long:  fmt.Sprintf(longMasterDescription, fullName),
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete())

			failed, err := o.RunDiagnostics()
			o.Logger.Summary()
			o.Logger.Finish()

			kcmdutil.CheckErr(err)
			if failed {
				os.Exit(255)
			}
		},
	}

	cmd.SetOutput(out) // for output re: usage / help

	cmd.Flags().StringVar(&o.MasterConfigLocation, "master-config", "", "path to master config file")
	diagnosticflags.BindLoggerOptionFlags(cmd.Flags(), o.LogOptions, diagnosticflags.RecommendedLoggerOptionFlags())
	diagnosticflags.BindDiagnosticFlag(cmd.Flags(), &o.RequestedDiagnostics, diagnosticflags.NewRecommendedDiagnosticFlag())

	return cmd
}

func (o *MasterDiagnosticsOptions) Complete() error {
	// set the master config location if it hasn't been set and we find it in an expected location
	if len(o.MasterConfigLocation) == 0 {
		if _, err := os.Stat(StandardMasterConfigPath); !os.IsNotExist(err) {
			o.MasterConfigLocation = StandardMasterConfigPath
		}

	}

	var err error
	o.Logger, err = o.LogOptions.NewLogger()
	if err != nil {
		return err
	}

	return nil
}

func (o MasterDiagnosticsOptions) RunDiagnostics() (bool, error) {
	diagnostics := map[string]diagnostictypes.Diagnostic{}

	// if we don't have a master config file, then there's no work to do
	if len(o.MasterConfigLocation) == 0 {
		// TODO remove MasterConfigCheck from the list
	}

	systemdUnits := systemddiagnostics.GetSystemdUnits(o.Logger)

	for _, diagnosticName := range o.RequestedDiagnostics {
		switch diagnosticName {
		case "AnalyzeLogs":
			diagnostics[diagnosticName] = systemddiagnostics.AnalyzeLogs{systemdUnits, o.Logger}

		case "UnitStatus":
			diagnostics[diagnosticName] = systemddiagnostics.UnitStatus{systemdUnits, o.Logger}

		case "MasterConfigCheck":
			diagnostics[diagnosticName] = masterdiagnostics.MasterConfigCheck{o.MasterConfigLocation, o.Logger}

		default:
			return false, fmt.Errorf("unknown diagnostic: %v", diagnosticName)
		}
	}

	for name, diagnostic := range diagnostics {
		if canRun, reason := diagnostic.CanRun(); !canRun {
			if reason == nil {
				o.Logger.Noticem(log.Message{ID: "diagSkip", Template: "Skipping diagnostic: {{.area}}.{{.name}}\nDescription: {{.diag}}", TemplateData: map[string]string{"area": "master", "name": name, "diag": diagnostic.Description()}})
			} else {
				o.Logger.Noticem(log.Message{ID: "diagSkip", Template: "Skipping diagnostic: {{.area}}.{{.name}}\nDescription: {{.diag}}\nBecause: {{.reason}}", TemplateData: map[string]string{"area": "master", "name": name, "diag": diagnostic.Description(), "reason": reason.Error()}})
			}
			continue
		}

		o.Logger.Noticem(log.Message{ID: "diagRun", Template: "Running diagnostic: {{.area}}.{{.name}}\nDescription: {{.diag}}", TemplateData: map[string]string{"area": "master", "name": name, "diag": diagnostic.Description()}})
		diagnostic.Check()
	}

	return o.Logger.ErrorsSeen(), nil
}
