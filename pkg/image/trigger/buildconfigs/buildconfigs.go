package buildconfigs

import (
	"fmt"
	"reflect"

	clientv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	"github.com/golang/glog"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	triggerapi "github.com/openshift/origin/pkg/image/apis/image/v1/trigger"
	"github.com/openshift/origin/pkg/image/trigger"
)

// calculateBuildConfigTriggers transforms a build config into a set of image change triggers.
// It uses synthetic field paths since we don't need to generically transform the config.
func calculateBuildConfigTriggers(bc *buildapi.BuildConfig) []triggerapi.ObjectFieldTrigger {
	var triggers []triggerapi.ObjectFieldTrigger
	for _, t := range bc.Spec.Triggers {
		if t.ImageChange == nil {
			continue
		}
		var (
			fieldPath string
			from      *kapi.ObjectReference
		)
		if t.ImageChange.From != nil {
			from = t.ImageChange.From
			fieldPath = "spec.triggers"
		} else {
			from = buildapi.GetInputReference(bc.Spec.Strategy)
			fieldPath = "spec.strategy.*.from"
		}
		if from == nil || from.Kind != "ImageStreamTag" || len(from.Name) == 0 {
			continue
		}

		// add a trigger
		triggers = append(triggers, triggerapi.ObjectFieldTrigger{
			From: triggerapi.ObjectReference{
				Name:       from.Name,
				Namespace:  from.Namespace,
				Kind:       from.Kind,
				APIVersion: from.APIVersion,
			},
			FieldPath: fieldPath,
		})
	}
	return triggers
}

// buildConfigTriggerIndexer converts build config events into entries for the trigger cache, and
// also calculates the latest state of the changes on the object.
type buildConfigTriggerIndexer struct {
	prefix string
}

func NewBuildConfigTriggerIndexer(prefix string) trigger.Indexer {
	return buildConfigTriggerIndexer{prefix: prefix}
}

func (i buildConfigTriggerIndexer) Index(obj, old interface{}) (string, *trigger.CacheEntry, cache.DeltaType, error) {
	var (
		triggers []triggerapi.ObjectFieldTrigger
		bc       *buildapi.BuildConfig
		change   cache.DeltaType
	)
	switch {
	case obj != nil && old == nil:
		// added
		bc = obj.(*buildapi.BuildConfig)
		triggers = calculateBuildConfigTriggers(bc)
		change = cache.Added
	case old != nil && obj == nil:
		// deleted
		bc = old.(*buildapi.BuildConfig)
		triggers = calculateBuildConfigTriggers(bc)
		change = cache.Deleted
	default:
		// updated
		bc = obj.(*buildapi.BuildConfig)
		triggers = calculateBuildConfigTriggers(bc)
		oldTriggers := calculateBuildConfigTriggers(old.(*buildapi.BuildConfig))
		switch {
		case len(oldTriggers) == 0:
			change = cache.Added
		case !reflect.DeepEqual(oldTriggers, triggers):
			change = cache.Updated
		}
	}

	if len(triggers) > 0 {
		key := i.prefix + bc.Namespace + "/" + bc.Name
		return key, &trigger.CacheEntry{
			Key:       key,
			Namespace: bc.Namespace,
			Triggers:  triggers,
		}, change, nil
	}
	return "", nil, change, nil
}

// BuildConfigInstantiator abstracts creating builds from build requests.
type BuildConfigInstantiator interface {
	// Instantiate should launch a build from the provided build request.
	Instantiate(namespace string, request *buildapi.BuildRequest) (*buildapi.Build, error)
}

// buildConfigReactor converts trigger changes into new builds. It will request a build if
// at least one image is out of date.
type buildConfigReactor struct {
	instantiator  BuildConfigInstantiator
	eventRecorder record.EventRecorder
}

// NewBuildConfigReactor creates a new buildConfigReactor
func NewBuildConfigReactor(instantiator BuildConfigInstantiator, restclient rest.Interface) trigger.ImageReactor {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(restclient).Events("")})
	eventRecorder := eventBroadcaster.NewRecorder(legacyscheme.Scheme, clientv1.EventSource{Component: "buildconfig-controller"})

	return &buildConfigReactor{instantiator: instantiator, eventRecorder: eventRecorder}
}

// ImageChanged is passed a build config and a set of changes and updates the object if
// necessary.
func (r *buildConfigReactor) ImageChanged(obj runtime.Object, tagRetriever trigger.TagRetriever) error {
	bc := obj.(*buildapi.BuildConfig)

	var request *buildapi.BuildRequest
	var fired map[kapi.ObjectReference]string
	for _, t := range bc.Spec.Triggers {
		p := t.ImageChange
		if p == nil || (p.From != nil && p.From.Kind != "ImageStreamTag") {
			continue
		}
		var from *kapi.ObjectReference
		if p.From != nil {
			from = p.From
		} else {
			from = buildapi.GetInputReference(bc.Spec.Strategy)
		}
		namespace := from.Namespace
		if len(namespace) == 0 {
			namespace = bc.Namespace
		}

		// lookup the source if we haven't already retrieved it
		var newSource bool
		latest, found := fired[*from]
		if !found {
			latest, _, found = tagRetriever.ImageStreamTag(namespace, from.Name)
			if !found {
				continue
			}
			newSource = true
		}

		// LastTriggeredImageID is an image ref, despite the name
		if latest == p.LastTriggeredImageID {
			continue
		}

		// prevent duplicate build trigger causes
		if fired == nil {
			fired = make(map[kapi.ObjectReference]string)
		}
		fired[*from] = latest

		if request == nil {
			request = &buildapi.BuildRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bc.Name,
					Namespace: bc.Namespace,
				},
			}
		}
		if request.TriggeredByImage == nil {
			request.TriggeredByImage = &kapi.ObjectReference{
				Kind: "DockerImage",
				Name: latest,
			}
		}
		if request.From == nil {
			request.From = from
		}

		if newSource {
			request.TriggeredBy = append(request.TriggeredBy, buildapi.BuildTriggerCause{
				Message: buildapi.BuildTriggerCauseImageMsg,
				ImageChangeBuild: &buildapi.ImageChangeCause{
					ImageID: latest,
					FromRef: from,
				},
			})
		}
	}

	if request == nil {
		return nil
	}

	// instantiate new build
	glog.V(4).Infof("Requesting build for BuildConfig based on image triggers %s/%s: %#v", bc.Namespace, bc.Name, request)
	_, err := r.instantiator.Instantiate(bc.Namespace, request)
	if err != nil {
		instantiateErr := fmt.Errorf("error triggering Build for BuildConfig %s/%s: %v", bc.Namespace, bc.Name, err)
		utilruntime.HandleError(instantiateErr)
		r.eventRecorder.Event(bc, kapi.EventTypeWarning, "BuildConfigTriggerFailed", instantiateErr.Error())
	}
	return err
}
