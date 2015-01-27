package util

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildv "github.com/openshift/origin/pkg/build/api/validation"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployv "github.com/openshift/origin/pkg/deploy/api/validation"
	imageapi "github.com/openshift/origin/pkg/image/api"
	imagev "github.com/openshift/origin/pkg/image/api/validation"
	projectapi "github.com/openshift/origin/pkg/project/api"
	projectv "github.com/openshift/origin/pkg/project/api/validation"
	routeapi "github.com/openshift/origin/pkg/route/api"
	routev "github.com/openshift/origin/pkg/route/api/validation"
)

type fakeServiceLister struct{}

func (f *fakeServiceLister) ListServices(kapi.Context) (*kapi.ServiceList, error) {
	return &kapi.ServiceList{}, nil
}

// ValidateObject validates the runtime.Object and returns the validation errors
func ValidateObject(obj runtime.Object) (errors []error) {
	ctx := kapi.NewDefaultContext()

	if m, err := meta.Accessor(obj); err == nil {
		if len(m.Namespace()) == 0 {
			m.SetNamespace(kapi.NamespaceDefault)
		}
	}

	switch t := obj.(type) {
	case *kapi.ReplicationController:
		errors = validation.ValidateReplicationController(t)
	case *kapi.Service:
		errors = validation.ValidateService(t, &fakeServiceLister{}, ctx)
	case *kapi.Pod:
		errors = validation.ValidatePod(t)
	case *imageapi.Image:
		errors = imagev.ValidateImage(t)
	case *imageapi.ImageRepository:
		// TODO: validate image repository
		// 	errors = imagev.ValidateImageRepository(t)
	case *imageapi.ImageRepositoryMapping:
		errors = imagev.ValidateImageRepositoryMapping(t)
	case *deployapi.DeploymentConfig:
		errors = deployv.ValidateDeploymentConfig(t)
	case *deployapi.Deployment:
		errors = deployv.ValidateDeployment(t)
	case *projectapi.Project:
		// this is a global resource that should not have a namespace
		t.Namespace = ""
		errors = projectv.ValidateProject(t)
	case *routeapi.Route:
		errors = routev.ValidateRoute(t)
	case *buildapi.BuildConfig:
		errors = buildv.ValidateBuildConfig(t)
	case *buildapi.Build:
		errors = buildv.ValidateBuild(t)
	default:
		if list, err := runtime.ExtractList(obj); err == nil {
			for i := range list {
				errs := ValidateObject(list[i])
				errors = append(errors, errs...)
			}
			return
		}
		// TODO: This should not be an error
		return []error{fmt.Errorf("no validation defined for %#v", obj)}
	}
	return errors

}
