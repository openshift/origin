package prune

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/runtime"
)

func JobByScheduledJobIndexFunc(obj interface{}) ([]string, error) {
	job, ok := obj.(*batch.Job)
	if !ok {
		return nil, fmt.Errorf("not a job: %v", job)
	}
	namespace, name, found := getParentFromJob(job)
	if !found {
		return []string{"orphan"}, nil
	}
	return []string{namespace + "/" + name}, nil
}

// getParentFromJob extracts namespace and name of job's parent and whether it was found
func getParentFromJob(job *batch.Job) (string, string, bool) {
	creatorRefJson, found := job.ObjectMeta.Annotations[kapi.CreatedByAnnotation]
	if !found {
		glog.V(4).Infof("Job with no created-by annotation, name %s namespace %s", job.Name, job.Namespace)
		return "", "", false
	}
	var sr kapi.SerializedReference
	err := json.Unmarshal([]byte(creatorRefJson), &sr)
	if err != nil {
		glog.V(4).Infof("Job with unparsable created-by annotation, name %s namespace %s: %v", job.Name, job.Namespace, err)
		return "", "", false
	}
	if sr.Reference.Kind != "ScheduledJob" {
		glog.V(4).Infof("Job with non-ScheduledJob parent, name %s namespace %s", job.Name, job.Namespace)
		return "", "", false
	}
	// Don't believe a job that claims to have a parent in a different namespace.
	if sr.Reference.Namespace != job.Namespace {
		glog.V(4).Infof("Alleged scheduledJob parent in different namespace (%s) from Job name %s namespace %s", sr.Reference.Namespace, job.Name, job.Namespace)
		return "", "", false
	}

	return sr.Reference.Namespace, sr.Reference.Name, true
}

// makeCreatedByRefJson makes a json string with an object reference for use in "created-by" annotation value
func makeCreatedByRefJson(object runtime.Object) (string, error) {
	createdByRef, err := kapi.GetReference(object)
	if err != nil {
		return fmt.Sprintf("unable to get controller reference: %v", err), fmt.Errorf("unable to get controller reference: %v", err)
	}

	codec := kapi.Codecs.LegacyCodec(unversioned.GroupVersion{Group: kapi.GroupName, Version: "v1"})

	createdByRefJson, err := runtime.Encode(codec, &kapi.SerializedReference{
		Reference: *createdByRef,
	})
	if err != nil {
		return fmt.Sprintf("unable to serialize controller reference: %v", err), fmt.Errorf("unable to serialize controller reference: %v", err)
	}
	return string(createdByRefJson), nil
}

// Filter filters the set of objects
type Filter interface {
	Filter(items []*batch.Job) []*batch.Job
}

// andFilter ands a set of predicate functions to know if it should be included in the return set
type andFilter struct {
	filterPredicates []FilterPredicate
}

// Filter ands the set of predicates evaluated against each item to make a filtered set
func (a *andFilter) Filter(items []*batch.Job) []*batch.Job {
	results := []*batch.Job{}
	for _, item := range items {
		include := true
		for _, filterPredicate := range a.filterPredicates {
			include = include && filterPredicate(item)
		}
		if include {
			results = append(results, item)
		}
	}
	return results
}

// FilterPredicate is a function that returns true if the object should be included in the filtered set
type FilterPredicate func(item *batch.Job) bool

// NewFilterBeforePredicate is a function that returns true if the job was created before the current time minus specified duration
func NewFilterBeforePredicate(d time.Duration) FilterPredicate {
	now := unversioned.Now()
	before := unversioned.NewTime(now.Time.Add(-1 * d))
	return func(item *batch.Job) bool {
		return item.CreationTimestamp.Before(before)
	}
}

// DataSet provides functions for working with deployment data
type DataSet interface {
	GetScheduledJob(job *batch.Job) (*batch.ScheduledJob, bool, error)
	ListScheduledJobs() ([]*batch.ScheduledJob, error)
	ListJobs() ([]*batch.Job, error)
	ListJobsByScheduledJob(scheduledJob *batch.ScheduledJob) ([]*batch.Job, error)
}

type dataSet struct {
	scheduledJobStore cache.Store
	jobIndexer        cache.Indexer
}

// NewDataSet returns a DataSet over the specified items
func NewDataSet(scheduledJobs []*batch.ScheduledJob, jobs []*batch.Job) DataSet {
	scheduledJobStore := cache.NewStore(cache.MetaNamespaceKeyFunc)
	for _, sj := range scheduledJobs {
		scheduledJobStore.Add(sj)
	}

	jobIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{
		"scheduledJob": JobByScheduledJobIndexFunc,
	})
	for _, job := range jobs {
		jobIndexer.Add(job)
	}

	return &dataSet{
		scheduledJobStore: scheduledJobStore,
		jobIndexer:        jobIndexer,
	}
}

// GetScheduledJob gets the ScheduledJob for the given Job
func (d *dataSet) GetScheduledJob(job *batch.Job) (*batch.ScheduledJob, bool, error) {
	namespace, name, found := getParentFromJob(job)
	if !found {
		return nil, false, nil
	}

	var scheduledJob *batch.ScheduledJob
	key := &batch.ScheduledJob{ObjectMeta: kapi.ObjectMeta{Name: name, Namespace: namespace}}
	item, exists, err := d.scheduledJobStore.Get(key)
	if exists {
		scheduledJob = item.(*batch.ScheduledJob)
	}
	return scheduledJob, exists, err
}

// ListScheduledJobs returns a list of ScheduledJobs
func (d *dataSet) ListScheduledJobs() ([]*batch.ScheduledJob, error) {
	results := []*batch.ScheduledJob{}
	for _, item := range d.scheduledJobStore.List() {
		results = append(results, item.(*batch.ScheduledJob))
	}
	return results, nil
}

// ListJobs returns a list of Jobs
func (d *dataSet) ListJobs() ([]*batch.Job, error) {
	results := []*batch.Job{}
	for _, item := range d.jobIndexer.List() {
		results = append(results, item.(*batch.Job))
	}
	return results, nil
}

// ListJobsByScheduledJob returns a list of Jobs for the provided ScheduledJob
func (d *dataSet) ListJobsByScheduledJob(scheduledJob *batch.ScheduledJob) ([]*batch.Job, error) {
	results := []*batch.Job{}
	createdByRefJson, err := makeCreatedByRefJson(scheduledJob)
	if err != nil {
		return results, err
	}
	key := &batch.Job{
		ObjectMeta: kapi.ObjectMeta{
			Namespace:   scheduledJob.Namespace,
			Annotations: map[string]string{kapi.CreatedByAnnotation: createdByRefJson},
		},
	}
	items, err := d.jobIndexer.Index("scheduledJob", key)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		results = append(results, item.(*batch.Job))
	}
	return results, nil
}
