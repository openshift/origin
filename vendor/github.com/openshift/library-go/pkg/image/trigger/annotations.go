package trigger

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"k8s.io/klog"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/library-go/pkg/image/referencemutator"
)

func CalculateAnnotationTriggers(m metav1.Object, prefix string) (string, string, []ObjectFieldTrigger, error) {
	var key, namespace string
	if namespace = m.GetNamespace(); len(namespace) > 0 {
		key = prefix + namespace + "/" + m.GetName()
	} else {
		key = prefix + m.GetName()
	}
	t, ok := m.GetAnnotations()[TriggerAnnotationKey]
	if !ok {
		return key, namespace, nil, nil
	}
	triggers := []ObjectFieldTrigger{}
	if err := json.Unmarshal([]byte(t), &triggers); err != nil {
		return key, namespace, nil, err
	}
	if hasDuplicateTriggers(triggers) {
		return key, namespace, nil, fmt.Errorf("duplicate triggers are not allowed")
	}
	return key, namespace, triggers, nil
}

func hasDuplicateTriggers(triggers []ObjectFieldTrigger) bool {
	for i := range triggers {
		for j := i + 1; j < len(triggers); j++ {
			if triggers[i].FieldPath == triggers[j].FieldPath {
				return true
			}
		}
	}
	return false
}

func parseContainerReference(path string) (init bool, selector string, remainder string, ok bool) {
	switch {
	case strings.HasPrefix(path, "containers["):
		remainder = strings.TrimPrefix(path, "containers[")
	case strings.HasPrefix(path, "initContainers["):
		init = true
		remainder = strings.TrimPrefix(path, "initContainers[")
	default:
		return false, "", "", false
	}
	end := strings.Index(remainder, "]")
	if end == -1 {
		return false, "", "", false
	}
	selector = remainder[:end]
	remainder = remainder[end+1:]
	if len(remainder) > 0 && remainder[0] == '.' {
		remainder = remainder[1:]
	}
	return init, selector, remainder, true
}

func findContainerBySelector(spec referencemutator.PodSpecReferenceMutator, init bool, selector string) (referencemutator.ContainerMutator, bool) {
	if i, err := strconv.Atoi(selector); err == nil {
		return spec.GetContainerByIndex(init, i)
	}
	// TODO: potentially make this more flexible, like whitespace
	if name := strings.TrimSuffix(strings.TrimPrefix(selector, "?(@.name==\""), "\")"); name != selector {
		return spec.GetContainerByName(name)
	}
	return nil, false
}

// ContainerForObjectFieldPath returns a reference to the container in the object with pod spec
// underneath fieldPath. Returns error if no such container exists or the field path is invalid.
// Returns the remaining field path beyond the container, if any.
func ContainerForObjectFieldPath(obj runtime.Object, fieldPath string) (referencemutator.ContainerMutator, string, error) {
	spec, err := referencemutator.GetPodSpecReferenceMutator(obj)
	if err != nil {
		return nil, fieldPath, err
	}
	specPath := spec.Path().String()
	containerPath := strings.TrimPrefix(fieldPath, specPath)
	if containerPath == fieldPath {
		return nil, fieldPath, fmt.Errorf("1 field path is not valid: %s", fieldPath)
	}
	containerPath = strings.TrimPrefix(containerPath, ".")
	init, selector, remainder, ok := parseContainerReference(containerPath)
	if !ok {
		return nil, fieldPath, fmt.Errorf("2 field path is not valid: %s", fieldPath)
	}
	container, ok := findContainerBySelector(spec, init, selector)
	if !ok {
		return nil, fieldPath, fmt.Errorf("no such container: %s", selector)
	}
	return container, remainder, nil
}

// UpdateObjectFromImages attempts to set the appropriate object information. If changes are necessary, it lazily copies
// obj and returns it, or if no changes are necessary returns nil.
func UpdateObjectFromImages(obj runtime.Object, tagRetriever TagRetriever) (runtime.Object, error) {
	var updated runtime.Object
	m, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	spec, err := referencemutator.GetPodSpecReferenceMutator(obj)
	if err != nil {
		return nil, err
	}
	path := spec.Path()
	basePath := path.String() + "."
	_, _, triggers, err := CalculateAnnotationTriggers(m, "/")
	if err != nil {
		return nil, err
	}
	klog.V(5).Infof("%T/%s has triggers: %#v", obj, m.GetName(), triggers)
	for _, trigger := range triggers {
		if trigger.Paused {
			continue
		}
		fieldPath := trigger.FieldPath
		if !strings.HasPrefix(trigger.FieldPath, basePath) {
			klog.V(5).Infof("%T/%s trigger %s did not match base path %s", obj, m.GetName(), trigger.FieldPath, basePath)
			continue
		}
		fieldPath = strings.TrimPrefix(fieldPath, basePath)

		namespace := trigger.From.Namespace
		if len(namespace) == 0 {
			namespace = m.GetNamespace()
		}
		ref, _, ok := tagRetriever.ImageStreamTag(namespace, trigger.From.Name)
		if !ok {
			klog.V(5).Infof("%T/%s detected no pending image on %s from %#v", obj, m.GetName(), trigger.FieldPath, trigger.From)
			continue
		}

		init, selector, remainder, ok := parseContainerReference(fieldPath)
		if !ok || remainder != "image" {
			return nil, fmt.Errorf("field path is not valid: %s", trigger.FieldPath)
		}

		container, ok := findContainerBySelector(spec, init, selector)
		if !ok {
			return nil, fmt.Errorf("no such container: %s", trigger.FieldPath)
		}

		if container.GetImage() != ref {
			if updated == nil {
				updated = obj.DeepCopyObject()
				spec, _ = referencemutator.GetPodSpecReferenceMutator(updated)
				container, _ = findContainerBySelector(spec, init, selector)
			}
			klog.V(5).Infof("%T/%s detected change on %s = %s", obj, m.GetName(), trigger.FieldPath, ref)
			container.SetImage(ref)
		}
	}
	return updated, nil
}

// ContainerImageChanged returns true if any container image referenced by newTriggers changed.
func ContainerImageChanged(oldObj, newObj runtime.Object, newTriggers []ObjectFieldTrigger) bool {
	for _, trigger := range newTriggers {
		if trigger.Paused {
			continue
		}

		newContainer, _, err := ContainerForObjectFieldPath(newObj, trigger.FieldPath)
		if err != nil {
			klog.V(5).Infof("%v", err)
			continue
		}

		oldContainer, _, err := ContainerForObjectFieldPath(oldObj, trigger.FieldPath)
		if err != nil {
			// might just be a result of the update
			continue
		}

		if newContainer.GetImage() != oldContainer.GetImage() {
			return true
		}
	}

	return false
}

type AnnotationUpdater interface {
	Update(obj runtime.Object) error
}

type AnnotationReactor struct {
	Updater AnnotationUpdater
}

func (r *AnnotationReactor) ImageChanged(obj runtime.Object, tagRetriever TagRetriever) error {
	changed, err := UpdateObjectFromImages(obj, tagRetriever)
	if err != nil {
		return err
	}
	if changed != nil {
		return r.Updater.Update(changed)
	}
	return nil
}
