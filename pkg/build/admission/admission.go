package admission

import (
	"errors"
	"fmt"
	"io"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"

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

func resourceForStrategyType(strategyType buildapi.BuildStrategyType) string {
	var resource string
	switch strategyType {
	case buildapi.DockerBuildStrategyType:
		resource = authorizationapi.DockerBuildResource
	case buildapi.CustomBuildStrategyType:
		resource = authorizationapi.CustomBuildResource
	case buildapi.SourceBuildStrategyType:
		resource = authorizationapi.SourceBuildResource
	}
	return resource

}

func resourceName(objectMeta kapi.ObjectMeta) string {
	if len(objectMeta.GenerateName) > 0 {
		return objectMeta.GenerateName
	}
	return objectMeta.Name
}

func (a *buildByStrategy) checkBuildAuthorization(build *buildapi.Build, attr admission.Attributes) error {
	strategyType := build.Spec.Strategy.Type
	subjectAccessReview := &authorizationapi.LocalSubjectAccessReview{
		Action: authorizationapi.AuthorizationAttributes{
			Verb:         "create",
			Resource:     resourceForStrategyType(strategyType),
			Content:      runtime.EmbeddedObject{Object: build},
			ResourceName: resourceName(build.ObjectMeta),
		},
		User:   attr.GetUserInfo().GetName(),
		Groups: util.NewStringSet(attr.GetUserInfo().GetGroups()...),
	}
	return a.checkAccess(strategyType, subjectAccessReview, attr)
}

func (a *buildByStrategy) checkBuildConfigAuthorization(buildConfig *buildapi.BuildConfig, attr admission.Attributes) error {
	strategyType := buildConfig.Spec.Strategy.Type
	subjectAccessReview := &authorizationapi.LocalSubjectAccessReview{
		Action: authorizationapi.AuthorizationAttributes{
			Verb:         "create",
			Resource:     resourceForStrategyType(strategyType),
			Content:      runtime.EmbeddedObject{Object: buildConfig},
			ResourceName: resourceName(buildConfig.ObjectMeta),
		},
		User:   attr.GetUserInfo().GetName(),
		Groups: util.NewStringSet(attr.GetUserInfo().GetGroups()...),
	}
	return a.checkAccess(strategyType, subjectAccessReview, attr)
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

func (a *buildByStrategy) checkAccess(strategyType buildapi.BuildStrategyType, subjectAccessReview *authorizationapi.LocalSubjectAccessReview, attr admission.Attributes) error {
	resp, err := a.client.LocalSubjectAccessReviews(attr.GetNamespace()).Create(subjectAccessReview)
	if err != nil {
		return err
	}
	if !resp.Allowed {
		return notAllowed(strategyType, attr)
	}
	return nil
}

func notAllowed(strategyType buildapi.BuildStrategyType, attr admission.Attributes) error {
	return admission.NewForbidden(attr, fmt.Errorf("build strategy type %s is not allowed", strategyType))
}
