package cmd

import (
	"fmt"
	"io"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/spf13/cobra"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
	osclient "github.com/openshift/origin/pkg/client"
)

func NewCmdStartBuild(f *Factory, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start-build (<buildConfig>|--from-build=<build>)",
		Short: "Starts a new build from existing build or buildConfig",
		Long: `
Manually starts build from existing build or buildConfig

NOTE: This command is experimental and is subject to change in the future.

Examples:
  $ osc start-build 3bd2ug53b
  <Starts build from buildConfig matching the name "3bd2ug53b">

  $ osc start-build --from-build=3bd2ug53b
  <Starts build from build matching the name "3bd2ug53b">`,
		Run: func(cmd *cobra.Command, args []string) {
			buildName := cmdutil.GetFlagString(cmd, "from-build")
			if len(args) != 1 && len(buildName) == 0 {
				usageError(cmd, "Must pass a name of buildConfig or specify build name with '--from-build' flag")
			}

			client, _, err := f.Clients(cmd)
			checkErr(err)

			namespace, err := f.DefaultNamespace(cmd)
			checkErr(err)

			var newBuild *buildapi.Build
			if len(buildName) == 0 {
				// from build config
				config, err := client.BuildConfigs(namespace).Get(args[0])
				checkErr(err)

				newBuild, err = buildutil.GenerateBuildWithImageTag(config, nil, client.ImageRepositories(kapi.NamespaceAll).(osclient.ImageRepositoryNamespaceGetter))
				checkErr(err)
			} else {
				build, err := client.Builds(namespace).Get(buildName)
				checkErr(err)

				newBuild = buildutil.GenerateBuildFromBuild(build)
			}

			newBuild, err = client.Builds(namespace).Create(newBuild)
			checkErr(err)
			fmt.Fprintf(out, "%s\n", newBuild.Name)
		},
	}
	cmd.Flags().StringP("from-build", "", "", "Specify the name of a build which should be re-run")
	return cmd
}
