package prune

import (
	"sort"
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/util/sets"
)

type mockDeleteRecorder struct {
	set sets.String
	err error
}

var _ JobDeleter = &mockDeleteRecorder{}

func (m *mockDeleteRecorder) DeleteJob(job *batch.Job) error {
	m.set.Insert(job.Name)
	return m.err
}

func (m *mockDeleteRecorder) Verify(t *testing.T, expected sets.String) {
	if len(m.set) != len(expected) || !m.set.HasAll(expected.List()...) {
		expectedValues := expected.List()
		actualValues := m.set.List()
		sort.Strings(expectedValues)
		sort.Strings(actualValues)
		t.Errorf("expected \n\t%v\n, actual \n\t%v\n", expectedValues, actualValues)
	}
}

func TestPruneTask(t *testing.T) {
	jobConditions := []batch.JobConditionType{
		batch.JobComplete,
		batch.JobFailed,
		// this one "mimics" running, because no such condition exists for a job
		batch.JobConditionType(""),
	}

	for _, jobCondition := range jobConditions {
		keepYoungerThan := time.Hour

		now := unversioned.Now()
		old := unversioned.NewTime(now.Time.Add(-1 * keepYoungerThan))

		scheduledJobs := []*batch.ScheduledJob{}
		jobs := []*batch.Job{}

		scheduledJob := mockScheduledJob("a", "scheduled-job")
		scheduledJobs = append(scheduledJobs, scheduledJob)

		jobs = append(jobs, withCreated(withCondition(mockJob("a", "sj-job-1", scheduledJob), jobCondition), now))
		jobs = append(jobs, withCreated(withCondition(mockJob("a", "sj-job-2", scheduledJob), jobCondition), old))
		jobs = append(jobs, withCreated(withCondition(mockJob("a", "standalone-job-1", nil), jobCondition), now))
		jobs = append(jobs, withCreated(withCondition(mockJob("a", "standalone-job-2", nil), jobCondition), old))

		keepComplete := 1
		keepFailed := 1
		expectedValues := sets.String{}
		filter := &andFilter{
			filterPredicates: []FilterPredicate{NewFilterBeforePredicate(keepYoungerThan)},
		}
		dataSet := NewDataSet(scheduledJobs, filter.Filter(jobs))
		resolver := NewPerScheduledJobResolver(dataSet, keepComplete, keepFailed)
		expectedJobs, err := resolver.Resolve()
		if err != nil {
			t.Errorf("Unexpected error %v", err)
		}
		for _, item := range expectedJobs {
			expectedValues.Insert(item.Name)
		}

		recorder := &mockDeleteRecorder{set: sets.String{}}

		options := PrunerOptions{
			KeepYoungerThan: keepYoungerThan,
			KeepComplete:    keepComplete,
			KeepFailed:      keepFailed,
			ScheduledJobs:   scheduledJobs,
			Jobs:            jobs,
		}
		pruner := NewPruner(options)
		if er := pruner.Prune(recorder); er != nil {
			t.Errorf("Unexpected error %v", er)
		}
		recorder.Verify(t, expectedValues)
	}
}
