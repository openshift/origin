package prune

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/batch"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/job/prune"
)

const PruneJobsRecommendedName = "jobs"

const (
	jobsLongDesc = `Prune old completed and failed jobs

By default, the prune operation performs a dry run making no changes to the jobs.
A --confirm flag is needed for changes to be effective.
`

	jobsExample = `  # Dry run deleting all but the last completed job for every ScheduledJob
  %[1]s %[2]s --keep-complete=1

  # To actually perform the prune operation, the confirm flag must be appended
  %[1]s %[2]s --keep-complete=1 --confirm`
)

// PruneJobsOptions holds all the required options for pruning jobs.
type PruneJobsOptions struct {
	Confirm         bool
	KeepYoungerThan time.Duration
	KeepComplete    int
	KeepFailed      int
	Namespace       string

	KClient kclient.Interface
	Out     io.Writer
}

// NewCmdPruneJobs implements the OpenShift cli prune jobs command.
func NewCmdPruneJobs(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {
	opts := &PruneJobsOptions{
		Confirm:         false,
		KeepYoungerThan: 60 * time.Minute,
		KeepComplete:    5,
		KeepFailed:      1,
	}

	cmd := &cobra.Command{
		Use:        name,
		Short:      "Remove old completed and failed jobs",
		Long:       jobsLongDesc,
		Example:    fmt.Sprintf(jobsExample, parentName, name),
		SuggestFor: []string{"job", "jobs"},
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(opts.Complete(f, cmd, args, out))
			if err := opts.Validate(); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}
			kcmdutil.CheckErr(opts.Run())
		},
	}

	cmd.Flags().BoolVar(&opts.Confirm, "confirm", opts.Confirm, "Specify that jobs pruning should proceed. Defaults to false, displaying what would be deleted but not actually deleting anything.")
	cmd.Flags().DurationVar(&opts.KeepYoungerThan, "keep-younger-than", opts.KeepYoungerThan, "Specify the minimum age of a job for it to be considered a candidate for pruning.")
	cmd.Flags().IntVar(&opts.KeepComplete, "keep-complete", opts.KeepComplete, "Per ScheduledJob, specify the number of jobs whose status is completed that will be preserved.")
	cmd.Flags().IntVar(&opts.KeepFailed, "keep-failed", opts.KeepFailed, "Per ScheduledJob, specify the number of jobs whose status is failed that will be preserved.")

	return cmd
}

// Complete turns a partially defined PruneJobsOptions into a solvent structure
// which can be validated and used for pruning jobs.
func (o *PruneJobsOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string, out io.Writer) error {
	if len(args) > 0 {
		return kcmdutil.UsageError(cmd, "no arguments are allowed to this command")
	}

	o.Namespace = kapi.NamespaceAll
	if cmd.Flags().Lookup("namespace").Changed {
		var err error
		o.Namespace, _, err = f.DefaultNamespace()
		if err != nil {
			return err
		}
	}
	o.Out = out

	_, kClient, err := f.Clients()
	if err != nil {
		return err
	}
	o.KClient = kClient

	return nil
}

// Validate ensures that a PruneJobsOptions is valid and can be used to execute pruning.
func (o PruneJobsOptions) Validate() error {
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

// Run contains all the necessary functionality for the OpenShift cli prune jobs command.
func (o PruneJobsOptions) Run() error {
	scheduledJobList, err := o.KClient.Batch().ScheduledJobs(o.Namespace).List(kapi.ListOptions{})
	if err != nil {
		return err
	}
	scheduledJobs := make([]*batch.ScheduledJob, len(scheduledJobList.Items))
	for i := range scheduledJobList.Items {
		scheduledJobs[i] = &scheduledJobList.Items[i]
	}
	jobList, err := o.KClient.Batch().Jobs(o.Namespace).List(kapi.ListOptions{})
	if err != nil {
		return err
	}
	jobs := make([]*batch.Job, len(jobList.Items))
	for i := range jobList.Items {
		jobs[i] = &jobList.Items[i]
	}

	options := prune.PrunerOptions{
		KeepYoungerThan: o.KeepYoungerThan,
		KeepComplete:    o.KeepComplete,
		KeepFailed:      o.KeepFailed,
		ScheduledJobs:   scheduledJobs,
		Jobs:            jobs,
	}
	pruner := prune.NewPruner(options)

	w := tabwriter.NewWriter(o.Out, 10, 4, 3, ' ', 0)
	defer w.Flush()

	jobDeleter := &describingJobDeleter{w: w}

	if o.Confirm {
		jobDeleter.delegate = prune.NewJobDeleter(o.KClient.Batch())
	} else {
		fmt.Fprintln(os.Stderr, "Dry run enabled - no modifications will be made. Add --confirm to remove jobs")
	}

	return pruner.Prune(jobDeleter)
}

// describingJobDeleter prints information about each job it removes.
// If a delegate exists, its DeleteJob function is invoked prior to returning.
type describingJobDeleter struct {
	w             io.Writer
	delegate      prune.JobDeleter
	headerPrinted bool
}

var _ prune.JobDeleter = &describingJobDeleter{}

func (p *describingJobDeleter) DeleteJob(job *batch.Job) error {
	if !p.headerPrinted {
		p.headerPrinted = true
		fmt.Fprintln(p.w, "NAMESPACE\tNAME")
	}

	fmt.Fprintf(p.w, "%s\t%s\n", job.Namespace, job.Name)

	if p.delegate == nil {
		return nil
	}

	if err := p.delegate.DeleteJob(job); err != nil {
		return err
	}

	return nil
}
