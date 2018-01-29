package apidocs

import (
	"reflect"
	"sort"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/go-openapi/jsonreference"
	"github.com/go-openapi/spec"
)

func TestRefType(t *testing.T) {
	s := RefType(&spec.Schema{
		SchemaProps: spec.SchemaProps{
			Ref: spec.Ref{
				Ref: jsonreference.MustCreateRef("#/definitions/foo"),
			},
		},
	})
	if s != "foo" {
		t.Error(s)
	}
}

func TestFriendlyTypeName(t *testing.T) {
	s := FriendlyTypeName(&spec.Schema{
		SchemaProps: spec.SchemaProps{
			Ref: spec.Ref{
				Ref: jsonreference.MustCreateRef(""),
			},
			Type: spec.StringOrArray{
				"string",
			},
		},
	})
	if s != "string" {
		t.Error(s)
	}

	s = FriendlyTypeName(&spec.Schema{
		SchemaProps: spec.SchemaProps{
			Ref: spec.Ref{
				Ref: jsonreference.MustCreateRef("#/definitions/baz.bar.foo"),
			},
		},
	})
	if s != "bar.foo" {
		t.Error(s)
	}
}

func TestEscapeMediaTypes(t *testing.T) {
	s := EscapeMediaTypes([]string{"foo", "*/*"})
	if !reflect.DeepEqual(s, []string{"foo", `\*/*`}) {
		t.Error(s)
	}
}

func TestGroupVersionKinds(t *testing.T) {
	s := GroupVersionKinds(spec.Schema{
		VendorExtensible: spec.VendorExtensible{
			Extensions: spec.Extensions{
				"x-kubernetes-group-version-kind": []interface{}{
					map[string]interface{}{
						"group":   "group1",
						"version": "version1",
						"kind":    "kind1",
					},
					map[string]interface{}{
						"group":   "group2",
						"version": "version2",
						"kind":    "kind2",
					},
				},
			},
		},
	})

	if !reflect.DeepEqual(s, []schema.GroupVersionKind{
		{
			Group:   "group1",
			Version: "version1",
			Kind:    "kind1",
		},
		{
			Group:   "group2",
			Version: "version2",
			Kind:    "kind2",
		},
	}) {
		t.Error(s)
	}
}

func TestOperations(t *testing.T) {
	var get, put, post, delete, options, head, patch spec.Operation
	s := Operations(spec.PathItem{
		PathItemProps: spec.PathItemProps{
			Get:     &get,
			Put:     &put,
			Post:    &post,
			Delete:  &delete,
			Options: &options,
			Head:    &head,
			Patch:   &patch,
		},
	})

	if !reflect.DeepEqual(s, map[string]*spec.Operation{
		"Get":     &get,
		"Put":     &put,
		"Post":    &post,
		"Delete":  &delete,
		"Options": &options,
		"Head":    &head,
		"Patch":   &patch,
	}) {
		t.Error(s)
	}
}

func TestEnvStyle(t *testing.T) {
	s := EnvStyle("foo/{bar}/baz/{qux}")
	if s != "foo/$BAR/baz/$QUX" {
		t.Error(s)
	}
}

func TestPluralise(t *testing.T) {
	tests := []struct {
		singular string
		plural   string
	}{
		{singular: "APIVersions", plural: "APIVersions"},
		{singular: "Endpoints", plural: "Endpoints"},
		{singular: "SecurityContextConstraints", plural: "SecurityContextConstraints"},
		{singular: "ComponentStatus", plural: "ComponentStatuses"},
		{singular: "Policy", plural: "Policies"},
		{singular: "Pod", plural: "Pods"},
	}

	for i, test := range tests {
		if Pluralise(test.singular) != test.plural {
			t.Errorf("%d: %s: expected %s, got %s", i, test.singular, test.plural, Pluralise(test.singular))
		}
	}
}

func TestSortedKeys(t *testing.T) {
	m := map[string]struct{}{
		"foo": {},
		"bar": {},
		"baz": {},
		"qux": {},
	}

	s := SortedKeys(m, reflect.TypeOf(sort.StringSlice{})).(sort.StringSlice)
	if !reflect.DeepEqual(s, sort.StringSlice{"bar", "baz", "foo", "qux"}) {
		t.Error(s)
	}
}

func TestToUpper(t *testing.T) {
	s := ToUpper("this is a test")
	if s != "This is a test" {
		t.Error(s)
	}
}

func TestReverseStringSlice(t *testing.T) {
	s := []string{"1", "2", "3", "4", "5"}
	c := make([]string, len(s))
	copy(c, s)
	r := ReverseStringSlice(c)

	if !reflect.DeepEqual(s, c) {
		t.Error("ReverseStringSlice mutated argument")
	}
	if !reflect.DeepEqual(r, []string{"5", "4", "3", "2", "1"}) {
		t.Error()
	}

}
