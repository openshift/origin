package monitortestlibrary

import (
	"strconv"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/test/e2e/framework"
)

type ObjCreateFunc func(obj interface{}) []monitorapi.Interval
type ObjUpdateFunc func(obj, oldObj interface{}) []monitorapi.Interval
type ObjDeleteFunc func(obj interface{}) []monitorapi.Interval

type ResourceTracker interface {
	// RecordResource stores a resource for later serialization.  Deletion is not tracked, so this can be used
	// to determine the final state of resource that are deleted in a namespace.
	// Annotations are added to indicate number of updates and the number of recreates.
	RecordResource(resourceType string, obj runtime.Object)
}

type IntervalRecorder interface {
	AddIntervals(intervals ...monitorapi.Interval)
}

type monitoringStore struct {
	*cache.FakeCustomStore

	// map event UIDs to the last resource version we observed, used to skip recording resources
	// we've already recorded.
	processedResourceUIDs map[types.UID]int
	cacheOfNow            map[types.UID]interface{}
}

func NewMonitoringStore(
	resourceType string,
	createHandlers []ObjCreateFunc,
	updateHandlers []ObjUpdateFunc,
	deleteHandlers []ObjDeleteFunc,
	resourceTracker ResourceTracker,
	intervalRecorder IntervalRecorder,
) *monitoringStore {
	s := &monitoringStore{
		FakeCustomStore:       &cache.FakeCustomStore{},
		processedResourceUIDs: map[types.UID]int{},
		cacheOfNow:            map[types.UID]interface{}{},
	}

	s.UpdateFunc = func(obj interface{}) error {
		currentUID := uidOf(obj)
		currentResourceVersion := resourceVersionAsInt(obj)
		if s.processedResourceUIDs[currentUID] >= currentResourceVersion {
			return nil
		}

		defer func() {
			s.processedResourceUIDs[currentUID] = currentResourceVersion
			s.cacheOfNow[currentUID] = obj
		}()

		resourceTracker.RecordResource(resourceType, obj.(runtime.Object))
		oldObj, ok := s.cacheOfNow[currentUID]
		if !ok {
			framework.Logf("#### missing object on update for %v\n", currentUID)
			return nil
		}

		for _, updateHandler := range updateHandlers {
			intervalRecorder.AddIntervals(updateHandler(obj, oldObj)...)
		}

		return nil
	}

	s.AddFunc = func(obj interface{}) error {
		currentUID := uidOf(obj)
		currentResourceVersion := resourceVersionAsInt(obj)
		if s.processedResourceUIDs[currentUID] >= currentResourceVersion {
			return nil
		}

		defer func() {
			s.processedResourceUIDs[currentUID] = currentResourceVersion
			s.cacheOfNow[currentUID] = obj
		}()

		resourceTracker.RecordResource(resourceType, obj.(runtime.Object))

		for _, createHandler := range createHandlers {
			intervalRecorder.AddIntervals(createHandler(obj)...)
		}

		return nil
	}

	s.DeleteFunc = func(obj interface{}) error {
		currentUID := uidOf(obj)
		currentResourceVersion := resourceVersionAsInt(obj)
		if s.processedResourceUIDs[currentUID] >= currentResourceVersion {
			return nil
		}

		// clear values that have been deleted
		defer func() {
			delete(s.processedResourceUIDs, currentUID)
			delete(s.cacheOfNow, currentUID)
		}()

		resourceTracker.RecordResource(resourceType, obj.(runtime.Object))

		for _, deleteHandler := range deleteHandlers {
			intervalRecorder.AddIntervals(deleteHandler(obj)...)
		}

		return nil
	}

	// ReplaceFunc called when we do our initial list on starting the reflector.
	// This can do adds, updates, and deletes.
	s.ReplaceFunc = func(items []interface{}, rv string) error {
		newUids := map[types.UID]bool{}
		for _, item := range items {
			newUids[uidOf(item)] = true
		}
		deletedUIDs := map[types.UID]bool{}
		for uid := range s.cacheOfNow {
			if !newUids[uid] {
				deletedUIDs[uid] = true
			}
		}

		for _, obj := range items {
			currentUID := uidOf(obj)

			_, oldObjExists := s.cacheOfNow[currentUID]
			switch {
			case oldObjExists:
				s.UpdateFunc(obj)
			case deletedUIDs[currentUID]:
				s.DeleteFunc(obj)
			default:
				s.AddFunc(obj)
			}
		}
		return nil
	}

	return s
}

func resourceVersionAsInt(obj interface{}) int {
	metadata, err := meta.Accessor(obj)
	if err != nil {
		panic(err)
	}

	asInt, err := strconv.ParseInt(metadata.GetResourceVersion(), 10, 64)
	if err != nil {
		panic(err)
	}

	return int(asInt)
}

func uidOf(obj interface{}) types.UID {
	metadata, err := meta.Accessor(obj)
	if err != nil {
		panic(err)
	}
	return metadata.GetUID()
}
