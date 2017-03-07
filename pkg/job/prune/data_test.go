package prune

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/util/sets"
)

func mockScheduledJob(namespace, name string) *batch.ScheduledJob {
	return &batch.ScheduledJob{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			SelfLink:  fmt.Sprintf("/api/batch/v2alpha1/namespaces/%s/scheduledjobs/%s", namespace, name),
		},
	}
}

func mockJob(namespace, name string, scheduledJob *batch.ScheduledJob) *batch.Job {
	job := &batch.Job{ObjectMeta: kapi.ObjectMeta{Namespace: namespace, Name: name}}
	job.Annotations = make(map[string]string)
	if scheduledJob != nil {

		createdByRefJson, _ := makeCreatedByRefJson(scheduledJob)
		job.Annotations[kapi.CreatedByAnnotation] = createdByRefJson
	}
	return job
}

func withCreated(item *batch.Job, creationTimestamp unversioned.Time) *batch.Job {
	item.CreationTimestamp = creationTimestamp
	return item
}

func withCondition(item *batch.Job, ctype batch.JobConditionType) *batch.Job {
	item.Status.Conditions = append(item.Status.Conditions, batch.JobCondition{
		Type:               ctype,
		Status:             kapi.ConditionTrue,
		LastProbeTime:      unversioned.Now(),
		LastTransitionTime: unversioned.Now(),
	})
	return item
}

func TestJobByScheduledJobIndexFunc(t *testing.T) {
	scheduledJob := mockScheduledJob("a", "b")
	job := mockJob("a", "c", scheduledJob)

	actualKey, err := JobByScheduledJobIndexFunc(job)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	expectedKey := []string{"a/b"}
	if !reflect.DeepEqual(actualKey, expectedKey) {
		t.Errorf("expected %v, actual %v", expectedKey, actualKey)
	}
	singleJob := &batch.Job{}
	actualKey, err = JobByScheduledJobIndexFunc(singleJob)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	expectedKey = []string{"orphan"}
	if !reflect.DeepEqual(actualKey, expectedKey) {
		t.Errorf("expected %v, actual %v", expectedKey, actualKey)
	}
}

func TestFilterBeforePredicate(t *testing.T) {
	youngerThan := time.Hour
	now := unversioned.Now()
	old := unversioned.NewTime(now.Time.Add(-1 * youngerThan))
	items := []*batch.Job{}
	items = append(items, withCreated(mockJob("a", "old", nil), old))
	items = append(items, withCreated(mockJob("a", "new", nil), now))
	filter := &andFilter{
		filterPredicates: []FilterPredicate{NewFilterBeforePredicate(youngerThan)},
	}
	result := filter.Filter(items)
	if len(result) != 1 {
		t.Errorf("Unexpected number of results")
	}
	if expected, actual := "old", result[0].Name; expected != actual {
		t.Errorf("expected %v, actual %v", expected, actual)
	}
}

func TestEmptyDataSet(t *testing.T) {
	scheduledJobs := []*batch.ScheduledJob{}
	jobs := []*batch.Job{}
	dataSet := NewDataSet(scheduledJobs, jobs)
	_, exists, err := dataSet.GetScheduledJob(&batch.Job{})
	if exists || err != nil {
		t.Errorf("Unexpected result %v, %v", exists, err)
	}
	scheduledJobResults, err := dataSet.ListScheduledJobs()
	if err != nil {
		t.Errorf("Unexpected result %v", err)
	}
	if len(scheduledJobResults) != 0 {
		t.Errorf("Unexpected result %v", scheduledJobResults)
	}
	jobResults, err := dataSet.ListJobs()
	if err != nil {
		t.Errorf("Unexpected result %v", err)
	}
	if len(jobResults) != 0 {
		t.Errorf("Unexpected result %v", jobResults)
	}
	jobResults, err = dataSet.ListJobsByScheduledJob(mockScheduledJob("a", "b"))
	if err != nil {
		t.Errorf("Unexpected result %v", err)
	}
	if len(jobResults) != 0 {
		t.Errorf("Unexpected result %v", jobResults)
	}
}

func TestPopulatedDataSet(t *testing.T) {
	scheduledJobs := []*batch.ScheduledJob{
		mockScheduledJob("a", "scheduled-job-1"),
		mockScheduledJob("b", "scheduled-job-2"),
	}
	jobs := []*batch.Job{
		mockJob("a", "job-1", scheduledJobs[0]),
		mockJob("a", "job-2", scheduledJobs[0]),
		mockJob("b", "job-3", scheduledJobs[1]),
		mockJob("c", "job-4", nil),
	}
	dataSet := NewDataSet(scheduledJobs, jobs)
	for _, job := range jobs {
		_, exists, err := dataSet.GetScheduledJob(job)
		if _, ok := job.Annotations[kapi.CreatedByAnnotation]; ok {
			if err != nil {
				t.Errorf("Item %v, unexpected error: %v", job, err)
			}
			if !exists {
				t.Errorf("Item %v, unexpected result: %v", job, exists)
			}
		} else {
			if err != nil {
				t.Errorf("Item %v, unexpected error: %v", job, err)
			}
			if exists {
				t.Errorf("Item %v, unexpected result: %v", job, exists)
			}
		}
	}
	expectedNames := sets.NewString("job-1", "job-2")
	jobResults, err := dataSet.ListJobsByScheduledJob(scheduledJobs[0])
	if err != nil {
		t.Errorf("Unexpected result %v", err)
	}
	if len(jobResults) != len(expectedNames) {
		t.Errorf("Unexpected result %v", jobResults)
	}
	for _, job := range jobResults {
		if !expectedNames.Has(job.Name) {
			t.Errorf("Unexpected name: %v", job.Name)
		}
	}
}
