package legacy

import (
	"testing"

	"github.com/openshift/origin/pkg/api/apihelpers/apitesting"
	"k8s.io/apimachinery/pkg/runtime"

	internal "github.com/openshift/origin/pkg/build/apis/build"
)

func TestBuildFieldSelectorConversions(t *testing.T) {
	install := func(scheme *runtime.Scheme) error {
		InstallInternalLegacyBuild(scheme)
		return nil
	}

	apitesting.FieldKeyCheck{
		SchemeBuilder: []func(*runtime.Scheme) error{install},
		Kind:          GroupVersion.WithKind("Build"),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		AllowedExternalFieldKeys: []string{"name", "status", "podName"},
		FieldKeyEvaluatorFn:      internal.BuildFieldSelector,
	}.Check(t)

	apitesting.FieldKeyCheck{
		SchemeBuilder: []func(*runtime.Scheme) error{install},
		Kind:          GroupVersion.WithKind("BuildConfig"),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		AllowedExternalFieldKeys: []string{"name"},
	}.Check(t)

}
