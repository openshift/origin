package prune

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/prune"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const buildsLongDesc = `%s %s - removes older completed and failed builds`

const PruneBuildsRecommendedName = "builds"

type pruneBuildsConfig struct {
	Confirm         bool
	KeepYoungerThan time.Duration
	Orphans         bool
	KeepComplete    int
	KeepFailed      int
}

func NewCmdPruneBuilds(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {
	cfg := &pruneBuildsConfig{
		Confirm:         false,
		KeepYoungerThan: 60 * time.Minute,
		Orphans:         false,
		KeepComplete:    5,
		KeepFailed:      1,
	}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Remove completed and failed builds",
		Long:  fmt.Sprintf(buildsLongDesc, parentName, name),

		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				glog.Fatalf("No arguments are allowed to this command")
			}

			osClient, _, err := f.Clients()
			if err != nil {
				cmdutil.CheckErr(err)
			}

			buildConfigList, err := osClient.BuildConfigs(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
			if err != nil {
				cmdutil.CheckErr(err)
			}

			buildList, err := osClient.Builds(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
			if err != nil {
				cmdutil.CheckErr(err)
			}

			buildConfigs := []*buildapi.BuildConfig{}
			for i := range buildConfigList.Items {
				buildConfigs = append(buildConfigs, &buildConfigList.Items[i])
			}

			builds := []*buildapi.Build{}
			for i := range buildList.Items {
				builds = append(builds, &buildList.Items[i])
			}

			var buildPruneFunc prune.PruneFunc

			w := tabwriter.NewWriter(out, 10, 4, 3, ' ', 0)
			defer w.Flush()

			describingPruneBuildFunc := func(build *buildapi.Build) error {
				fmt.Fprintf(w, "%s\t%s\n", build.Namespace, build.Name)
				return nil
			}

			switch cfg.Confirm {
			case true:
				buildPruneFunc = func(build *buildapi.Build) error {
					describingPruneBuildFunc(build)
					err := osClient.Builds(build.Namespace).Delete(build.Name)
					if err != nil {
						return err
					}
					return nil
				}
			default:
				fmt.Fprintln(os.Stderr, "Dry run enabled - no modifications will be made. Add --confirm to remove builds")
				buildPruneFunc = describingPruneBuildFunc
			}

			fmt.Fprintln(w, "NAMESPACE\tNAME")
			pruneTask := prune.NewPruneTasker(buildConfigs, builds, cfg.KeepYoungerThan, cfg.Orphans, cfg.KeepComplete, cfg.KeepFailed, buildPruneFunc)
			err = pruneTask.PruneTask()
			if err != nil {
				cmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().BoolVar(&cfg.Confirm, "confirm", cfg.Confirm, "Specify that build pruning should proceed. Defaults to false, displaying what would be deleted but not actually deleting anything.")
	cmd.Flags().BoolVar(&cfg.Orphans, "orphans", cfg.Orphans, "Prune all builds whose associated BuildConfig no longer exists and whose status is complete, failed, error, or canceled.")
	cmd.Flags().DurationVar(&cfg.KeepYoungerThan, "keep-younger-than", cfg.KeepYoungerThan, "Specify the minimum age of a Build for it to be considered a candidate for pruning.")
	cmd.Flags().IntVar(&cfg.KeepComplete, "keep-complete", cfg.KeepComplete, "Per BuildConfig, specify the number of builds whose status is complete that will be preserved.")
	cmd.Flags().IntVar(&cfg.KeepFailed, "keep-failed", cfg.KeepFailed, "Per BuildConfig, specify the number of builds whose status is failed, error, or canceled that will be preserved.")

	return cmd
}
