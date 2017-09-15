package v1

import (
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/origin/pkg/api/apihelpers"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc("v1", "Identity",
		apihelpers.GetFieldLabelConversionFunc(userapi.IdentityToSelectableFields(&userapi.Identity{}), nil),
	); err != nil {
		return err
	}

	return nil
}
