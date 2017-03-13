package meta

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kapi "k8s.io/kubernetes/pkg/api"

	_ "github.com/openshift/origin/pkg/api/install"
)

func TestResourcesToCheck(t *testing.T) {
	known := knownResourceKinds()
	detected := resourcesToCheck
	for _, k := range detected {
		if _, isKnown := known[k]; !isKnown {
			t.Errorf("Unknown resource kind %s contains a PodSpec", (&k).String())
			continue
		}
		delete(known, k)
	}
	if len(known) > 0 {
		t.Errorf("These known kinds were not detected to have a PodSpec: %#v", known)
	}
}

var podSpecType = reflect.TypeOf(kapi.PodSpec{})

func hasPodSpec(visited map[reflect.Type]bool, t reflect.Type) bool {
	if visited[t] {
		return false
	}
	visited[t] = true

	switch t.Kind() {
	case reflect.Struct:
		if t == podSpecType {
			return true
		}
		for i := 0; i < t.NumField(); i++ {
			if hasPodSpec(visited, t.Field(i).Type) {
				return true
			}
		}
	case reflect.Array, reflect.Slice, reflect.Chan, reflect.Map, reflect.Ptr:
		return hasPodSpec(visited, t.Elem())
	}
	return false
}

func internalGroupVersions() []schema.GroupVersion {
	groupVersions := kapi.Registry.EnabledVersions()
	groups := map[string]struct{}{}
	for _, gv := range groupVersions {
		groups[gv.Group] = struct{}{}
	}
	result := []schema.GroupVersion{}
	for group := range groups {
		result = append(result, schema.GroupVersion{Group: group, Version: runtime.APIVersionInternal})
	}
	return result
}

func isList(t reflect.Type) bool {
	if t.Kind() != reflect.Struct {
		return false
	}

	_, hasListMeta := t.FieldByName("ListMeta")
	return hasListMeta
}

func kindsWithPodSpecs() []schema.GroupKind {
	result := []schema.GroupKind{}
	for _, gv := range internalGroupVersions() {
		knownTypes := kapi.Scheme.KnownTypes(gv)
		for kind, knownType := range knownTypes {
			if !isList(knownType) && hasPodSpec(map[reflect.Type]bool{}, knownType) {
				result = append(result, schema.GroupKind{Group: gv.Group, Kind: kind})
			}
		}
	}

	return result
}

func knownResourceKinds() map[schema.GroupKind]struct{} {
	result := map[schema.GroupKind]struct{}{}
	for _, ka := range resourcesToCheck {
		result[ka] = struct{}{}
	}
	return result
}
