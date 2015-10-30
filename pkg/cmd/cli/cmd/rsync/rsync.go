package rsync

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	// RsyncRecommendedName is the recommended name for the rsync command
	RsyncRecommendedName = "rsync"

	rsyncLong = `
Copy local files to or from a pod container

This command will copy local files to or from a remote container.
It only copies the changed files using the rsync command from your OS.
To ensure optimum performance, install rsync locally. In UNIX systems,
use your package manager. In Windows, install cwRsync from
https://www.itefix.net/cwrsync.

If no container is specified, the first container of the pod is used
for the copy.`

	rsyncExample = `
  # Synchronize a local directory with a pod directory
  $ %[1]s ./local/dir/ POD:/remote/dir

  # Synchronize a pod directory with a local directory
  $ %[1]s POD:/remote/dir/ ./local/dir`

	noRsyncUnixWarning    = "WARNING: rsync command not found in path. Please use your package manager to install it."
	noRsyncWindowsWarning = "WARNING: rsync command not found in path. Download cwRsync for Windows and add it to your PATH."
)

// copyStrategy
type copyStrategy interface {
	Copy(source, destination *pathSpec, out, errOut io.Writer) error
	Validate() error
	String() string
}

// executor executes commands
type executor interface {
	Execute(command []string, in io.Reader, out, err io.Writer) error
}

// forwarder forwards pod ports to the local machine
type forwarder interface {
	ForwardPorts(ports []string, stopChan <-chan struct{}) error
}

// RsyncOptions holds the options to execute the sync command
type RsyncOptions struct {
	Namespace     string
	ContainerName string
	Source        *pathSpec
	Destination   *pathSpec
	Strategy      copyStrategy
	StrategyName  string
	Quiet         bool
	Delete        bool

	Out    io.Writer
	ErrOut io.Writer
}

// NewCmdRsync creates a new sync command
func NewCmdRsync(name, parent string, f *clientcmd.Factory, out, errOut io.Writer) *cobra.Command {
	o := RsyncOptions{
		Out:    out,
		ErrOut: errOut,
	}
	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s SOURCE DESTINATION", name),
		Short:   "Copy files between local filesystem and a pod",
		Long:    rsyncLong,
		Example: fmt.Sprintf(rsyncExample, parent+" "+name),
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, c, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.RunRsync())
		},
	}

	cmd.Flags().StringVarP(&o.ContainerName, "container", "c", "", "Container within the pod")
	cmd.Flags().StringVar(&o.StrategyName, "strategy", "", "Specify which strategy to use for copy: rsync, rsync-daemon, or tar")

	// Flags for rsync options, Must match rsync flag names
	cmd.Flags().BoolVarP(&o.Quiet, "quiet", "q", false, "Suppress non-error messages")
	cmd.Flags().BoolVar(&o.Delete, "delete", false, "Delete files not present in source")
	return cmd
}

func warnNoRsync(out io.Writer) {
	if isWindows() {
		fmt.Fprintf(out, noRsyncWindowsWarning)
		return
	}
	fmt.Fprintf(out, noRsyncUnixWarning)
}

func (o *RsyncOptions) determineStrategy(f *clientcmd.Factory, cmd *cobra.Command, name string) (copyStrategy, error) {
	switch name {
	case "":
		// Default case, use an rsync strategy first and then fallback to Tar
		strategies := copyStrategies{}
		if hasLocalRsync() {
			if isWindows() {
				strategy, err := newRsyncDaemonStrategy(f, cmd, o)
				if err != nil {
					return nil, err
				}
				strategies = append(strategies, strategy)
			} else {
				strategy, err := newRsyncStrategy(f, cmd, o)
				if err != nil {
					return nil, err
				}
				strategies = append(strategies, strategy)
			}
		} else {
			warnNoRsync(o.Out)
		}
		strategy, err := newTarStrategy(f, cmd, o)
		if err != nil {
			return nil, err
		}
		strategies = append(strategies, strategy)
		return strategies, nil
	case "rsync":
		return newRsyncStrategy(f, cmd, o)

	case "rsync-daemon":
		return newRsyncDaemonStrategy(f, cmd, o)

	case "tar":
		return newTarStrategy(f, cmd, o)

	default:
		return nil, fmt.Errorf("unknown strategy: %s", name)
	}
}

// Complete verifies command line arguments and loads data from the command environment
func (o *RsyncOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string) error {
	switch n := len(args); {
	case n == 0:
		cmd.Help()
		fallthrough
	case n < 2:
		return kcmdutil.UsageError(cmd, "SOURCE_DIR and POD:DESTINATION_DIR are required arguments")
	case n > 2:
		return kcmdutil.UsageError(cmd, "only SOURCE_DIR and POD:DESTINATION_DIR should be specified as arguments")
	}

	// Set main command arguments
	var err error
	o.Source, err = parsePathSpec(args[0])
	if err != nil {
		return err
	}
	o.Destination, err = parsePathSpec(args[1])
	if err != nil {
		return err
	}

	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}
	o.Namespace = namespace

	o.Strategy, err = o.determineStrategy(f, cmd, o.StrategyName)
	if err != nil {
		return err
	}

	return nil
}

// Validate checks that SyncOptions has all necessary fields
func (o *RsyncOptions) Validate() error {
	if o.Out == nil || o.ErrOut == nil {
		return errors.New("output and error streams must be specified")
	}
	if o.Source == nil || o.Destination == nil {
		return errors.New("source and destination must be specified")
	}
	if err := o.Source.Validate(); err != nil {
		return err
	}
	if err := o.Destination.Validate(); err != nil {
		return err
	}
	// If source and destination are both local or both remote throw an error
	if o.Source.Local() == o.Destination.Local() {
		return errors.New("rsync is only valid between a local directory and a pod directory; " +
			"specify a pod directory as [PODNAME]:[DIR]")
	}
	if err := o.Strategy.Validate(); err != nil {
		return err
	}

	return nil
}

// RunRsync copies files from source to destination
func (o *RsyncOptions) RunRsync() error {
	return o.Strategy.Copy(o.Source, o.Destination, o.Out, o.ErrOut)
}

// PodName returns the name of the pod as specified in either the
// the source or destination arguments
func (o *RsyncOptions) PodName() string {
	if len(o.Source.PodName) > 0 {
		return o.Source.PodName
	}
	return o.Destination.PodName
}
