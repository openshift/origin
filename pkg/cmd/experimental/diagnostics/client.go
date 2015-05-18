package diagnostics

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kclientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	diagnosticflags "github.com/openshift/origin/pkg/cmd/experimental/diagnostics/options"
	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
	clientdiagnostics "github.com/openshift/origin/pkg/diagnostics/client"
	"github.com/openshift/origin/pkg/diagnostics/log"
	diagnostictypes "github.com/openshift/origin/pkg/diagnostics/types/diagnostic"
)

const ClientDiagnosticsRecommendedName = "client"

var (
	AvailableClientDiagnostics = util.NewStringSet("ConfigContexts", "NodeDefinitions")
)

// user options for openshift-diagnostics client command
type ClientDiagnosticsOptions struct {
	RequestedDiagnostics util.StringList

	KubeClient *kclient.Client
	KubeConfig *kclientcmdapi.Config

	LogOptions *log.LoggerOptions
	Logger     *log.Logger
}

const longClientDescription = `
OpenShift Diagnostics

This command helps you understand and troubleshoot OpenShift as a user. It is
intended to be run from the same context as an OpenShift client
("openshift cli" or "osc") and with the same configuration options.

    $ %s
`

func NewClientCommand(name string, fullName string, out io.Writer) *cobra.Command {
	o := &ClientDiagnosticsOptions{
		RequestedDiagnostics: AvailableClientDiagnostics.List(),
		LogOptions:           &log.LoggerOptions{Out: out},
	}

	var factory *osclientcmd.Factory

	cmd := &cobra.Command{
		Use:   name,
		Short: "Troubleshoot using the OpenShift v3 client.",
		Long:  fmt.Sprintf(longClientDescription, fullName),
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete())

			_, kubeClient, err := factory.Clients()
			kcmdutil.CheckErr(err)

			kubeConfig, err := factory.OpenShiftClientConfig.RawConfig()
			kcmdutil.CheckErr(err)

			o.KubeClient = kubeClient
			o.KubeConfig = &kubeConfig

			failed, err := o.RunDiagnostics()
			o.Logger.Summary()
			o.Logger.Finish()

			kcmdutil.CheckErr(err)
			if failed {
				os.Exit(255)
			}

		},
	}
	cmd.SetOutput(out)                     // for output re: usage / help
	factory = osclientcmd.New(cmd.Flags()) // side effect: add standard persistent flags for openshift client
	diagnosticflags.BindLoggerOptionFlags(cmd.Flags(), o.LogOptions, diagnosticflags.RecommendedLoggerOptionFlags())
	diagnosticflags.BindDiagnosticFlag(cmd.Flags(), &o.RequestedDiagnostics, diagnosticflags.NewRecommendedDiagnosticFlag())

	return cmd
}

func (o *ClientDiagnosticsOptions) Complete() error {
	var err error
	o.Logger, err = o.LogOptions.NewLogger()
	if err != nil {
		return err
	}

	return nil
}

func (o ClientDiagnosticsOptions) RunDiagnostics() (bool, error) {
	diagnostics := map[string]diagnostictypes.Diagnostic{}

	for _, diagnosticName := range o.RequestedDiagnostics {
		switch diagnosticName {
		case "ConfigContexts":
			for contextName, _ := range o.KubeConfig.Contexts {
				diagnostics[diagnosticName+"["+contextName+"]"] = clientdiagnostics.ConfigContext{o.KubeConfig, contextName, o.Logger}
			}

		case "NodeDefinitions":
			diagnostics[diagnosticName] = clientdiagnostics.NodeDefinition{o.KubeClient, o.Logger}

		default:
			return false, fmt.Errorf("unknown diagnostic: %v", diagnosticName)
		}
	}

	for name, diagnostic := range diagnostics {

		if canRun, reason := diagnostic.CanRun(); !canRun {
			if reason == nil {
				o.Logger.Noticem(log.Message{ID: "diagSkip", Template: "Skipping diagnostic: {{.area}}.{{.name}}\nDescription: {{.diag}}", TemplateData: map[string]string{"area": "client", "name": name, "diag": diagnostic.Description()}})
			} else {
				o.Logger.Noticem(log.Message{ID: "diagSkip", Template: "Skipping diagnostic: {{.area}}.{{.name}}\nDescription: {{.diag}}\nBecause: {{.reason}}", TemplateData: map[string]string{"area": "client", "name": name, "diag": diagnostic.Description(), "reason": reason.Error()}})
			}
			continue
		}

		o.Logger.Noticem(log.Message{ID: "diagRun", Template: "Running diagnostic: {{.area}}.{{.name}}\nDescription: {{.diag}}", TemplateData: map[string]string{"area": "client", "name": name, "diag": diagnostic.Description()}})
		diagnostic.Check()
	}

	return o.Logger.ErrorsSeen(), nil
}
