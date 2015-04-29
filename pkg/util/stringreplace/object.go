package stringreplace

import (
	"reflect"

	"github.com/golang/glog"
)

// VisitObjectStrings visits recursively all string fields in the object and call the
// visitor function on them. The visitor function can be used to modify the
// value of the string fields.
func VisitObjectStrings(obj interface{}, visitor func(string) string) {
	visitValue(reflect.ValueOf(obj), visitor)
}

func visitValue(v reflect.Value, visitor func(string) string) {
	switch v.Kind() {

	case reflect.Ptr:
		visitValue(v.Elem(), visitor)
	case reflect.Interface:
		visitValue(reflect.ValueOf(v.Interface()), visitor)

	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			visitValue(v.Index(i), visitor)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			visitValue(v.Field(i), visitor)
		}

	case reflect.Map:
		vt := v.Type().Elem()
		for _, k := range v.MapKeys() {
			val := reflect.New(vt).Elem()
			existing := v.MapIndex(k)
			// if the map value type is interface, we must resolve it to a concrete
			// value prior to setting it back.
			if existing.CanInterface() {
				existing = reflect.ValueOf(existing.Interface())
			}
			switch existing.Kind() {
			case reflect.String:
				s := visitor(existing.String())
				val.Set(reflect.ValueOf(s))
			default:
				val.Set(existing)
				visitValue(val, visitor)
			}
			v.SetMapIndex(k, val)
		}

	case reflect.String:
		if !v.CanSet() {
			glog.V(5).Infof("Unable to set String value '%v'", v)
			return
		}
		v.SetString(visitor(v.String()))

	default:
		glog.V(5).Infof("Unknown field type '%s': %v", v.Kind(), v)
	}
}
