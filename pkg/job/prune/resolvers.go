package prune

import (
	"sort"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/batch"
)

// Resolver knows how to resolve the set of candidate objects to prune
type Resolver interface {
	Resolve() ([]*batch.Job, error)
}

// mergeResolver merges the set of results from multiple resolvers
type mergeResolver struct {
	resolvers []Resolver
}

func (m *mergeResolver) Resolve() ([]*batch.Job, error) {
	results := []*batch.Job{}
	for _, resolver := range m.resolvers {
		items, err := resolver.Resolve()
		if err != nil {
			return nil, err
		}
		results = append(results, items...)
	}
	return results, nil
}

type perScheduledJobResolver struct {
	dataSet      DataSet
	keepComplete int
	keepFailed   int
}

// NewPerScheduledJobResolver returns a Resolver that selects items to prune per ScheduledJob
func NewPerScheduledJobResolver(dataSet DataSet, keepComplete int, keepFailed int) Resolver {
	return &perScheduledJobResolver{
		dataSet:      dataSet,
		keepComplete: keepComplete,
		keepFailed:   keepFailed,
	}
}

func (o *perScheduledJobResolver) Resolve() ([]*batch.Job, error) {
	scheduledJobs, err := o.dataSet.ListScheduledJobs()
	if err != nil {
		return nil, err
	}

	prunableJobs := []*batch.Job{}
	for _, sj := range scheduledJobs {
		jobs, er := o.dataSet.ListJobsByScheduledJob(sj)
		if er != nil {
			return nil, er
		}

		completeJobs, failedJobs := []*batch.Job{}, []*batch.Job{}
		for _, job := range jobs {
			if getStatus(job, batch.JobComplete) {
				completeJobs = append(completeJobs, job)
			} else if getStatus(job, batch.JobFailed) {
				failedJobs = append(failedJobs, job)
			}
		}
		sort.Sort(sort.Reverse(jobAge(completeJobs)))
		sort.Sort(sort.Reverse(jobAge(failedJobs)))

		if o.keepComplete >= 0 && o.keepComplete < len(completeJobs) {
			prunableJobs = append(prunableJobs, completeJobs[o.keepComplete:]...)
		}
		if o.keepFailed >= 0 && o.keepFailed < len(failedJobs) {
			prunableJobs = append(prunableJobs, failedJobs[o.keepFailed:]...)
		}
	}
	return prunableJobs, nil
}

func getStatus(j *batch.Job, ct batch.JobConditionType) bool {
	for _, c := range j.Status.Conditions {
		if c.Type == ct && c.Status == kapi.ConditionTrue {
			return true
		}
	}
	return false
}

// jobAge sorts jobs by most recently created.
type jobAge []*batch.Job

func (s jobAge) Len() int      { return len(s) }
func (s jobAge) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s jobAge) Less(i, j int) bool {
	return !s[i].CreationTimestamp.Before(s[j].CreationTimestamp)
}
