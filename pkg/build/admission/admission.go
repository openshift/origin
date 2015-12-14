package admission

import (
	"errors"
	"fmt"
	"io"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
)

func init() {
	admission.RegisterPlugin("BuildByStrategy", func(c kclient.Interface, config io.Reader) (admission.Interface, error) {
		osClient, ok := c.(client.Interface)
		if !ok {
			return nil, errors.New("client is not an Origin client")
		}
		return NewBuildByStrategy(osClient), nil
	})
}

type buildByStrategy struct {
	*admission.Handler
	client client.Interface
}

// NewBuildByStrategy returns an admission control for builds that checks
// on policy based on the build strategy type
func NewBuildByStrategy(client client.Interface) admission.Interface {
	return &buildByStrategy{
		Handler: admission.NewHandler(admission.Create),
		client:  client,
	}
}

const (
	buildsResource       = "builds"
	buildConfigsResource = "buildconfigs"
)

func (a *buildByStrategy) Admit(attr admission.Attributes) error {
	if resource := attr.GetResource(); resource != buildsResource && resource != buildConfigsResource {
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
		return admission.NewForbidden(attr, fmt.Errorf("Unrecognized request object %#v", obj))
	}
}

func resourceForStrategyType(strategy buildapi.BuildStrategy) string {
	switch {
	case strategy.DockerStrategy != nil:
		return authorizationapi.DockerBuildResource
	case strategy.CustomStrategy != nil:
		return authorizationapi.CustomBuildResource
	case strategy.SourceStrategy != nil:
		return authorizationapi.SourceBuildResource
	}
	return ""
}

func resourceName(objectMeta kapi.ObjectMeta) string {
	if len(objectMeta.GenerateName) > 0 {
		return objectMeta.GenerateName
	}
	return objectMeta.Name
}

func (a *buildByStrategy) checkBuildAuthorization(build *buildapi.Build, attr admission.Attributes) error {
	strategy := build.Spec.Strategy
	subjectAccessReview := &authorizationapi.LocalSubjectAccessReview{
		Action: authorizationapi.AuthorizationAttributes{
			Verb:         "create",
			Resource:     resourceForStrategyType(strategy),
			Content:      runtime.EmbeddedObject{Object: build},
			ResourceName: resourceName(build.ObjectMeta),
		},
		User:   attr.GetUserInfo().GetName(),
		Groups: sets.NewString(attr.GetUserInfo().GetGroups()...),
	}
	return a.checkAccess(strategy, subjectAccessReview, attr)
}

func (a *buildByStrategy) checkBuildConfigAuthorization(buildConfig *buildapi.BuildConfig, attr admission.Attributes) error {
	strategy := buildConfig.Spec.Strategy
	subjectAccessReview := &authorizationapi.LocalSubjectAccessReview{
		Action: authorizationapi.AuthorizationAttributes{
			Verb:         "create",
			Resource:     resourceForStrategyType(strategy),
			Content:      runtime.EmbeddedObject{Object: buildConfig},
			ResourceName: resourceName(buildConfig.ObjectMeta),
		},
		User:   attr.GetUserInfo().GetName(),
		Groups: sets.NewString(attr.GetUserInfo().GetGroups()...),
	}
	return a.checkAccess(strategy, subjectAccessReview, attr)
}

func (a *buildByStrategy) checkBuildRequestAuthorization(req *buildapi.BuildRequest, attr admission.Attributes) error {
	switch attr.GetResource() {
	case buildsResource:
		build, err := a.client.Builds(attr.GetNamespace()).Get(req.Name)
		if err != nil {
			return err
		}
		return a.checkBuildAuthorization(build, attr)
	case buildConfigsResource:
		build, err := a.client.BuildConfigs(attr.GetNamespace()).Get(req.Name)
		if err != nil {
			return err
		}
		return a.checkBuildConfigAuthorization(build, attr)
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
