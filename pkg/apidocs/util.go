package apidocs

import (
	"reflect"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/go-openapi/spec"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// RefType returns the type name of a reference, suitable for looking up in
// s.Definitions.
func RefType(s *spec.Schema) string {
	return strings.TrimPrefix(s.Ref.String(), "#/definitions/")
}

// FriendlyTypeName returns a user-friendly type name.
func FriendlyTypeName(s *spec.Schema) string {
	refType := RefType(s)
	if refType == "" { // a base type, e.g. "string"
		return s.Type[0]
	}

	// convert, e.g. "io.k8s.kubernetes.pkg.api.v1.Pod" -> "v1.Pod"
	parts := strings.Split(refType, ".")
	return strings.Join(parts[len(parts)-2:], ".")
}

// EscapeMediaTypes ensures that */* renders correctly in asciidoc format.
// TODO: it'd be better if the template library could escape correctly for
// asciidoc.
func EscapeMediaTypes(mediatypes []string) []string {
	rv := make([]string, len(mediatypes))
	for i, mediatype := range mediatypes {
		rv[i] = mediatype
		if mediatype == "*/*" {
			rv[i] = `\*/*`
		}
	}
	return rv
}

// GroupVersionKinds returns the GroupVersionKinds from the
// "x-kubernetes-group-version-kind" OpenAPI extension.
func GroupVersionKinds(s spec.Schema) []schema.GroupVersionKind {
	e := s.Extensions["x-kubernetes-group-version-kind"]
	if e == nil {
		return nil
	}

	gvks := make([]schema.GroupVersionKind, 0, len(e.([]interface{})))
	for _, gvk := range e.([]interface{}) {
		gvk := gvk.(map[string]interface{})
		gvks = append(gvks, schema.GroupVersionKind{
			Group:   gvk["group"].(string),
			Version: gvk["version"].(string),
			Kind:    gvk["kind"].(string),
		})
	}

	return gvks
}

var opNames = []string{"Get", "Put", "Post", "Delete", "Options", "Head", "Patch"}

// Operations returns the populated operations of a spec.PathItem as a map, for
// easier iteration
func Operations(path spec.PathItem) map[string]*spec.Operation {
	ops := make(map[string]*spec.Operation, len(opNames))

	v := reflect.ValueOf(path)
	for _, opName := range opNames {
		op := v.FieldByName(opName).Interface().(*spec.Operation)
		if op != nil {
			ops[opName] = op
		}
	}
	return ops
}

var envStyleRegexp = regexp.MustCompile(`\{[^}]+\}`)

// EnvStyle replaces instances of {foo} in a string with $FOO.
func EnvStyle(s string) string {
	return envStyleRegexp.ReplaceAllStringFunc(s, func(s string) string {
		return "$" + strings.ToUpper(s[1:len(s)-1])
	})
}

var alreadyPluralSuffixes = []string{"versions", "constraints", "endpoints"}

// Pluralise dumbly attempts to pluralise s.
func Pluralise(s string) string {
	l := strings.ToLower(s)
	for _, ss := range alreadyPluralSuffixes {
		if strings.HasSuffix(l, ss) {
			return s
		}
	}
	if strings.HasSuffix(s, "s") {
		return s + "es"
	}
	if strings.HasSuffix(s, "y") {
		return s[:len(s)-1] + "ies"
	}
	return s + "s"
}

// SortedKeys returns a slice containing the sorted keys of map m.  Argument t
// is the type implementing sort.Interface which is used to sort.
func SortedKeys(m interface{}, t reflect.Type) interface{} {
	v := reflect.ValueOf(m)
	if v.Kind() != reflect.Map {
		panic("wrong type")
	}

	s := reflect.MakeSlice(reflect.SliceOf(v.Type().Key()), v.Len(), v.Len())
	for i, k := range v.MapKeys() {
		s.Index(i).Set(k)
	}

	s = s.Convert(t)
	sort.Sort(s.Interface().(sort.Interface))

	return s.Interface()
}

// ToUpper returns s with the first letter capitalised.
func ToUpper(s string) string {
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// ReverseStringSlice returns s reversed, not in place.
func ReverseStringSlice(s []string) []string {
	r := make([]string, len(s))
	for i := range s {
		r[len(r)-1-i] = s[i]
	}
	return r
}
