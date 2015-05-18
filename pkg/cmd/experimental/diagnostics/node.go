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
	nodediagnostics "github.com/openshift/origin/pkg/diagnostics/node"
	systemddiagnostics "github.com/openshift/origin/pkg/diagnostics/systemd"
	diagnostictypes "github.com/openshift/origin/pkg/diagnostics/types/diagnostic"
)

const (
	NodeDiagnosticsRecommendedName = "node"

	StandardNodeConfigPath string = "/etc/openshift/node/node-config.yaml"
)

var (
	AvailableNodeDiagnostics = util.NewStringSet("AnalyzeLogs", "UnitStatus", "NodeConfigCheck")
)

// user options for openshift-diagnostics client command
type NodeDiagnosticsOptions struct {
	RequestedDiagnostics util.StringList

	NodeConfigLocation string

	LogOptions *log.LoggerOptions
	Logger     *log.Logger
}

const longNodeDescription = `
OpenShift Diagnostics

This command helps you understand and troubleshoot a running OpenShift
node. It is intended to be run from the same context as the node
(where "openshift start" or "openshift start node" is run, possibly from
systemd or inside a container) and with the same configuration options.

    $ %s
`

func NewNodeCommand(name string, fullName string, out io.Writer) *cobra.Command {
	o := &NodeDiagnosticsOptions{
		RequestedDiagnostics: AvailableNodeDiagnostics.List(),
		LogOptions:           &log.LoggerOptions{Out: out},
	}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Troubleshoot an OpenShift v3 node.",
		Long:  fmt.Sprintf(longNodeDescription, fullName),
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

	cmd.Flags().StringVar(&o.NodeConfigLocation, "node-config", "", "path to node config file")
	diagnosticflags.BindLoggerOptionFlags(cmd.Flags(), o.LogOptions, diagnosticflags.RecommendedLoggerOptionFlags())
	diagnosticflags.BindDiagnosticFlag(cmd.Flags(), &o.RequestedDiagnostics, diagnosticflags.NewRecommendedDiagnosticFlag())

	return cmd
}

func (o *NodeDiagnosticsOptions) Complete() error {
	// set the node config location if it hasn't been set and we find it in an expected location
	if len(o.NodeConfigLocation) == 0 {
		if _, err := os.Stat(StandardNodeConfigPath); !os.IsNotExist(err) {
			o.NodeConfigLocation = StandardNodeConfigPath
		}
	}

	var err error
	o.Logger, err = o.LogOptions.NewLogger()
	if err != nil {
		return err
	}

	return nil
}

func (o NodeDiagnosticsOptions) RunDiagnostics() (bool, error) {
	diagnostics := map[string]diagnostictypes.Diagnostic{}

	// if we don't have a node config file, then there's no work to do
	if len(o.NodeConfigLocation) == 0 {
		// TODO remove NodeConfigCheck from the list
	}

	systemdUnits := systemddiagnostics.GetSystemdUnits(o.Logger)

	for _, diagnosticName := range o.RequestedDiagnostics {
		switch diagnosticName {
		case "AnalyzeLogs":
			diagnostics[diagnosticName] = systemddiagnostics.AnalyzeLogs{systemdUnits, o.Logger}

		case "UnitStatus":
			diagnostics[diagnosticName] = systemddiagnostics.UnitStatus{systemdUnits, o.Logger}

		case "NodeConfigCheck":
			diagnostics[diagnosticName] = nodediagnostics.NodeConfigCheck{o.NodeConfigLocation, o.Logger}

		default:
			return false, fmt.Errorf("unknown diagnostic: %v", diagnosticName)
		}
	}

	for name, diagnostic := range diagnostics {
		if canRun, reason := diagnostic.CanRun(); !canRun {
			if reason == nil {
				o.Logger.Noticem(log.Message{ID: "diagSkip", Template: "Skipping diagnostic: {{.area}}.{{.name}}\nDescription: {{.diag}}", TemplateData: map[string]string{"area": "node", "name": name, "diag": diagnostic.Description()}})
			} else {
				o.Logger.Noticem(log.Message{ID: "diagSkip", Template: "Skipping diagnostic: {{.area}}.{{.name}}\nDescription: {{.diag}}\nBecause: {{.reason}}", TemplateData: map[string]string{"area": "node", "name": name, "diag": diagnostic.Description(), "reason": reason.Error()}})
			}
			continue
		}

		o.Logger.Noticem(log.Message{ID: "diagRun", Template: "Running diagnostic: {{.area}}.{{.name}}\nDescription: {{.diag}}", TemplateData: map[string]string{"area": "node", "name": name, "diag": diagnostic.Description()}})
		diagnostic.Check()
	}

	return o.Logger.ErrorsSeen(), nil
}
