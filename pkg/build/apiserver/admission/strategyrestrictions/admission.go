package strategyrestrictions

import (
	"fmt"
	"io"
	"strings"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/client-go/kubernetes"
	authorizationclient "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/rest"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"
	rbacregistry "k8s.io/kubernetes/pkg/registry/rbac"

	"github.com/openshift/api/build"
	buildclient "github.com/openshift/client-go/build/clientset/versioned"
	"github.com/openshift/origin/pkg/api/legacy"
	"github.com/openshift/origin/pkg/authorization/util"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/build/buildscheme"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"k8s.io/apiserver/pkg/admission/initializer"
)

func Register(plugins *admission.Plugins) {
	plugins.Register("BuildByStrategy",
		func(config io.Reader) (admission.Interface, error) {
			return NewBuildByStrategy(), nil
		})
}

type buildByStrategy struct {
	*admission.Handler
	sarClient   authorizationclient.SubjectAccessReviewInterface
	buildClient buildclient.Interface
}

var _ = initializer.WantsExternalKubeClientSet(&buildByStrategy{})
var _ = oadmission.WantsRESTClientConfig(&buildByStrategy{})

// NewBuildByStrategy returns an admission control for builds that checks
// on policy based on the build strategy type
func NewBuildByStrategy() admission.Interface {
	return &buildByStrategy{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}
}

func (a *buildByStrategy) Admit(attr admission.Attributes) error {
	gr := attr.GetResource().GroupResource()
	switch gr {
	case build.Resource("buildconfigs"),
		legacy.Resource("buildconfigs"):
	case build.Resource("builds"),
		legacy.Resource("builds"):
		// Explicitly exclude the builds/details subresource because it's only
		// updating commit info and cannot change build type.
		if attr.GetSubresource() == "details" {
			return nil
		}
	default:
		return nil
	}

	// if this is an update, see if we are only updating the ownerRef.  Garbage collection does this
	// and we should allow it in general, since you had the power to update and the power to delete.
	// The worst that happens is that you delete something, but you aren't controlling the privileged object itself
	if attr.GetOldObject() != nil && rbacregistry.IsOnlyMutatingGCFields(attr.GetObject(), attr.GetOldObject(), kapihelper.Semantic) {
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

func (a *buildByStrategy) SetExternalKubeClientSet(c kubernetes.Interface) {
	a.sarClient = c.AuthorizationV1().SubjectAccessReviews()
}

func (a *buildByStrategy) SetRESTClientConfig(restClientConfig rest.Config) {
	var err error
	a.buildClient, err = buildclient.NewForConfig(&restClientConfig)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
}

func (a *buildByStrategy) ValidateInitialization() error {
	if a.buildClient == nil {
		return fmt.Errorf("BuildByStrategy needs an Openshift buildClient")
	}
	if a.sarClient == nil {
		return fmt.Errorf("BuildByStrategy needs an Openshift sarClient")
	}
	return nil
}

func resourceForStrategyType(strategy buildapi.BuildStrategy) (schema.GroupResource, error) {
	switch {
	case strategy.DockerStrategy != nil && strategy.DockerStrategy.ImageOptimizationPolicy != nil && *strategy.DockerStrategy.ImageOptimizationPolicy != buildapi.ImageOptimizationNone:
		return build.Resource(bootstrappolicy.OptimizedDockerBuildResource), nil
	case strategy.DockerStrategy != nil:
		return build.Resource(bootstrappolicy.DockerBuildResource), nil
	case strategy.CustomStrategy != nil:
		return build.Resource(bootstrappolicy.CustomBuildResource), nil
	case strategy.SourceStrategy != nil:
		return build.Resource(bootstrappolicy.SourceBuildResource), nil
	case strategy.JenkinsPipelineStrategy != nil:
		return build.Resource(bootstrappolicy.JenkinsPipelineBuildResource), nil
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
	subresource := ""
	tokens := strings.SplitN(resource.Resource, "/", 2)
	resourceType := tokens[0]
	if len(tokens) == 2 {
		subresource = tokens[1]
	}

	sar := util.AddUserToSAR(attr.GetUserInfo(), &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace:   attr.GetNamespace(),
				Verb:        "create",
				Group:       resource.Group,
				Resource:    resourceType,
				Subresource: subresource,
				Name:        resourceName(build.ObjectMeta),
			},
		},
	})
	return a.checkAccess(strategy, sar, attr)
}

