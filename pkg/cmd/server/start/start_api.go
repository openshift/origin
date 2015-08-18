package start

import (
	"fmt"
	"io"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kerrors "k8s.io/kubernetes/pkg/api/errors"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/flagtypes"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

const apiLong = `Start the master API

This command starts the master API.  Running

  $ %[1]s start master %[2]s

will start the server listening for incoming API requests. The server
will run in the foreground until you terminate the process.`

// NewCommandStartMasterAPI starts only the APIserver
func NewCommandStartMasterAPI(name, basename string, out io.Writer) (*cobra.Command, *MasterOptions) {
	options := &MasterOptions{Output: out}
	options.DefaultsFromName(basename)

	cmd := &cobra.Command{
		Use:   name,
		Short: "Launch master API",
		Long:  fmt.Sprintf(apiLong, basename, name),
		Run: func(c *cobra.Command, args []string) {
			if err := options.Complete(); err != nil {
				fmt.Fprintln(c.Out(), kcmdutil.UsageError(c, err.Error()))
				return
			}

			if len(options.ConfigFile) == 0 {
				fmt.Fprintln(c.Out(), kcmdutil.UsageError(c, "--config is required for this command"))
				return
			}

			if err := options.Validate(args); err != nil {
				fmt.Fprintln(c.Out(), kcmdutil.UsageError(c, err.Error()))
				return
			}

			startProfiler()

			if err := options.StartMaster(); err != nil {
				if kerrors.IsInvalid(err) {
					if details := err.(*kerrors.StatusError).ErrStatus.Details; details != nil {
						fmt.Fprintf(c.Out(), "Invalid %s %s\n", details.Kind, details.Name)
						for _, cause := range details.Causes {
							fmt.Fprintf(c.Out(), "  %s: %s\n", cause.Field, cause.Message)
						}
						os.Exit(255)
					}
				}
				glog.Fatal(err)
			}
		},
	}

	masterAddr := flagtypes.Addr{
		Value:         "127.0.0.1:8443",
		DefaultScheme: "https",
		DefaultPort:   8443,
		AllowPrefix:   true,
	}.Default()

	options.MasterArgs = NewDefaultMasterArgs()
	options.MasterArgs.StartAPI = true
	options.MasterArgs.OverrideConfig = func(config *configapi.MasterConfig) error {
		if config.KubernetesMasterConfig != nil && masterAddr.Provided {
			config.KubernetesMasterConfig.MasterIP = masterAddr.Host
		}
		return nil
	}

	flags := cmd.Flags()
	// This command only supports reading from config and the override master address
	flags.StringVar(&options.ConfigFile, "config", "", "Location of the master configuration file to run from. Required")
	cmd.MarkFlagFilename("config", "yaml", "yml")
	flags.Var(&masterAddr, "master-ip", "The IP address the master should register for itself. Defaults to the master address from the config.")

	return cmd, options
}
