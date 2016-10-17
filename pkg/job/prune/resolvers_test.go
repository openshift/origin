package prune

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/util/sets"
)

type mockResolver struct {
	items []*batch.Job
	err   error
}

func (m *mockResolver) Resolve() ([]*batch.Job, error) {
	return m.items, m.err
}

func TestMergeResolver(t *testing.T) {
	resolverA := &mockResolver{
		items: []*batch.Job{
			mockJob("a", "b", nil),
		},
	}
	resolverB := &mockResolver{
		items: []*batch.Job{
			mockJob("c", "d", nil),
		},
	}
	resolver := &mergeResolver{resolvers: []Resolver{resolverA, resolverB}}
	results, err := resolver.Resolve()
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Unexpected results %v", results)
	}
	expectedNames := sets.NewString("b", "d")
	for _, item := range results {
		if !expectedNames.Has(item.Name) {
			t.Errorf("Unexpected name %v", item.Name)
		}
	}
}

func TestPerScheduledJobResolver(t *testing.T) {
	jobConditions := []batch.JobConditionType{
		batch.JobComplete,
		batch.JobFailed,
		// this one "mimics" running, because no such condition exists for a job
		batch.JobConditionType(""),
	}
	scheduledJobs := []*batch.ScheduledJob{
		mockScheduledJob("a", "scheduled-job-1"),
		mockScheduledJob("b", "scheduled-job-2"),
	}
	jobsPerCondition := 100
	jobs := []*batch.Job{}
	for _, sj := range scheduledJobs {
		for _, jc := range jobConditions {
			for i := 0; i < jobsPerCondition; i++ {
				jobs = append(jobs, withCondition(mockJob(sj.Namespace, fmt.Sprintf("job-%v-%v", jc, i), sj), jc))
			}
		}
	}

	now := unversioned.Now()
	for i := range jobs {
		creationTimestamp := unversioned.NewTime(now.Time.Add(-1 * time.Duration(i) * time.Hour))
		jobs[i].CreationTimestamp = creationTimestamp
	}

	// test number to keep at varying ranges
	for keep := 0; keep < jobsPerCondition*2; keep++ {
		dataSet := NewDataSet(scheduledJobs, jobs)

		expectedNames := sets.String{}

		for _, sj := range scheduledJobs {
			jobItems, err := dataSet.ListJobsByScheduledJob(sj)
			if err != nil {
				t.Errorf("Unexpected err %v", err)
			}
			completeJobs, failedJobs := []*batch.Job{}, []*batch.Job{}
			for _, job := range jobItems {
				if getStatus(job, batch.JobComplete) {
					completeJobs = append(completeJobs, job)
				} else if getStatus(job, batch.JobFailed) {
					failedJobs = append(failedJobs, job)
				}
			}
			sort.Sort(sort.Reverse(jobAge(completeJobs)))
			sort.Sort(sort.Reverse(jobAge(failedJobs)))
			purgeCompleted := []*batch.Job{}
			purgeFailed := []*batch.Job{}
			if keep >= 0 && keep < len(completeJobs) {
				purgeCompleted = completeJobs[keep:]
			}
			if keep >= 0 && keep < len(failedJobs) {
				purgeFailed = failedJobs[keep:]
			}
			for _, job := range purgeCompleted {
				expectedNames.Insert(job.Name)
			}
			for _, job := range purgeFailed {
				expectedNames.Insert(job.Name)
			}
		}

		resolver := NewPerScheduledJobResolver(dataSet, keep, keep)
		results, err := resolver.Resolve()
		if err != nil {
			t.Errorf("Unexpected error %v", err)
		}
		foundNames := sets.String{}
		for _, result := range results {
			foundNames.Insert(result.Name)
		}
		if len(foundNames) != len(expectedNames) || !expectedNames.HasAll(foundNames.List()...) {
			expectedValues := expectedNames.List()
			actualValues := foundNames.List()
			sort.Strings(expectedValues)
			sort.Strings(actualValues)
			t.Errorf("keep %v\n, expected \n\t%v\n, actual \n\t%v\n", keep, expectedValues, actualValues)
		}
	}
}
