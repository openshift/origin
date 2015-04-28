package stringreplace

import (
	"reflect"

	"github.com/golang/glog"
)

// VisitObjectStrings visits recursively all string fields in the object and call the
// visitor function on them. The visitor function can be used to modify the
// value of the string fields.
func VisitObjectStrings(obj interface{}, visitor func(string) string) {
	v := reflect.ValueOf(obj).Elem()

	visitField := func(val reflect.Value) {
		switch val.Kind() {
		case reflect.Ptr, reflect.Interface:
			if val.CanInterface() {
				VisitObjectStrings(val.Interface(), visitor)
			}
		case reflect.Struct, reflect.Map, reflect.Slice, reflect.Array:
			if val.CanInterface() {
				VisitObjectStrings(val.Addr().Interface(), visitor)
			}
		case reflect.String:
			if !val.CanSet() {
				glog.V(5).Infof("Unable to set String value '%v'", v)
			}
			val.SetString(visitor(val.String()))
		}
	}

	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		for c := 0; c < v.Len(); c++ {
			visitField(v.Index(c))
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			visitField(v.Field(i))
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			c := reflect.New(v.MapIndex(k).Type()).Elem()
			c.Set(v.MapIndex(k))
			visitField(c)

			// we only want to substitute key when they are string keys
			if k.Kind() == reflect.String {
				s := k.String()
				VisitObjectStrings(&s, visitor)
				newKey := reflect.ValueOf(s)
				v.SetMapIndex(newKey, c)
				v.SetMapIndex(k, reflect.Value{})

			} else {
				v.SetMapIndex(k, c)
			}
		}
	default:
		glog.V(5).Infof("Unknown field type '%s': %v", v.Kind(), v)
		visitField(v)
	}
}