func (a *buildByStrategy) checkBuildConfigAuthorization(buildConfig *buildapi.BuildConfig, attr admission.Attributes) error {
	strategy := buildConfig.Spec.Strategy
	resource, err := resourceForStrategyType(strategy)
	if err != nil {
		return admission.NewForbidden(attr, err)
	}
	subresource := ""
	tokens := strings.SplitN(resource.Resource, "/", 2)
	resourceType := tokens[0]
	if len(tokens) == 2 {
		subresource = tokens[1]
	}

	sar := util.AddUserToSAR(attr.GetUserInfo(), &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace:   attr.GetNamespace(),
				Verb:        "create",
				Group:       resource.Group,
				Resource:    resourceType,
				Subresource: subresource,
				Name:        resourceName(buildConfig.ObjectMeta),
			},
		},
	})
	return a.checkAccess(strategy, sar, attr)
}

func (a *buildByStrategy) checkBuildRequestAuthorization(req *buildapi.BuildRequest, attr admission.Attributes) error {
	gr := attr.GetResource().GroupResource()
	switch gr {
	case build.Resource("builds"),
		legacy.Resource("builds"):
		build, err := a.buildClient.BuildV1().Builds(attr.GetNamespace()).Get(req.Name, metav1.GetOptions{})
		if err != nil {
			return admission.NewForbidden(attr, err)
		}
		internalBuild := &buildapi.Build{}
		if err := buildscheme.InternalExternalScheme.Convert(build, internalBuild, nil); err != nil {
			return admission.NewForbidden(attr, err)
		}
		return a.checkBuildAuthorization(internalBuild, attr)

	case build.Resource("buildconfigs"),
		legacy.Resource("buildconfigs"):
		buildConfig, err := a.buildClient.BuildV1().BuildConfigs(attr.GetNamespace()).Get(req.Name, metav1.GetOptions{})
		if err != nil {
			return admission.NewForbidden(attr, err)
		}
		internalBuildConfig := &buildapi.BuildConfig{}
		if err := buildscheme.InternalExternalScheme.Convert(buildConfig, internalBuildConfig, nil); err != nil {
			return admission.NewForbidden(attr, err)
		}
		return a.checkBuildConfigAuthorization(internalBuildConfig, attr)
	default:
		return admission.NewForbidden(attr, fmt.Errorf("Unknown resource type %s for BuildRequest", attr.GetResource()))
	}
}

func (a *buildByStrategy) checkAccess(strategy buildapi.BuildStrategy, subjectAccessReview *authorizationv1.SubjectAccessReview, attr admission.Attributes) error {
	resp, err := a.sarClient.Create(subjectAccessReview)
	if err != nil {
		return admission.NewForbidden(attr, err)
	}
	if !resp.Status.Allowed {
		return notAllowed(strategy, attr)
	}
	return nil
}

func notAllowed(strategy buildapi.BuildStrategy, attr admission.Attributes) error {
	return admission.NewForbidden(attr, fmt.Errorf("build strategy %s is not allowed", strategyTypeString(strategy)))
}

func strategyTypeString(strategy buildapi.BuildStrategy) string {
	switch {
	case strategy.DockerStrategy != nil:
		return "Docker"
	case strategy.CustomStrategy != nil:
		return "Custom"
	case strategy.SourceStrategy != nil:
		return "Source"
	case strategy.JenkinsPipelineStrategy != nil:
		return "JenkinsPipeline"
	}
	return ""
}
