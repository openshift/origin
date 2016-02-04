package v1beta3

import (
	"fmt"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = unversioned.GroupVersion{Group: "", Version: "v1beta3"}

// Codec encodes internal objects to the v1beta3 scheme
var Codec = runtime.CodecFor(api.Scheme, SchemeGroupVersion.String())

func init() {
	api.Scheme.AddKnownTypes(SchemeGroupVersion,
		&Route{},
		&RouteList{},
	)
}

func addConversionFuncs() {
	// Add field conversion funcs.
	err := api.Scheme.AddFieldLabelConversionFunc("v1beta3", "Route",
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name",
				"spec.host",
				"spec.path",
				"spec.to.name":
				return label, value, nil
				// This is for backwards compatibility with old v1 clients which send spec.host
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
}

func (*Route) IsAnAPIObject()     {}
func (*RouteList) IsAnAPIObject() {}
