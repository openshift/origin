package v1beta3

import (
	"fmt"

	"k8s.io/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1beta3",
		&Route{},
		&RouteList{},
	)

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
