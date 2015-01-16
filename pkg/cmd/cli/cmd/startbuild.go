package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kubecmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"

	build "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/util"
)

func NewCmdStartBuild(f *Factory, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start-build (<buildConfig>|--from-build=<build>)",
		Short: "Starts a new build from existing build or buildConfig",
		Long: `
Manually starts build from existing build or buildConfig

NOTE: This command is experimental and is subject to change in the future.

Examples:
  $ kubectl start-build 3bd2ug53b
  <Starts build from buildConfig matching the name "3bd2ug53b">

  $ kubectl start-build --from-build=3bd2ug53b
  <Starts build from build matching the name "3bd2ug53b">`,
		Run: func(cmd *cobra.Command, args []string) {
			buildName := kubecmd.GetFlagString(cmd, "from-build")
			if len(args) != 1 && len(buildName) == 0 {
				usageError(cmd, "Must pass a name of buildConfig or specify build name with '--from-build' flag")
			}

			client, _, err := f.Clients(cmd)
			checkErr(err)

			namespace := getOriginNamespace(cmd)

			var newBuild *build.Build
			if len(buildName) == 0 {
				// from build config
				config, err := client.BuildConfigs(namespace).Get(args[0])
				checkErr(err)

				newBuild = util.GenerateBuildFromConfig(config, nil, nil)
			} else {
				build, err := client.Builds(namespace).Get(buildName)
				checkErr(err)

				newBuild = util.GenerateBuildFromBuild(build)
			}

			newBuild, err = client.Builds(namespace).Create(newBuild)
			checkErr(err)
			fmt.Fprintf(out, "%s\n", newBuild.Name)
		},
	}
	cmd.Flags().StringP("from-build", "", "", "Specify the name of a build which should be re-run")
	return cmd
}
