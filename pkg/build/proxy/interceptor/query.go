package interceptor

import (
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

// StrictDecodeFromQuery parses query values into a struct. It returns an error if unrecognized
// query parameters are present, if the provided value does not correctly serialize to
// the matching field type, or if the provided object is not a pointer to a struct.
// This method is intentionally strict to prevent new security critical fields from being
// passed unknowningly to a backend.
func StrictDecodeFromQuery(obj interface{}, values url.Values) error {
	return decodeQueryString(obj, values)
}

func decodeQueryString(obj interface{}, values url.Values) error {
	if obj == nil {
		return nil
	}
	value := reflect.ValueOf(obj)
	if value.Kind() != reflect.Ptr {
		return fmt.Errorf("options must be a pointer to a struct %T", obj)
	}
	value = value.Elem()
	if value.Kind() != reflect.Struct {
		return fmt.Errorf("options must be a pointer to a struct %T", obj)
	}
	keys := make(map[string]struct{})
	for i := 0; i < value.NumField(); i++ {
		field := value.Type().Field(i)
		if field.PkgPath != "" {
			continue
		}
		key := field.Tag.Get("qs")
		if key == "" {
			key = strings.ToLower(field.Name)
		}
		if key == "-" {
			continue
		}
		keys[key] = struct{}{}
		if err := setQueryStringValue(values[key], key, value.Field(i)); err != nil {
			return err
		}
	}
	for key := range values {
		if _, ok := keys[key]; !ok {
			return fmt.Errorf("key %q is not recognized", key)
		}
	}
	return nil
}

func setQueryStringValue(values []string, key string, v reflect.Value) error {
	if len(values) == 0 {
		v.Set(reflect.Zero(v.Type()))
		return nil
	}
	last := values[len(values)-1]
	switch v.Kind() {
	case reflect.Bool:
		if last == "1" {
			v.SetBool(true)
		}
	case reflect.Int8:
		i, err := strconv.ParseInt(last, 10, 8)
		if err != nil {
			return err
		}
		v.SetInt(i)
	case reflect.Int16:
		i, err := strconv.ParseInt(last, 10, 16)
		if err != nil {
			return err
		}
		v.SetInt(i)
	case reflect.Int32:
		i, err := strconv.ParseInt(last, 10, 32)
		if err != nil {
			return err
		}
		v.SetInt(i)
	case reflect.Int64, reflect.Int:
		i, err := strconv.ParseInt(last, 10, 64)
		if err != nil {
			return err
		}
		v.SetInt(i)
	case reflect.Float32:
		f, err := strconv.ParseFloat(last, 32)
		if err != nil {
			return err
		}
		v.SetFloat(f)
	case reflect.Float64:
		f, err := strconv.ParseFloat(last, 32)
		if err != nil {
			return err
		}
		v.SetFloat(f)
	case reflect.String:
		v.SetString(last)
	// case reflect.Ptr:
	// 	if !v.IsNil() {
	// 		if b, err := json.Marshal(v.Interface()); err == nil {
	// 			items.Add(key, string(b))
	// 		}
	// 	}
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String || v.Type().Elem().Kind() != reflect.String {
			return fmt.Errorf("map of type %s is not supported for key %s", v.Type(), key)
		}
		keys := make(map[string]string)
		if err := json.Unmarshal([]byte(last), &keys); err != nil {
			return fmt.Errorf("JSON string map for key %s could not be parsed: %v", key, err)
		}
		v.Set(reflect.ValueOf(keys))
	case reflect.Array, reflect.Slice:
		if v.Type().Elem().Kind() == reflect.String {
			v.Set(reflect.ValueOf(values))
		} else {
			// special case, historical empty value?
			if last == "{}" {
				v.Set(reflect.Zero(v.Type()))
				return nil
			}
			if !v.CanAddr() {
				return fmt.Errorf("cannot decode value for key %s because it cannot take an address", key)
			}
			obj := v.Addr().Interface()
			if err := json.Unmarshal([]byte(last), obj); err != nil {
				return fmt.Errorf("JSON array for key %s could not be parsed into %T: %v", key, obj, err)
			}
			v.Set(reflect.ValueOf(obj).Elem())
		}
	default:
		return fmt.Errorf("unrecognized value type %s for key %s", v.Type(), key)
	}
	return nil
}

func EncodeToQuery(opts interface{}) url.Values {
	if opts == nil {
		return nil
	}
	value := reflect.ValueOf(opts)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return nil
	}
	items := url.Values(map[string][]string{})
	for i := 0; i < value.NumField(); i++ {
		field := value.Type().Field(i)
		if field.PkgPath != "" {
			continue
		}
		key := field.Tag.Get("qs")
		if key == "" {
			key = strings.ToLower(field.Name)
		} else if key == "-" {
			continue
		}
		addQueryStringValue(items, key, value.Field(i))
	}
	return items
}

func addQueryStringValue(items url.Values, key string, v reflect.Value) {
	switch v.Kind() {
	case reflect.Bool:
		if v.Bool() {
			items.Add(key, "1")
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.Int() > 0 {
			items.Add(key, strconv.FormatInt(v.Int(), 10))
		}
	case reflect.Float32, reflect.Float64:
		if v.Float() > 0 {
			items.Add(key, strconv.FormatFloat(v.Float(), 'f', -1, 64))
		}
	case reflect.String:
		if v.String() != "" {
			items.Add(key, v.String())
		}
	case reflect.Ptr:
		if !v.IsNil() {
			if b, err := json.Marshal(v.Interface()); err == nil {
				items.Add(key, string(b))
			}
		}
	case reflect.Map:
		if len(v.MapKeys()) > 0 {
			if b, err := json.Marshal(v.Interface()); err == nil {
				items.Add(key, string(b))
			}
		}
	case reflect.Array, reflect.Slice:
		vLen := v.Len()
		if vLen > 0 {
			for i := 0; i < vLen; i++ {
				addQueryStringValue(items, key, v.Index(i))
			}
		}
	}
}
