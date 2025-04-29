package property

import (
	"fmt"
	"reflect"
)

func init() {
	scheme = map[reflect.Type]string{
		reflect.TypeOf(&Package{}):         TypePackage,
		reflect.TypeOf(&PackageRequired{}): TypePackageRequired,
		reflect.TypeOf(&GVK{}):             TypeGVK,
		reflect.TypeOf(&GVKRequired{}):     TypeGVKRequired,
		reflect.TypeOf(&BundleObject{}):    TypeBundleObject,
		reflect.TypeOf(&CSVMetadata{}):     TypeCSVMetadata,
		// NOTICE: The Channel properties are for internal use only.
		//   DO NOT use it for any public-facing functionalities.
		//   This API is in alpha stage and it is subject to change.
		reflect.TypeOf(&Channel{}): TypeChannel,
	}
}

var scheme map[reflect.Type]string

func AddToScheme(typ string, p interface{}) {
	t := reflect.TypeOf(p)
	if t.Kind() != reflect.Ptr {
		panic("input must be a pointer to a type")
	}
	if _, ok := scheme[t]; ok {
		panic(fmt.Sprintf("scheme already contains registration for type %q", t))
	}
	scheme[t] = typ
}
