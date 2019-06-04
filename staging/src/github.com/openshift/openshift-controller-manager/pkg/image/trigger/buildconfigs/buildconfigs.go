package buildconfigs

import (
	"fmt"
	"reflect"

	"k8s.io/klog"

	clientv1 "k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	buildv1 "github.com/openshift/api/build/v1"
	buildclientv1 "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"
	triggerutil "github.com/openshift/library-go/pkg/image/trigger"
	"github.com/openshift/openshift-controller-manager/pkg/image/trigger"
)

const BuildTriggerCauseImageMsg = "Image change"

// GetInputReference returns the From ObjectReference associated with the
// BuildStrategy.
func GetInputReference(strategy buildv1.BuildStrategy) *corev1.ObjectReference {
	switch {
	case strategy.SourceStrategy != nil:
		return &strategy.SourceStrategy.From
	case strategy.DockerStrategy != nil:
		return strategy.DockerStrategy.From
	case strategy.CustomStrategy != nil:
		return &strategy.CustomStrategy.From
	default:
		return nil
	}
}

// calculateBuildConfigTriggers transforms a build config into a set of image change triggers.
// It uses synthetic field paths since we don't need to generically transform the config.
func calculateBuildConfigTriggers(bc *buildv1.BuildConfig) []triggerutil.ObjectFieldTrigger {
	var triggers []triggerutil.ObjectFieldTrigger
	for _, t := range bc.Spec.Triggers {
		if t.ImageChange == nil {
			continue
		}
		var (
			fieldPath string
			from      *corev1.ObjectReference
		)
		if t.ImageChange.From != nil {
			from = t.ImageChange.From
			fieldPath = "spec.triggers"
		} else {
			from = GetInputReference(bc.Spec.Strategy)
			fieldPath = "spec.strategy.*.from"
		}
		if from == nil || from.Kind != "ImageStreamTag" || len(from.Name) == 0 {
			continue
		}

		// add a trigger
		triggers = append(triggers, triggerutil.ObjectFieldTrigger{
			From: triggerutil.ObjectReference{
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
		triggers []triggerutil.ObjectFieldTrigger
		bc       *buildv1.BuildConfig
		change   cache.DeltaType
	)
	switch {
	case obj != nil && old == nil:
		// added
		bc = obj.(*buildv1.BuildConfig)
		triggers = calculateBuildConfigTriggers(bc)
		change = cache.Added
	case old != nil && obj == nil:
		// deleted
		bc = old.(*buildv1.BuildConfig)
		triggers = calculateBuildConfigTriggers(bc)
		change = cache.Deleted
	default:
		// updated
		bc = obj.(*buildv1.BuildConfig)
		triggers = calculateBuildConfigTriggers(bc)
		oldTriggers := calculateBuildConfigTriggers(old.(*buildv1.BuildConfig))
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
	Instantiate(namespace string, request *buildv1.BuildRequest) (*buildv1.Build, error)
}

// buildConfigReactor converts trigger changes into new builds. It will request a build if
// at least one image is out of date.
type buildConfigReactor struct {
	instantiator  buildclientv1.BuildConfigsGetter
	eventRecorder record.EventRecorder
}

// NewBuildConfigReactor creates a new buildConfigReactor
func NewBuildConfigReactor(instantiator buildclientv1.BuildConfigsGetter, restclient rest.Interface) trigger.ImageReactor {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(restclient).Events("")})
	eventRecorder := eventBroadcaster.NewRecorder(legacyscheme.Scheme, clientv1.EventSource{Component: "buildconfig-controller"})

	return &buildConfigReactor{instantiator: instantiator, eventRecorder: eventRecorder}
}

// ImageChanged is passed a build config and a set of changes and updates the object if
// necessary.
func (r *buildConfigReactor) ImageChanged(obj runtime.Object, tagRetriever triggerutil.TagRetriever) error {
	bc := obj.(*buildv1.BuildConfig)

	var request *buildv1.BuildRequest
	var fired map[corev1.ObjectReference]string
	for _, t := range bc.Spec.Triggers {
		p := t.ImageChange
		if p == nil || (p.From != nil && p.From.Kind != "ImageStreamTag") {
			continue
		}
		if p.Paused {
			klog.V(5).Infof("Skipping paused build on bc: %s/%s for trigger: %+v", bc.Namespace, bc.Name, t)
			continue
		}
		var from *corev1.ObjectReference
		if p.From != nil {
			from = p.From
		} else {
			from = GetInputReference(bc.Spec.Strategy)
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
			fired = make(map[corev1.ObjectReference]string)
		}
		fired[*from] = latest

		if request == nil {
			request = &buildv1.BuildRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bc.Name,
					Namespace: bc.Namespace,
				},
			}
		}
		if request.TriggeredByImage == nil {
			request.TriggeredByImage = &corev1.ObjectReference{
				Kind: "DockerImage",
				Name: latest,
			}
		}
		if request.From == nil {
			request.From = from
		}

		if newSource {
			request.TriggeredBy = append(request.TriggeredBy, buildv1.BuildTriggerCause{
				Message: BuildTriggerCauseImageMsg,
				ImageChangeBuild: &buildv1.ImageChangeCause{
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
	klog.V(4).Infof("Requesting build for BuildConfig based on image triggers %s/%s: %#v", bc.Namespace, bc.Name, request)
	_, err := r.instantiator.BuildConfigs(bc.Namespace).Instantiate(bc.Namespace, request)
	if err != nil {
		instantiateErr := fmt.Errorf("error triggering Build for BuildConfig %s/%s: %v", bc.Namespace, bc.Name, err)
		utilruntime.HandleError(instantiateErr)
		r.eventRecorder.Event(bc, corev1.EventTypeWarning, "BuildConfigTriggerFailed", instantiateErr.Error())
	}
	return err
}
