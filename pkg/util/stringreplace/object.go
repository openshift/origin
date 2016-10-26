package stringreplace

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/golang/glog"
)

// VisitObjectStrings recursively visits all string fields in the object and calls the
// visitor function on them. The visitor function can be used to modify the
// value of the string fields.
func VisitObjectStrings(obj interface{}, visitor func(string) (string, bool)) error {
	return visitValue(reflect.ValueOf(obj), visitor)
}

func visitValue(v reflect.Value, visitor func(string) (string, bool)) error {
	// you'll never be able to substitute on a nil.  Check the kind first or you'll accidentally
	// end up panic-ing
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		if v.IsNil() {
			return nil
		}
	}

	switch v.Kind() {

	case reflect.Ptr, reflect.Interface:
		err := visitValue(v.Elem(), visitor)
		if err != nil {
			return err
		}
	case reflect.Slice, reflect.Array:
		vt := v.Type().Elem()
		for i := 0; i < v.Len(); i++ {
			val, err := visitUnsettableValues(vt, v.Index(i), visitor)
			if err != nil {
				return err
			}
			v.Index(i).Set(val)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			err := visitValue(v.Field(i), visitor)
			if err != nil {
				return err
			}
		}
	case reflect.Map:
		vt := v.Type().Elem()
		for _, oldKey := range v.MapKeys() {
			newKey, err := visitUnsettableValues(oldKey.Type(), oldKey, visitor)
			if err != nil {
				return err
			}

			oldValue := v.MapIndex(oldKey)
			newValue, err := visitUnsettableValues(vt, oldValue, visitor)
			if err != nil {
				return err
			}
			v.SetMapIndex(oldKey, reflect.Value{})
			v.SetMapIndex(newKey, newValue)
		}
	case reflect.String:
		if !v.CanSet() {
			return fmt.Errorf("unable to set String value '%v'", v)
		}
		s, asString := visitor(v.String())
		if !asString {
			return fmt.Errorf("attempted to set String field to non-string value '%v'", s)
		}
		v.SetString(s)
	default:
		glog.V(5).Infof("Unknown field type '%s': %v", v.Kind(), v)
		return nil
	}
	return nil
}

// visitUnsettableValues creates a copy of the object you want to modify and returns the modified result
func visitUnsettableValues(typeOf reflect.Type, original reflect.Value, visitor func(string) (string, bool)) (reflect.Value, error) {
	val := reflect.New(typeOf).Elem()
	existing := original
	// if the value type is interface, we must resolve it to a concrete value prior to setting it back.
	if existing.CanInterface() {
		existing = reflect.ValueOf(existing.Interface())
	}
	switch existing.Kind() {
	case reflect.String:
		s, asString := visitor(existing.String())

		if asString {
			val = reflect.ValueOf(s)
		} else {
			b := []byte(s)
			var data interface{}
			err := json.Unmarshal(b, &data)
			if err != nil {
				// the result of substitution may have been an unquoted string value,
				// which is an error when decoding in json(only "true", "false", and numeric
				// values can be unquoted), so try wrapping the value in quotes so it will be
				// properly converted to a string type during decoding.
				val = reflect.ValueOf(s)
			} else {
				val = reflect.ValueOf(data)
			}
		}

	default:
		if existing.IsValid() && existing.Kind() != reflect.Invalid {
			val.Set(existing)
		}
		visitValue(val, visitor)
	}

	return val, nil
}
