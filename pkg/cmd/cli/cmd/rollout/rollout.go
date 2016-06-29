package rollout

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/kubectl/cmd/rollout"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	rolloutLong = `
Manage deployments.
`
)

// NewCmdRollout facilitates kubectl rollout subcommands
func NewCmdRollout(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rollout SUBCOMMAND",
		Short: "rollout manages a deployment",
		Long:  rolloutLong,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	// subcommands
	cmd.AddCommand(NewCmdRolloutHistory(fullName, f, out))
	cmd.AddCommand(NewCmdRolloutPause(fullName, f, out))
	cmd.AddCommand(NewCmdRolloutResume(fullName, f, out))
	cmd.AddCommand(NewCmdRolloutUndo(fullName, f, out))

	return cmd
}

const (
	rolloutHistoryLong = `
View the history of rollouts for a specific deployment config

You can also view more detailed information for a specific revision
by using the --revision flag.
`

	rolloutHistoryExample = `  # View the rollout history of a deployment
  %[1]s rollout history dc/nginx

  # View the details of deployment revision 3
  %[1]s rollout history dc/nginx --revision=3`
)

// NewCmdRolloutHistory is a wrapper for the Kubernetes cli rollout history command
func NewCmdRolloutHistory(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := rollout.NewCmdRolloutHistory(f.Factory, out)
	cmd.Long = rolloutHistoryLong
	cmd.Example = fmt.Sprintf(rolloutHistoryExample, fullName)
	return cmd
}

const (
	rolloutPauseLong = `
Mark the provided resource as paused

Paused resources will not be reconciled by a controller.
Use \"%[1]s rollout resume\" to resume a paused resource.`

	rolloutPauseExample = `  # Mark the nginx deployment as paused. Any current state of
  # the deployment will continue its function, new updates to the deployment will not
  # have an effect as long as the deployment is paused.
  %[1]s rollout pause dc/nginx`
)

// NewCmdRolloutPause is a wrapper for the Kubernetes cli rollout pause command
func NewCmdRolloutPause(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := rollout.NewCmdRolloutPause(f.Factory, out)
	cmd.Long = rolloutPauseLong
	cmd.Example = fmt.Sprintf(rolloutPauseExample, fullName)
	return cmd
}

const (
	rolloutResumeLong = `
Resume a paused resource

Paused resources will not be reconciled by a controller. By resuming a
resource, we allow it to be reconciled again.`

	rolloutResumeExample = `  # Resume an already paused deployment
  %[1]s rollout resume dc/nginx`
)

// NewCmdRolloutResume is a wrapper for the Kubernetes cli rollout resume command
func NewCmdRolloutResume(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := rollout.NewCmdRolloutResume(f.Factory, out)
	cmd.Long = rolloutResumeLong
	cmd.Example = fmt.Sprintf(rolloutResumeExample, fullName)
	return cmd
}

const (
	rolloutUndoLong = `
Revert an application back to a previous deployment

When you run this command your deployment configuration will be updated to
match a previous deployment. By default only the pod and container
configuration will be changed and scaling or trigger settings will be left as-
is. Note that environment variables and volumes are included in rollbacks, so
if you've recently updated security credentials in your environment your
previous deployment may not have the correct values.

Any image triggers present in the rolled back configuration will be disabled
with a warning. This is to help prevent your rolled back deployment from being
replaced by a triggered deployment soon after your rollback. To re-enable the
triggers, use the 'deploy --enable-triggers' command.

If you would like to review the outcome of the rollback, pass '--dry-run' to print
a human-readable representation of the updated deployment configuration instead of
executing the rollback. This is useful if you're not quite sure what the outcome
will be.`

	rolloutUndoExample = `  # Rollback to the previous deployment
  %[1]s rollout undo dc/nginx

  # Rollback to deployment revision 3. The replication controller for that version must exist.
  %[1]s rollout undo dc/nginx --to-revision=3`
)

// NewCmdRolloutUndo is a wrapper for the Kubernetes cli rollout undo command
func NewCmdRolloutUndo(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := rollout.NewCmdRolloutUndo(f.Factory, out)
	cmd.Long = rolloutUndoLong
	cmd.Example = fmt.Sprintf(rolloutUndoExample, fullName)
	return cmd
}
