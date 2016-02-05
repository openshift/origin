package v1

import (
	"k8s.io/kubernetes/pkg/runtime"

	oapi "github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/user/api"
)

func addConversionFuncs(scheme *runtime.Scheme) {
	if err := scheme.AddFieldLabelConversionFunc("v1", "Group",
		oapi.GetFieldLabelConversionFunc(api.GroupToSelectableFields(&api.Group{}), nil),
	); err != nil {
		panic(err)
	}

	if err := scheme.AddFieldLabelConversionFunc("v1", "Identity",
		oapi.GetFieldLabelConversionFunc(api.IdentityToSelectableFields(&api.Identity{}), nil),
	); err != nil {
		panic(err)
	}

	if err := scheme.AddFieldLabelConversionFunc("v1", "User",
		oapi.GetFieldLabelConversionFunc(api.UserToSelectableFields(&api.User{}), nil),
	); err != nil {
		panic(err)
	}
}
