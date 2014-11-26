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
	objectVal := reflect.ValueOf(object)

	//if we're given a pointer then get the underlying value/type objects that the pointer is pointing to
	if objectType.Kind() == reflect.Ptr {
		objectVal = reflect.Indirect(objectVal)
		objectType = objectType.Elem()
	}

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

			//TODO if there is an embedded pointer type we need to go through the recursion again rather than list as an unsupported type
		}
	}

	return fieldSet
}

//matchers are used in List and Watch commands.  However, if the list/watch is returning n objects it means
//we must construct n maps to run the selector.  This utility method avoids the map overhead if the field selector
//is labels.Everything() which is a common use case
func FieldMatches(obj interface{}, selector labels.Selector) bool{
	return selector.Empty() || selector.Matches(FieldSet(obj))
}
