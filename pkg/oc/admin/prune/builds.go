package prune

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildinternalclient "github.com/openshift/origin/pkg/build/generated/internalclientset"
	"github.com/openshift/origin/pkg/oc/cli/builds/prune"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

const PruneBuildsRecommendedName = "builds"

var (
	buildsLongDesc = templates.LongDesc(`
		Prune old completed and failed builds

		By default, the prune operation performs a dry run making no changes to internal registry. A
		--confirm flag is needed for changes to be effective.`)

	buildsExample = templates.Examples(`
		# Dry run deleting older completed and failed builds and also including
	  # all builds whose associated BuildConfig no longer exists
	  %[1]s %[2]s --orphans

	  # To actually perform the prune operation, the confirm flag must be appended
	  %[1]s %[2]s --orphans --confirm`)
)

// PruneBuildsOptions holds all the required options for pruning builds.
type PruneBuildsOptions struct {
	Confirm         bool
	Orphans         bool
	KeepYoungerThan time.Duration
	KeepComplete    int
	KeepFailed      int
	Namespace       string

	BuildClient buildinternalclient.Interface
	Out         io.Writer
}

// NewCmdPruneBuilds implements the OpenShift cli prune builds command.
func NewCmdPruneBuilds(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {
	opts := &PruneBuildsOptions{
		Confirm:         false,
		Orphans:         false,
		KeepYoungerThan: 60 * time.Minute,
		KeepComplete:    5,
		KeepFailed:      1,
	}
	cmd := &cobra.Command{
		Use:     name,
		Short:   "Remove old completed and failed builds",
		Long:    buildsLongDesc,
		Example: fmt.Sprintf(buildsExample, parentName, name),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(opts.Complete(f, cmd, args, out))
			kcmdutil.CheckErr(opts.Validate())
			kcmdutil.CheckErr(opts.Run())
		},
	}

	cmd.Flags().BoolVar(&opts.Confirm, "confirm", opts.Confirm, "If true, specify that build pruning should proceed. Defaults to false, displaying what would be deleted but not actually deleting anything.")
	cmd.Flags().BoolVar(&opts.Orphans, "orphans", opts.Orphans, "If true, prune all builds whose associated BuildConfig no longer exists and whose status is complete, failed, error, or cancelled.")
	cmd.Flags().DurationVar(&opts.KeepYoungerThan, "keep-younger-than", opts.KeepYoungerThan, "Specify the minimum age of a Build for it to be considered a candidate for pruning.")
	cmd.Flags().IntVar(&opts.KeepComplete, "keep-complete", opts.KeepComplete, "Per BuildConfig, specify the number of builds whose status is complete that will be preserved.")
	cmd.Flags().IntVar(&opts.KeepFailed, "keep-failed", opts.KeepFailed, "Per BuildConfig, specify the number of builds whose status is failed, error, or cancelled that will be preserved.")

	return cmd
}

// Complete turns a partially defined PruneBuildsOptions into a solvent structure
// which can be validated and used for pruning builds.
func (o *PruneBuildsOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string, out io.Writer) error {
	if len(args) > 0 {
		return kcmdutil.UsageErrorf(cmd, "no arguments are allowed to this command")
	}

	o.Namespace = metav1.NamespaceAll
	if cmd.Flags().Lookup("namespace").Changed {
		var err error
		o.Namespace, _, err = f.DefaultNamespace()
		if err != nil {
			return err
		}
	}
	o.Out = out

	config, err := f.ClientConfig()
	if err != nil {
		return err
	}
	buildClient, err := buildinternalclient.NewForConfig(config)
	if err != nil {
		return err
	}
	o.BuildClient = buildClient

	return nil
}

// Validate ensures that a PruneBuildsOptions is valid and can be used to execute pruning.
func (o PruneBuildsOptions) Validate() error {
	if o.KeepYoungerThan < 0 {
		return fmt.Errorf("--keep-younger-than must be greater than or equal to 0")
	}
	if o.KeepComplete < 0 {
		return fmt.Errorf("--keep-complete must be greater than or equal to 0")
	}
	if o.KeepFailed < 0 {
		return fmt.Errorf("--keep-failed must be greater than or equal to 0")
	}
	return nil
}

// Run contains all the necessary functionality for the OpenShift cli prune builds command.
func (o PruneBuildsOptions) Run() error {
	buildConfigList, err := o.BuildClient.Build().BuildConfigs(o.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	buildConfigs := []*buildapi.BuildConfig{}
	for i := range buildConfigList.Items {
		buildConfigs = append(buildConfigs, &buildConfigList.Items[i])
	}

	buildList, err := o.BuildClient.Build().Builds(o.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	builds := []*buildapi.Build{}
	for i := range buildList.Items {
		builds = append(builds, &buildList.Items[i])
	}

	options := prune.PrunerOptions{
		KeepYoungerThan: o.KeepYoungerThan,
		Orphans:         o.Orphans,
		KeepComplete:    o.KeepComplete,
		KeepFailed:      o.KeepFailed,
		BuildConfigs:    buildConfigs,
		Builds:          builds,
	}
	pruner := prune.NewPruner(options)

	w := tabwriter.NewWriter(o.Out, 10, 4, 3, ' ', 0)
	defer w.Flush()

	buildDeleter := &describingBuildDeleter{w: w}

	if o.Confirm {
		buildDeleter.delegate = buildclient.NewClientBuildClient(o.BuildClient)
	} else {
		fmt.Fprintln(os.Stderr, "Dry run enabled - no modifications will be made. Add --confirm to remove builds")
	}

	return pruner.Prune(buildDeleter)
}

// describingBuildDeleter prints information about each build it removes.
// If a delegate exists, its DeleteBuild function is invoked prior to returning.
type describingBuildDeleter struct {
	w             io.Writer
	delegate      buildclient.BuildDeleter
	headerPrinted bool
}

var _ buildclient.BuildDeleter = &describingBuildDeleter{}

func (p *describingBuildDeleter) DeleteBuild(build *buildapi.Build) error {
	if !p.headerPrinted {
		p.headerPrinted = true
		fmt.Fprintln(p.w, "NAMESPACE\tNAME")
	}

	fmt.Fprintf(p.w, "%s\t%s\n", build.Namespace, build.Name)

	if p.delegate == nil {
		return nil
	}

	if err := p.delegate.DeleteBuild(build); err != nil {
		return err
	}

	return nil
}
