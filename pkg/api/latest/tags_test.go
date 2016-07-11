package latest

import (
	"reflect"
	"strings"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/api"
)

func TestDescriptions(t *testing.T) {
	for _, version := range Versions {
		if version == OldestVersion {
			// we don't care about descriptions here
			continue
		}

		seen := map[reflect.Type]bool{}

		for _, apiType := range kapi.Scheme.KnownTypes(version) {
			checkDescriptions(apiType, &seen, t)
		}
	}
}

func checkDescriptions(objType reflect.Type, seen *map[reflect.Type]bool, t *testing.T) {
	if _, exists := (*seen)[objType]; exists {
		return
	}
	(*seen)[objType] = true
	if !strings.Contains(objType.PkgPath(), "github.com/openshift/origin/pkg") {
		return
	}

	for i := 0; i < objType.NumField(); i++ {
		structField := objType.FieldByIndex([]int{i})

		// these fields don't need descriptions
		if structField.Name == "TypeMeta" || structField.Name == "ObjectMeta" || structField.Name == "ListMeta" {
			continue
		}
		if structField.Type == reflect.TypeOf(unversioned.Time{}) || structField.Type == reflect.TypeOf(time.Time{}) || structField.Type == reflect.TypeOf(runtime.RawExtension{}) {
			continue
		}

		descriptionTag := structField.Tag.Get("description")
		if len(descriptionTag) > 0 {
			t.Errorf("%v", structField.Tag)
			t.Errorf("%v.%v should not have a description tag", objType, structField.Name)
		}

		switch structField.Type.Kind() {
		case reflect.Struct:
			checkDescriptions(structField.Type, seen, t)
		}
	}
}

func TestInternalJsonTags(t *testing.T) {
	seen := map[reflect.Type]bool{}

	for _, apiType := range kapi.Scheme.KnownTypes(api.SchemeGroupVersion) {
		checkJsonTags(apiType, &seen, t)
	}
}

// internalTypesWithAllowedJsonTags is the list of special structs that have a particular need to have json tags on their
// internal types.  Do not add to this list without having you paperwork checked in triplicate.
var internalTypesWithAllowedJsonTags = sets.NewString("DockerConfig", "DockerImage")

func checkJsonTags(objType reflect.Type, seen *map[reflect.Type]bool, t *testing.T) {
	if _, exists := (*seen)[objType]; exists {
		return
	}
	(*seen)[objType] = true
	if !strings.Contains(objType.PkgPath(), "github.com/openshift/origin/pkg") {
		return
	}
	if internalTypesWithAllowedJsonTags.Has(objType.Name()) {
		return
	}

	for i := 0; i < objType.NumField(); i++ {
		structField := objType.FieldByIndex([]int{i})

		jsonTag := structField.Tag.Get("json")
		if len(jsonTag) != 0 {
			t.Errorf("%v.%v should not have a json tag", objType, structField.Name)
		}

		switch structField.Type.Kind() {
		case reflect.Struct:
			checkJsonTags(structField.Type, seen, t)
		}
	}
}
