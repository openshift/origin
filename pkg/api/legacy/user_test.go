package legacy

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/origin/pkg/api/apihelpers/apitesting"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
)

func TestUserFieldSelectorConversions(t *testing.T) {
	install := func(scheme *runtime.Scheme) error {
		InstallInternalLegacyUser(scheme)
		return nil
	}
	apitesting.FieldKeyCheck{
		SchemeBuilder: []func(*runtime.Scheme) error{install},
		Kind:          GroupVersion.WithKind("Identity"),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		AllowedExternalFieldKeys: []string{"providerName", "providerUserName", "user.name", "user.uid"},
		FieldKeyEvaluatorFn:      userapi.IdentityFieldSelector,
	}.Check(t)

}
