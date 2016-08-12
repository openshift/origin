package meta

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	"k8s.io/kubernetes/pkg/runtime"

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

func internalGroupVersions() []unversioned.GroupVersion {
	groupVersions := registered.EnabledVersions()
	groups := map[string]struct{}{}
	for _, gv := range groupVersions {
		groups[gv.Group] = struct{}{}
	}
	result := []unversioned.GroupVersion{}
	for group := range groups {
		result = append(result, unversioned.GroupVersion{Group: group, Version: runtime.APIVersionInternal})
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

func kindsWithPodSpecs() []unversioned.GroupKind {
	result := []unversioned.GroupKind{}
	for _, gv := range internalGroupVersions() {
		knownTypes := kapi.Scheme.KnownTypes(gv)
		for kind, knownType := range knownTypes {
			if !isList(knownType) && hasPodSpec(map[reflect.Type]bool{}, knownType) {
				result = append(result, unversioned.GroupKind{Group: gv.Group, Kind: kind})
			}
		}
	}

	return result
}

func knownResourceKinds() map[unversioned.GroupKind]struct{} {
	result := map[unversioned.GroupKind]struct{}{}
	for _, ka := range resourcesToCheck {
		result[ka] = struct{}{}
	}
	return result
}
