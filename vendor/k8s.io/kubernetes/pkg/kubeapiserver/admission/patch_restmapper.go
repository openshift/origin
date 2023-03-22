package admission

import (
	"github.com/openshift/library-go/pkg/client/openshiftrestmapper"

	"k8s.io/apimachinery/pkg/api/meta"
)

func NewAdmissionRESTMapper(delegate meta.RESTMapper) meta.RESTMapper {
	return openshiftrestmapper.NewOpenShiftHardcodedRESTMapper(delegate)
}
