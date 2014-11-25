package util

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"reflect"
	"strconv"
	"github.com/golang/glog"
)

func FieldSet(object interface{}) labels.Set {
	fieldSet := labels.Set{}

	objectType := reflect.TypeOf(object)

	//if we're given a pointer then get the underlying element
	if objectType.Kind() == reflect.Ptr {
		objectType = objectType.Elem()
	}

	objectVal := reflect.ValueOf(object)

	for i := 0; i < objectVal.NumField(); i++ {
		fieldVal := objectVal.Field(i)

		//if we're dealing with a struct the recurse and add everything.
		if fieldVal.Kind() == reflect.Struct {
			embeddedData := FieldSet(fieldVal.Interface())

			for k, v := range embeddedData {
				fieldSet[k] = v
			}
		}else {
			fieldType := objectType.Field(i)

			switch fieldVal.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				fieldSet[fieldType.Name] = strconv.FormatInt(fieldVal.Int(), 10)
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				fieldSet[fieldType.Name] = strconv.FormatUint(fieldVal.Uint(), 10)
			case reflect.Bool:
				fieldSet[fieldType.Name] = strconv.FormatBool(fieldVal.Bool())
			case reflect.Float32, reflect.Float64:
				fieldSet[fieldType.Name] = strconv.FormatFloat(fieldVal.Float(), 'f', -1, 64)
			case reflect.String:
				fieldSet[fieldType.Name] = fieldVal.String()
			default:
				glog.Warningf("Skipping unsupported field %s of type %s.", fieldType.Name, fieldVal.Kind())
			}
		}
	}

	return fieldSet
}
