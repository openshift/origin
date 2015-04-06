package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const startBuildLongDesc = `
Manually starts build from existing build or buildConfig

NOTE: This command is experimental and is subject to change in the future.

Examples:

	# Starts build from build configuration matching the name "3bd2ug53b"
	$ %[1]s start-build 3bd2ug53b

	# Starts build from build matching the name "3bd2ug53b"
	$ %[1]s start-build --from-build=3bd2ug53b

	# Starts build from build configuration matching the name "3bd2ug53b" and watches the logs until the build completes or fails
	$ %[1]s start-build 3bd2ug53b --follow
`

func NewCmdStartBuild(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start-build (<buildConfig>|--from-build=<build>)",
		Short: "Starts a new build from existing build or buildConfig",
		Long:  fmt.Sprintf(startBuildLongDesc, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			err := RunStartBuild(f, out, cmd, args)
			cmdutil.CheckErr(err)
		},
	}
	cmd.Flags().String("from-build", "", "Specify the name of a build which should be re-run")
	cmd.Flags().Bool("follow", false, "Start a build and watch its logs until it completes or fails")
	return cmd
}

func RunStartBuild(f *clientcmd.Factory, out io.Writer, cmd *cobra.Command, args []string) error {
	buildName := cmdutil.GetFlagString(cmd, "from-build")
	follow := cmdutil.GetFlagBool(cmd, "follow")
	if len(args) != 1 && len(buildName) == 0 {
		return cmdutil.UsageError(cmd, "Must pass a name of buildConfig or specify build name with '--from-build' flag")
	}

	client, _, err := f.Clients()
	if err != nil {
		return err
	}

	namespace, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	var newBuild *buildapi.Build
	if len(buildName) == 0 {
		request := &buildapi.BuildRequest{
			ObjectMeta: kapi.ObjectMeta{Name: args[0]},
		}
		newBuild, err = client.BuildConfigs(namespace).Instantiate(request)
		if err != nil {
			return err
		}
	} else {
		request := &buildapi.BuildRequest{
			ObjectMeta: kapi.ObjectMeta{Name: buildName},
		}
		newBuild, err = client.Builds(namespace).Clone(request)
		if err != nil {
			return err
		}
	}

	if follow {
		set := labels.Set(newBuild.Labels)
		selector := labels.SelectorFromSet(set)

		// Add a watcher for the build about to start
		watcher, err := client.Builds(namespace).Watch(selector, fields.Everything(), newBuild.ResourceVersion)
		if err != nil {
			return err
		}
		defer watcher.Stop()

		for event := range watcher.ResultChan() {
			build, ok := event.Object.(*buildapi.Build)
			if !ok {
				return fmt.Errorf("cannot convert input to Build")
			}

			// Iterate over watcher's results and search for
			// the build we just started. Also make sure that
			// the build is running, complete, or has failed
			if build.Name == newBuild.Name {
				switch build.Status {
				case buildapi.BuildStatusRunning, buildapi.BuildStatusComplete, buildapi.BuildStatusFailed:
					rd, err := client.BuildLogs(namespace).Get(newBuild.Name).Stream()
					if err != nil {
						return err
					}
					defer rd.Close()

					_, err = io.Copy(out, rd)
					if err != nil {
						return err
					}
					break
				}

				if build.Status == buildapi.BuildStatusComplete || build.Status == buildapi.BuildStatusFailed {
					break
				}
			}
		}
	}

	fmt.Fprintf(out, "%s\n", newBuild.Name)
	return nil
}
