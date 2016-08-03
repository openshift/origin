package rsync

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/sets"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

// rsyncStrategy implements the rsync copy strategy
// The rsync strategy calls the local rsync command directly and passes the OpenShift
// CLI rsh command as the remote shell command for rsync. It requires that rsync be
// present in both the client machine and the remote container.
type rsyncStrategy struct {
	Flags          []string
	RshCommand     string
	LocalExecutor  executor
	RemoteExecutor executor
	podChecker     podChecker
}

// rshExcludeFlags are flags that are passed to oc rsync, and should not be passed on to the underlying command being invoked via oc rsh.
var rshExcludeFlags = sets.NewString("delete", "strategy", "quiet", "include", "exclude", "progress", "no-perms", "watch")

func newRsyncStrategy(f *clientcmd.Factory, c *cobra.Command, o *RsyncOptions) (copyStrategy, error) {
	// Determine the rsh command to pass to the local rsync command
	rsh := cmdutil.SiblingCommand(c, "rsh")
	rshCmd := []string{rsh}
	// Append all original flags to rsh command
	c.Flags().Visit(func(flag *pflag.Flag) {
		if rshExcludeFlags.Has(flag.Name) {
			return
		}
		rshCmd = append(rshCmd, fmt.Sprintf("--%s=%s", flag.Name, flag.Value.String()))
	})
	rshCmdStr := strings.Join(rshCmd, " ")
	glog.V(4).Infof("Rsh command: %s", rshCmdStr)

	remoteExec, err := newRemoteExecutor(f, o)
	if err != nil {
		return nil, err
	}
	// The blocking-io flag is used to resolve a sync issue when
	// copying from the pod to the local machine
	flags := []string{"-a", "--blocking-io", "--omit-dir-times", "--numeric-ids"}
	flags = append(flags, rsyncFlagsFromOptions(o)...)

	podName := o.Source.PodName
	if o.Source.Local() {
		podName = o.Destination.PodName
	}
	client, err := f.Client()
	if err != nil {
		return nil, err
	}

	return &rsyncStrategy{
		Flags:          flags,
		RshCommand:     rshCmdStr,
		RemoteExecutor: remoteExec,
		LocalExecutor:  newLocalExecutor(),
		podChecker:     podAPIChecker{client, o.Namespace, podName},
	}, nil
}

func (r *rsyncStrategy) Copy(source, destination *pathSpec, out, errOut io.Writer) error {
	glog.V(3).Infof("Copying files with rsync")
	cmd := append([]string{"rsync"}, r.Flags...)
	cmd = append(cmd, "-e", r.RshCommand, source.RsyncPath(), destination.RsyncPath())
	errBuf := &bytes.Buffer{}
	err := r.LocalExecutor.Execute(cmd, nil, out, errBuf)
	if isExitError(err) {
		// Check if pod exists
		if podCheckErr := r.podChecker.CheckPod(); podCheckErr != nil {
			return podCheckErr
		}
		// Determine whether rsync is present in the pod container
		testRsyncErr := checkRsync(r.RemoteExecutor)
		if testRsyncErr != nil {
			return strategySetupError("rsync not available in container")
		}
	}
	io.Copy(errOut, errBuf)
	return err
}

func (r *rsyncStrategy) Validate() error {
	errs := []error{}
	if len(r.RshCommand) == 0 {
		errs = append(errs, errors.New("rsh command must be provided"))
	}
	if r.LocalExecutor == nil {
		errs = append(errs, errors.New("local executor must not be nil"))
	}
	if r.RemoteExecutor == nil {
		errs = append(errs, errors.New("remote executor must not be nil"))
	}
	if len(errs) > 0 {
		return kerrors.NewAggregate(errs)
	}
	return nil
}

func (r *rsyncStrategy) String() string {
	return "rsync"
}
