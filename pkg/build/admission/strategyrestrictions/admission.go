package strategyrestrictions

import (
	"fmt"
	"io"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
)

func init() {
	admission.RegisterPlugin("BuildByStrategy", func(c clientset.Interface, config io.Reader) (admission.Interface, error) {
		return NewBuildByStrategy(), nil
	})
}

type buildByStrategy struct {
	*admission.Handler
	client client.Interface
}

var _ = oadmission.WantsOpenshiftClient(&buildByStrategy{})
var _ = oadmission.Validator(&buildByStrategy{})

// NewBuildByStrategy returns an admission control for builds that checks
// on policy based on the build strategy type
func NewBuildByStrategy() admission.Interface {
	return &buildByStrategy{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}
}

var (
	buildsResource       = buildapi.Resource("builds")
	buildConfigsResource = buildapi.Resource("buildconfigs")
)

func (a *buildByStrategy) Admit(attr admission.Attributes) error {
	if resource := attr.GetResource().GroupResource(); resource != buildsResource && resource != buildConfigsResource {
		return nil
	}
	// Explicitly exclude the builds/details subresource because it's only
	// updating commit info and cannot change build type.
	if attr.GetResource().GroupResource() == buildsResource && attr.GetSubresource() == "details" {
		return nil
	}
	switch obj := attr.GetObject().(type) {
	case *buildapi.Build:
		return a.checkAndSetBuildAuthorization(obj, attr)
	case *buildapi.BuildConfig:
		return a.checkAndSetBuildConfigAuthorization(obj, attr)
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

func resourceForStrategyType(strategy buildapi.BuildStrategy) (unversioned.GroupResource, error) {
	switch {
	case strategy.DockerStrategy != nil:
		return buildapi.Resource(authorizationapi.DockerBuildResource), nil
	case strategy.CustomStrategy != nil:
		return buildapi.Resource(authorizationapi.CustomBuildResource), nil
	case strategy.SourceStrategy != nil:
		return buildapi.Resource(authorizationapi.SourceBuildResource), nil
	default:
		return unversioned.GroupResource{}, fmt.Errorf("unrecognized build strategy: %#v", strategy)
	}
}

func resourceName(objectMeta kapi.ObjectMeta) string {
	if len(objectMeta.GenerateName) > 0 {
		return objectMeta.GenerateName
	}
	return objectMeta.Name
}

// setImplicitBuildPermission determines if the user has permission to perform
// docker builds, which is needed if an s2i build needs to perform a docker
// build to add content to the builder image.  If they do not have permission,
// prevent s2i from doing an implicit docker build during the s2i build process.
// Mutates strategy argument.
func (a *buildByStrategy) setImplicitBuildPermission(strategy *buildapi.BuildStrategy, attr admission.Attributes) error {
	if strategy.SourceStrategy != nil && !strategy.SourceStrategy.DisableImplicitBuild {
		resource := buildapi.Resource(authorizationapi.DockerBuildResource)
		subjectAccessReview := &authorizationapi.LocalSubjectAccessReview{
			Action: authorizationapi.AuthorizationAttributes{
				Verb:     "create",
				Group:    resource.Group,
				Resource: resource.Resource,
			},
			User:   attr.GetUserInfo().GetName(),
			Groups: sets.NewString(attr.GetUserInfo().GetGroups()...),
		}
		resp, err := a.client.LocalSubjectAccessReviews(attr.GetNamespace()).Create(subjectAccessReview)
		if err != nil {
			return err
		}
		if !resp.Allowed {
			glog.V(5).Infof("Setting DisableImplicitBuild to true")
			strategy.SourceStrategy.DisableImplicitBuild = true
		}
	}
	return nil
}
func (a *buildByStrategy) checkAndSetBuildAuthorization(build *buildapi.Build, attr admission.Attributes) error {
	strategy := build.Spec.Strategy
	resource, err := resourceForStrategyType(strategy)
	if err != nil {
		return err
	}
	subjectAccessReview := &authorizationapi.LocalSubjectAccessReview{
		Action: authorizationapi.AuthorizationAttributes{
			Verb:         "create",
			Group:        resource.Group,
			Resource:     resource.Resource,
			Content:      build,
			ResourceName: resourceName(build.ObjectMeta),
		},
		User:   attr.GetUserInfo().GetName(),
		Groups: sets.NewString(attr.GetUserInfo().GetGroups()...),
	}
	if err := a.checkAccess(strategy, subjectAccessReview, attr); err != nil {
		return err
	}
	return a.setImplicitBuildPermission(&build.Spec.Strategy, attr)
}

func (a *buildByStrategy) checkAndSetBuildConfigAuthorization(buildConfig *buildapi.BuildConfig, attr admission.Attributes) error {
	strategy := buildConfig.Spec.Strategy
	resource, err := resourceForStrategyType(strategy)
	if err != nil {
		return err
	}
	subjectAccessReview := &authorizationapi.LocalSubjectAccessReview{
		Action: authorizationapi.AuthorizationAttributes{
			Verb:         "create",
			Group:        resource.Group,
			Resource:     resource.Resource,
			Content:      buildConfig,
			ResourceName: resourceName(buildConfig.ObjectMeta),
		},
		User:   attr.GetUserInfo().GetName(),
		Groups: sets.NewString(attr.GetUserInfo().GetGroups()...),
	}
	if err := a.checkAccess(strategy, subjectAccessReview, attr); err != nil {
		return err
	}
	return a.setImplicitBuildPermission(&buildConfig.Spec.Strategy, attr)
}

func (a *buildByStrategy) checkBuildRequestAuthorization(req *buildapi.BuildRequest, attr admission.Attributes) error {
	switch attr.GetResource().GroupResource() {
	case buildsResource:
		build, err := a.client.Builds(attr.GetNamespace()).Get(req.Name)
		if err != nil {
			return err
		}
		return a.checkAndSetBuildAuthorization(build, attr)
	case buildConfigsResource:
		buildcfg, err := a.client.BuildConfigs(attr.GetNamespace()).Get(req.Name)
		if err != nil {
			return err
		}
		return a.checkAndSetBuildConfigAuthorization(buildcfg, attr)
	default:
		return admission.NewForbidden(attr, fmt.Errorf("Unknown resource type %s for BuildRequest", attr.GetResource()))
	}
}

func (a *buildByStrategy) checkAccess(strategy buildapi.BuildStrategy, subjectAccessReview *authorizationapi.LocalSubjectAccessReview, attr admission.Attributes) error {
	resp, err := a.client.LocalSubjectAccessReviews(attr.GetNamespace()).Create(subjectAccessReview)
	if err != nil {
		return err
	}
	if !resp.Allowed {
		return notAllowed(strategy, attr)
	}
	return nil
}

func notAllowed(strategy buildapi.BuildStrategy, attr admission.Attributes) error {
	return admission.NewForbidden(attr, fmt.Errorf("build strategy %s is not allowed", buildapi.StrategyType(strategy)))
}
