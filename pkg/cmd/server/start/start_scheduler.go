package start

import (
	"fmt"
	"io"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/server/origin"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

var schedulerLong = templates.LongDesc(`
	Start the master scheduler

	This command starts the scheduler for the master.  Running

	    %[1]s start master %[2]s

	will start the the scheduler. The scheduler will run in the foreground until you terminate the process.`)

// NewCommandStartMasterScheduler starts only the scheduler
func NewCommandStartMasterScheduler(name, basename string, out, errout io.Writer) (*cobra.Command, *MasterOptions) {
	options := &MasterOptions{Output: out}
	options.DefaultsFromName(basename)

	cmd := &cobra.Command{
		Use:   "scheduler",
		Short: "Launch master Scheduler",
		Long:  fmt.Sprintf(schedulerLong, basename, name),
		Run: func(c *cobra.Command, args []string) {
			if err := options.Complete(); err != nil {
				fmt.Fprintln(errout, kcmdutil.UsageErrorf(c, err.Error()))
				return
			}

			if len(options.ConfigFile) == 0 {
				fmt.Fprintln(errout, kcmdutil.UsageErrorf(c, "--config is required for this command"))
				return
			}

			if err := options.Validate(args); err != nil {
				fmt.Fprintln(errout, kcmdutil.UsageErrorf(c, err.Error()))
				return
			}

			origin.StartProfiler()

			if err := options.StartMaster(); err != nil {
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

	options.MasterArgs = NewDefaultMasterArgs()
	options.MasterArgs.StartScheduler = true
	flags := cmd.Flags()
	// This command only supports reading from config and the listen argument
	flags.StringVar(&options.ConfigFile, "config", "", "Location of the master configuration file to run from. Required")
	cmd.MarkFlagFilename("config", "yaml", "yml")

	return cmd, options
}
