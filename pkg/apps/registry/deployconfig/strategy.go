package deployconfig

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage/names"
	kapi "k8s.io/kubernetes/pkg/api"

	deployapi "github.com/openshift/origin/pkg/apps/apis/apps"
	deployapiv1 "github.com/openshift/origin/pkg/apps/apis/apps/v1"
	"github.com/openshift/origin/pkg/apps/apis/apps/validation"
)

// strategy implements behavior for DeploymentConfig objects
type strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// CommonStrategy is the default logic that applies when creating and updating DeploymentConfig objects.
var CommonStrategy = strategy{kapi.Scheme, names.SimpleNameGenerator}

// LegacyStrategy is the logic that applies when creating and updating DeploymentConfig objects in the legacy API.
// An example would be setting different defaults depending on API group
var LegacyStrategy = legacyStrategy{CommonStrategy}

// GroupStrategy is the logic that applies when creating and updating DeploymentConfig objects in the group API.
// An example would be setting different defaults depending on API group
var GroupStrategy = groupStrategy{CommonStrategy}

// NamespaceScoped is true for DeploymentConfig objects.
func (strategy) NamespaceScoped() bool {
	return true
}

// AllowCreateOnUpdate is false for DeploymentConfig objects.
func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) AllowUnconditionalUpdate() bool {
	return false
}

func (s strategy) Export(ctx apirequest.Context, obj runtime.Object, exact bool) error {
	s.PrepareForCreate(ctx, obj)
	return nil
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (strategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
	dc := obj.(*deployapi.DeploymentConfig)
	dc.Generation = 1
	dc.Status = deployapi.DeploymentConfigStatus{}

	for i := range dc.Spec.Triggers {
		if params := dc.Spec.Triggers[i].ImageChangeParams; params != nil {
			params.LastTriggeredImage = ""
		}
	}
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (strategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {
	newDc := obj.(*deployapi.DeploymentConfig)
	oldDc := old.(*deployapi.DeploymentConfig)

	newVersion := newDc.Status.LatestVersion
	oldVersion := oldDc.Status.LatestVersion

	// Persist status
	newDc.Status = oldDc.Status

	// oc deploy --latest from old clients
	// TODO: Remove once we drop support for older clients
	if newVersion == oldVersion+1 {
		newDc.Status.LatestVersion = newVersion
	}

	// TODO: Disallow lastTriggeredImage updates from this update path.

	// Any changes to the spec or labels, increment the generation number, any changes
	// to the status should reflect the generation number of the corresponding object
	// (should be handled by the controller).
	if !reflect.DeepEqual(oldDc.Spec, newDc.Spec) || newDc.Status.LatestVersion != oldDc.Status.LatestVersion {
		newDc.Generation = oldDc.Generation + 1
	}
}

// Canonicalize normalizes the object after validation.
func (strategy) Canonicalize(obj runtime.Object) {
}

// Validate validates a new policy.
func (strategy) Validate(ctx apirequest.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateDeploymentConfig(obj.(*deployapi.DeploymentConfig))
}

// ValidateUpdate is the default update validation for an end user.
func (strategy) ValidateUpdate(ctx apirequest.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateDeploymentConfigUpdate(obj.(*deployapi.DeploymentConfig), old.(*deployapi.DeploymentConfig))
}

// CheckGracefulDelete allows a deployment config to be gracefully deleted.
func (strategy) CheckGracefulDelete(obj runtime.Object, options *metav1.DeleteOptions) bool {
	return false
}

// legacyStrategy implements behavior for DeploymentConfig objects in the legacy API
type legacyStrategy struct {
	strategy
}

// PrepareForCreate delegates to the common strategy.
func (s legacyStrategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
	s.strategy.PrepareForCreate(ctx, obj)
}

// DefaultGarbageCollectionPolicy for legacy DeploymentConfigs will orphan dependents.
func (s legacyStrategy) DefaultGarbageCollectionPolicy() rest.GarbageCollectionPolicy {
	return rest.OrphanDependents
}

// groupStrategy implements behavior for DeploymentConfig objects in the Group API
type groupStrategy struct {
	strategy
}

// PrepareForCreate delegates to the common strategy and sets defaults applicable only to Group API
func (s groupStrategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
	s.strategy.PrepareForCreate(ctx, obj)

	dc := obj.(*deployapi.DeploymentConfig)
	deployapiv1.AppsV1DeploymentConfigLayeredDefaults(dc)
}

// statusStrategy implements behavior for DeploymentConfig status updates.
type statusStrategy struct {
	strategy
}

var StatusStrategy = statusStrategy{CommonStrategy}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update of status.
func (statusStrategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {
	newDc := obj.(*deployapi.DeploymentConfig)
	oldDc := old.(*deployapi.DeploymentConfig)
	newDc.Spec = oldDc.Spec
	newDc.Labels = oldDc.Labels
}

// ValidateUpdate is the default update validation for an end user updating status.
func (statusStrategy) ValidateUpdate(ctx apirequest.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateDeploymentConfigStatusUpdate(obj.(*deployapi.DeploymentConfig), old.(*deployapi.DeploymentConfig))
}
