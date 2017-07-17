package strategyrestrictions

import (
	"fmt"
	"io"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/client"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
)

func Register(plugins *admission.Plugins) {
	plugins.Register("BuildByStrategy",
		func(config io.Reader) (admission.Interface, error) {
			return NewBuildByStrategy(), nil
		})
}

type buildByStrategy struct {
	*admission.Handler
	client client.Interface
}

var _ = oadmission.WantsOpenshiftClient(&buildByStrategy{})

// NewBuildByStrategy returns an admission control for builds that checks
// on policy based on the build strategy type
func NewBuildByStrategy() admission.Interface {
	return &buildByStrategy{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}
}

func (a *buildByStrategy) Admit(attr admission.Attributes) error {
	gr := attr.GetResource().GroupResource()
	if !buildapi.IsResourceOrLegacy("buildconfigs", gr) && !buildapi.IsResourceOrLegacy("builds", gr) {
		return nil
	}
	// Explicitly exclude the builds/details subresource because it's only
	// updating commit info and cannot change build type.
	if buildapi.IsResourceOrLegacy("builds", gr) && attr.GetSubresource() == "details" {
		return nil
	}

	// if this is an update, see if we are only updating the ownerRef.  Garbage collection does this
	// and we should allow it in general, since you had the power to update and the power to delete.
	// The worst that happens is that you delete something, but you aren't controlling the privileged object itself
	if attr.GetOldObject() != nil && oadmission.IsOnlyMutatingGCFields(attr.GetObject(), attr.GetOldObject()) {
		return nil
	}

	switch obj := attr.GetObject().(type) {
	case *buildapi.Build:
		return a.checkBuildAuthorization(obj, attr)
	case *buildapi.BuildConfig:
		return a.checkBuildConfigAuthorization(obj, attr)
	case *buildapi.BuildRequest:
		return a.checkBuildRequestAuthorization(obj, attr)
	default:
		return admission.NewForbidden(attr, fmt.Errorf("unrecognized request object %#v", obj))
	}
}

func (a *buildByStrategy) SetOpenshiftClient(c client.Interface) {
	a.client = c
}

func (a *buildByStrategy) Validate() error {
	if a.client == nil {
		return fmt.Errorf("BuildByStrategy needs an Openshift client")
	}
	return nil
}

func resourceForStrategyType(strategy buildapi.BuildStrategy) (schema.GroupResource, error) {
	switch {
	case strategy.DockerStrategy != nil && strategy.DockerStrategy.ImageOptimizationPolicy != nil && *strategy.DockerStrategy.ImageOptimizationPolicy != buildapi.ImageOptimizationNone:
		return buildapi.Resource(authorizationapi.OptimizedDockerBuildResource), nil
	case strategy.DockerStrategy != nil:
		return buildapi.Resource(authorizationapi.DockerBuildResource), nil
	case strategy.CustomStrategy != nil:
		return buildapi.Resource(authorizationapi.CustomBuildResource), nil
	case strategy.SourceStrategy != nil:
		return buildapi.Resource(authorizationapi.SourceBuildResource), nil
	case strategy.JenkinsPipelineStrategy != nil:
		return buildapi.Resource(authorizationapi.JenkinsPipelineBuildResource), nil
	default:
		return schema.GroupResource{}, fmt.Errorf("unrecognized build strategy: %#v", strategy)
	}
}

func resourceName(objectMeta metav1.ObjectMeta) string {
	if len(objectMeta.GenerateName) > 0 {
		return objectMeta.GenerateName
	}
	return objectMeta.Name
}

func (a *buildByStrategy) checkBuildAuthorization(build *buildapi.Build, attr admission.Attributes) error {
	strategy := build.Spec.Strategy
	resource, err := resourceForStrategyType(strategy)
	if err != nil {
		return admission.NewForbidden(attr, err)
	}
	subjectAccessReview := authorizationapi.AddUserToLSAR(attr.GetUserInfo(),
		&authorizationapi.LocalSubjectAccessReview{
			Action: authorizationapi.Action{
				Verb:         "create",
				Group:        resource.Group,
				Resource:     resource.Resource,
				Content:      build,
				ResourceName: resourceName(build.ObjectMeta),
			},
		})
	return a.checkAccess(strategy, subjectAccessReview, attr)
}

func (a *buildByStrategy) checkBuildConfigAuthorization(buildConfig *buildapi.BuildConfig, attr admission.Attributes) error {
	strategy := buildConfig.Spec.Strategy
	resource, err := resourceForStrategyType(strategy)
	if err != nil {
		return admission.NewForbidden(attr, err)
	}
	subjectAccessReview := authorizationapi.AddUserToLSAR(attr.GetUserInfo(),
		&authorizationapi.LocalSubjectAccessReview{
			Action: authorizationapi.Action{
				Verb:         "create",
				Group:        resource.Group,
				Resource:     resource.Resource,
				Content:      buildConfig,
				ResourceName: resourceName(buildConfig.ObjectMeta),
			},
		})
	return a.checkAccess(strategy, subjectAccessReview, attr)
}

func (a *buildByStrategy) checkBuildRequestAuthorization(req *buildapi.BuildRequest, attr admission.Attributes) error {
	gr := attr.GetResource().GroupResource()
	switch {
	case buildapi.IsResourceOrLegacy("builds", gr):
		build, err := a.client.Builds(attr.GetNamespace()).Get(req.Name, metav1.GetOptions{})
		if err != nil {
			return admission.NewForbidden(attr, err)
		}
		return a.checkBuildAuthorization(build, attr)
	case buildapi.IsResourceOrLegacy("buildconfigs", gr):
		build, err := a.client.BuildConfigs(attr.GetNamespace()).Get(req.Name, metav1.GetOptions{})
		if err != nil {
			return admission.NewForbidden(attr, err)
		}
		return a.checkBuildConfigAuthorization(build, attr)
	default:
		return admission.NewForbidden(attr, fmt.Errorf("Unknown resource type %s for BuildRequest", attr.GetResource()))
	}
}

func (a *buildByStrategy) checkAccess(strategy buildapi.BuildStrategy, subjectAccessReview *authorizationapi.LocalSubjectAccessReview, attr admission.Attributes) error {
	resp, err := a.client.LocalSubjectAccessReviews(attr.GetNamespace()).Create(subjectAccessReview)
	if err != nil {
		return admission.NewForbidden(attr, err)
	}
	// If not allowed, try to check against the legacy resource
	// FIXME: Remove this when the legacy API is deprecated
	if !resp.Allowed {
		obj, err := kapi.Scheme.DeepCopy(subjectAccessReview)
		if err != nil {
			return admission.NewForbidden(attr, err)
		}
		legacySar := obj.(*authorizationapi.LocalSubjectAccessReview)
		legacySar.Action.Group = ""
		resp, err := a.client.LocalSubjectAccessReviews(attr.GetNamespace()).Create(legacySar)
		if err != nil {
			return admission.NewForbidden(attr, err)
		}
		if !resp.Allowed {
			return notAllowed(strategy, attr)
		}
	}
	return nil
}

func notAllowed(strategy buildapi.BuildStrategy, attr admission.Attributes) error {
	return admission.NewForbidden(attr, fmt.Errorf("build strategy %s is not allowed", buildapi.StrategyType(strategy)))
}
