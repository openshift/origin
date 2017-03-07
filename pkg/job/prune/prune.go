package prune

import (
	"time"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/apis/batch"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
)

type Pruner interface {
	// Prune is responsible for actual removal of jobs identified as candidates
	// for pruning based on pruning algorithm.
	Prune(deleter JobDeleter) error
}

// JobDeleter knows how to delete jobs from OpenShift.
type JobDeleter interface {
	// DeleteJob removes the job from OpenShift's storage.
	DeleteJob(job *batch.Job) error
}

// pruner is an object that knows how to prune a data set
type pruner struct {
	resolver Resolver
}

var _ Pruner = &pruner{}

// PrunerOptions contains the fields used to initialize a new Pruner.
type PrunerOptions struct {
	// KeepYoungerThan will filter out all objects from prune data set that are younger than the specified time duration.
	KeepYoungerThan time.Duration
	// KeepComplete specifies how many of the most recent completed jobs should be preserved.
	KeepComplete int
	// KeepFailed is specifies how many of the most recent failed jobs should be preserved.
	KeepFailed int
	// ScheduledJobs is the entire list of scheduled jobs across all namespaces in the cluster.
	ScheduledJobs []*batch.ScheduledJob
	// Jobs is the entire list of jobs across all namespaces in the cluster.
	Jobs []*batch.Job
}

// NewPruner returns a Pruner over specified data using specified options.
func NewPruner(options PrunerOptions) Pruner {
	filter := &andFilter{
		filterPredicates: []FilterPredicate{NewFilterBeforePredicate(options.KeepYoungerThan)},
	}
	jobs := filter.Filter(options.Jobs)
	dataSet := NewDataSet(options.ScheduledJobs, jobs)

	resolvers := []Resolver{}
	resolvers = append(resolvers, NewPerScheduledJobResolver(dataSet, options.KeepComplete, options.KeepFailed))

	return &pruner{
		resolver: &mergeResolver{resolvers: resolvers},
	}
}

// Prune will visit each item in the prunable set and invoke the associated JobDeleter.
func (p *pruner) Prune(deleter JobDeleter) error {
	jobs, err := p.resolver.Resolve()
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if er := deleter.DeleteJob(job); er != nil {
			return er
		}
	}
	return nil
}

// jobDeleter removes a job from OpenShift.
type jobDeleter struct {
	jobs kclient.JobsNamespacer
}

var _ JobDeleter = &jobDeleter{}

// NewJobDeleter creates a new jobDeleter.
func NewJobDeleter(jobs kclient.JobsNamespacer) JobDeleter {
	return &jobDeleter{
		jobs: jobs,
	}
}

func (p *jobDeleter) DeleteJob(job *batch.Job) error {
	glog.V(4).Infof("Deleting job %q", job.Name)
	return p.jobs.Jobs(job.Namespace).Delete(job.Name, nil)
}
