package cmd

import (
	"fmt"
	"io"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	cancelBuildLong = `
Cancels a pending or running build

This command requests a graceful shutdown of the running build. There may be a delay between requesting 
the build and the time the build is terminated.`

	cancelBuildExample = `  # Cancel the build with the given name
  $ %[1]s cancel-build 1da32cvq

  # Cancel the named build and print the build logs
  $ %[1]s cancel-build 1da32cvq --dump-logs

  # Cancel the named build and create a new one with the same parameters
  $ %[1]s cancel-build 1da32cvq --restart`
)

// NewCmdCancelBuild implements the OpenShift cli cancel-build command
func NewCmdCancelBuild(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "cancel-build BUILD",
		Short:   "Cancel a pending or running build",
		Long:    cancelBuildLong,
		Example: fmt.Sprintf(cancelBuildExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			err := RunCancelBuild(f, out, cmd, args)
			cmdutil.CheckErr(err)
		},
	}

	cmd.Flags().Bool("dump-logs", false, "Specify if the build logs for the cancelled build should be shown.")
	cmd.Flags().Bool("restart", false, "Specify if a new build should be created after the current build is cancelled.")
	return cmd
}

// RunCancelBuild contains all the necessary functionality for the OpenShift cli cancel-build command
func RunCancelBuild(f *clientcmd.Factory, out io.Writer, cmd *cobra.Command, args []string) error {
	if len(args) == 0 || len(args[0]) == 0 {
		return cmdutil.UsageError(cmd, "You must specify the name of a build to cancel.")
	}

	buildName := args[0]
	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	client, _, err := f.Clients()
	if err != nil {
		return err
	}
	buildClient := client.Builds(namespace)
	build, err := buildClient.Get(buildName)
	if err != nil {
		return err
	}

	if !isBuildCancellable(build) {
		return nil
	}

	// Print build logs before cancelling build.
	if cmdutil.GetFlagBool(cmd, "dump-logs") {
		opts := buildapi.BuildLogOptions{
			NoWait: true,
			Follow: false,
		}
		response, err := client.BuildLogs(namespace).Get(buildName, opts).Do().Raw()
		if err != nil {
			glog.Errorf("Could not fetch build logs for %s: %v", buildName, err)
		} else {
			glog.Infof("Build logs for %s:\n%v", buildName, string(response))
		}
	}

	// Mark build to be cancelled.
	for {
		build.Status.Cancelled = true
		if _, err = buildClient.Update(build); err != nil && errors.IsConflict(err) {
			build, err = buildClient.Get(buildName)
			if err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		break
	}
	glog.V(2).Infof("Build %s was cancelled.", buildName)

	// Create a new build with the same configuration.
	if cmdutil.GetFlagBool(cmd, "restart") {
		request := &buildapi.BuildRequest{
			ObjectMeta: kapi.ObjectMeta{Name: build.Name},
		}
		newBuild, err := client.Builds(namespace).Clone(request)
		if err != nil {
			return err
		}
		glog.V(2).Infof("Restarted build %s.", buildName)
		fmt.Fprintf(out, "%s\n", newBuild.Name)
	} else {
		fmt.Fprintf(out, "%s\n", build.Name)
	}
	return nil
}

// isBuildCancellable checks if another cancellation event was triggered, and if the build status is correct.
func isBuildCancellable(build *buildapi.Build) bool {
	if build.Status.Phase != buildapi.BuildPhaseNew &&
		build.Status.Phase != buildapi.BuildPhasePending &&
		build.Status.Phase != buildapi.BuildPhaseRunning {

		glog.V(2).Infof("A build can be cancelled only if it has new/pending/running status.")
		return false
	}

	if build.Status.Cancelled {
		glog.V(2).Infof("A cancellation event was already triggered for the build %s.", build.Name)
		return false
	}
	return true
}
