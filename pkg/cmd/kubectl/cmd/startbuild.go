package cmd

import (
	"io"

	"github.com/spf13/cobra"

	kubecmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/util"
)

func (f *OriginFactory) NewCmdStartBuild(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start-build <buildConfig>",
		Short: "Starts build from existing build or buildConfig",
		Long: `Manually starts build from existing build or buildConfig

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

			resourceName, resourceKind := buildName, "build"
			if len(resourceName) == 0 {
				resourceName, resourceKind = args[0], "buildConfig"
			}

			mapping, namespace, _ := kubecmd.ResourceOrTypeFromArgs(cmd, []string{resourceKind}, f.Mapper)
			client, err := f.GetRESTHelperFunc(cmd)(mapping)
			checkErr(err)
			resource, err := client.Get(namespace, resourceName, labels.Everything())
			checkErr(err)

			var newBuild *buildapi.Build
			switch resourceKind {
			case "build":
				newBuild = util.GenerateBuildFromBuild(resource.(*buildapi.Build))
			case "buildConfig":
				newBuild = util.GenerateBuildFromConfig(resource.(*buildapi.BuildConfig), nil)
			}

			mapping, namespace, _ = kubecmd.ResourceOrTypeFromArgs(cmd, []string{"build"}, f.Mapper)
			client, err = f.GetRESTHelperFunc(cmd)(mapping)
			checkErr(err)
			buildJSON, err := mapping.Codec.Encode(newBuild)
			checkErr(err)
			err = client.Create(namespace, true, buildJSON)
			checkErr(err)
		},
	}
	cmd.Flags().StringP("from-build", "", "", "Specify the name of a build which should be re-run")
	return cmd
}
