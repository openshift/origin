package annotations

import (
	"reflect"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"

	triggerutil "github.com/openshift/library-go/pkg/image/trigger"
	"github.com/openshift/origin/pkg/image/trigger"
)

// annotationTriggerIndexer uses annotations on objects to trigger changes.
type annotationTriggerIndexer struct {
	prefix string
}

// NewAnnotationTriggerIndexer creates an indexer that deals with objects that have a pod spec and use
// annotations to indicate the desire to trigger.
func NewAnnotationTriggerIndexer(prefix string) trigger.Indexer {
	return annotationTriggerIndexer{prefix: prefix}
}

func (i annotationTriggerIndexer) Index(obj, old interface{}) (string, *trigger.CacheEntry, cache.DeltaType, error) {
	var (
		triggers  []triggerutil.ObjectFieldTrigger
		key       string
		namespace string
		change    cache.DeltaType
	)
	switch {
	case obj != nil && old == nil:
		// added
		m, err := meta.Accessor(obj)
		if err != nil {
			return "", nil, change, err
		}
		key, namespace, triggers, err = triggerutil.CalculateAnnotationTriggers(m, i.prefix)
		if err != nil {
			return "", nil, change, err
		}
		change = cache.Added
	case old != nil && obj == nil:
		// deleted
		m, err := meta.Accessor(old)
		if err != nil {
			return "", nil, change, err
		}
		key, namespace, triggers, err = triggerutil.CalculateAnnotationTriggers(m, i.prefix)
		if err != nil {
			return "", nil, change, err
		}
		change = cache.Deleted
	default:
		// updated
		m, err := meta.Accessor(obj)
		if err != nil {
			return "", nil, change, err
		}
		key, namespace, triggers, err = triggerutil.CalculateAnnotationTriggers(m, i.prefix)
		if err != nil {
			return "", nil, change, err
		}
		oldM, err := meta.Accessor(old)
		if err != nil {
			return "", nil, change, err
		}
		_, _, oldTriggers, err := triggerutil.CalculateAnnotationTriggers(oldM, i.prefix)
		if err != nil {
			return "", nil, change, err
		}
		switch {
		case len(oldTriggers) == 0:
			change = cache.Added
		case !reflect.DeepEqual(oldTriggers, triggers):
			change = cache.Updated
		case triggerutil.ContainerImageChanged(old.(runtime.Object), obj.(runtime.Object), triggers):
			change = cache.Updated
		}
	}

	if len(triggers) > 0 {
		return key, &trigger.CacheEntry{
			Key:       key,
			Namespace: namespace,
			Triggers:  triggers,
		}, change, nil
	}
	return "", nil, change, nil
}
