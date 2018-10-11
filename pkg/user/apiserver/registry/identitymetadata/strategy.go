package identitymetadata

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	userapi "github.com/openshift/origin/pkg/user/apis/user"
	"github.com/openshift/origin/pkg/user/apis/user/validation"
)

type strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

var Strategy = &strategy{ObjectTyper: legacyscheme.Scheme, NameGenerator: names.SimpleNameGenerator}

var _ rest.GarbageCollectionDeleteStrategy = &strategy{}

func (*strategy) DefaultGarbageCollectionPolicy(_ context.Context) rest.GarbageCollectionPolicy {
	return rest.Unsupported
}

func (*strategy) NamespaceScoped() bool {
	return false
}

func (*strategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	_ = obj.(*userapi.IdentityMetadata)
}

func (*strategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	_ = obj.(*userapi.IdentityMetadata)
	_ = old.(*userapi.IdentityMetadata)
}

func (*strategy) Canonicalize(obj runtime.Object) {
	metadata := obj.(*userapi.IdentityMetadata)
	// sort and deduplicate
	// this runs after validation so that error messages line up with the original data
	metadata.ProviderGroups = sets.NewString(metadata.ProviderGroups...).List()
}

func (*strategy) AllowCreateOnUpdate() bool {
	return false
}

func (*strategy) AllowUnconditionalUpdate() bool {
	return false
}

func (*strategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateIdentityMetadata(obj.(*userapi.IdentityMetadata))
}

func (*strategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateIdentityMetadataUpdate(obj.(*userapi.IdentityMetadata), old.(*userapi.IdentityMetadata))
}
