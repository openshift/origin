package imagerepository

import (
	"fmt"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/api/validation"
)

// imageRepositoryStrategy implements behavior for ImageRepositories.
type imageRepositoryStrategy struct {
	runtime.ObjectTyper
	kapi.NameGenerator
	defaultRegistry DefaultRegistry
}

// Strategy is the default logic that applies when creating and updating
// ImageRepository objects via the REST API.
func NewStrategy(defaultRegistry DefaultRegistry) imageRepositoryStrategy {
	return imageRepositoryStrategy{kapi.Scheme, kapi.SimpleNameGenerator, defaultRegistry}
}

// NamespaceScoped is true for image repositories.
func (s imageRepositoryStrategy) NamespaceScoped() bool {
	return true
}

// ResetBeforeCreate clears fields that are not allowed to be set by end users on creation.
func (s imageRepositoryStrategy) ResetBeforeCreate(obj runtime.Object) {
	ir := obj.(*api.ImageRepository)
	ir.Status = api.ImageRepositoryStatus{
		DockerImageRepository: "",
		Tags: make(map[string]api.TagEventList),
	}
}

// Validate validates a new image repository.
func (s imageRepositoryStrategy) Validate(obj runtime.Object) errors.ValidationErrorList {
	ir := obj.(*api.ImageRepository)
	ir.Status.DockerImageRepository = s.dockerImageRepository(ir)
	updateTagHistory(ir)
	return validation.ValidateImageRepository(ir)
}

// AllowCreateOnUpdate is false for image repositories.
func (s imageRepositoryStrategy) AllowCreateOnUpdate() bool {
	return false
}

// dockerImageRepository determines the docker image repository for repo.
// If repo.DockerImageRepository is set, that value is returned. Otherwise,
// if a default registry exists, the value returned is of the form
// <default registry>/<namespace>/<repo name>.
func (s imageRepositoryStrategy) dockerImageRepository(repo *api.ImageRepository) string {
	if len(repo.DockerImageRepository) != 0 {
		return repo.DockerImageRepository
	}

	registry, ok := s.defaultRegistry.DefaultRegistry()
	if !ok {
		return ""
	}

	if len(repo.Namespace) == 0 {
		repo.Namespace = kapi.NamespaceDefault
	}
	return api.JoinDockerPullSpec(registry, repo.Namespace, repo.Name, "")
}

// updateTagHistory updates repo.Status.Tags to add any new or updated tags
// to the history.
func updateTagHistory(repo *api.ImageRepository) {
	// add new tags
	for tag, imageRef := range repo.Tags {
		_, ok := repo.Status.Tags[tag]
		if !ok {
			repo.Status.Tags[tag] = api.TagEventList{}
		}

		var pullSpec string
		if strings.Contains(imageRef, ":") {
			// v2 registry with pull by digest
			pullSpec = fmt.Sprintf("%s@%s", repo.Status.DockerImageRepository, imageRef)
		} else {
			// v1 registry with fake pull by id
			pullSpec = fmt.Sprintf("%s:%s", repo.Status.DockerImageRepository, imageRef)
		}

		entry := repo.Status.Tags[tag]
		if len(entry.Items) == 0 || entry.Items[0].DockerImageReference != pullSpec {
			newTagEvent := api.TagEvent{
				Created:              util.Now(),
				Image:                imageRef,
				DockerImageReference: pullSpec,
			}

			entry.Items = append([]api.TagEvent{newTagEvent}, entry.Items...)
		}
		repo.Status.Tags[tag] = entry
	}

	// TODO should we remove tags deleted from repo.Tags from repo.Status.Tags?
}

// ValidateUpdate is the default update validation for an end user.
func (s imageRepositoryStrategy) ValidateUpdate(obj, old runtime.Object) errors.ValidationErrorList {
	repo := obj.(*api.ImageRepository)
	oldRepo := old.(*api.ImageRepository)

	repo.Status = oldRepo.Status
	if repo.Status.Tags == nil {
		repo.Status.Tags = make(map[string]api.TagEventList)
	}

	repo.Status.DockerImageRepository = s.dockerImageRepository(repo)
	updateTagHistory(repo)

	return validation.ValidateImageRepositoryUpdate(repo, oldRepo)
}

// Decorate decorates repo.Status.DockerImageRepository using the logic from
// dockerImageRepository().
func (s imageRepositoryStrategy) Decorate(obj runtime.Object) error {
	ir := obj.(*api.ImageRepository)
	ir.Status.DockerImageRepository = s.dockerImageRepository(ir)
	return nil
}

// MatchImageRepository returns a generic matcher for a given label and field selector.
func MatchImageRepository(label, field labels.Selector) generic.Matcher {
	return generic.MatcherFunc(func(obj runtime.Object) (bool, error) {
		ir, ok := obj.(*api.ImageRepository)
		if !ok {
			return false, fmt.Errorf("not an image repository")
		}
		fields := ImageRepositoryToSelectableFields(ir)
		return label.Matches(labels.Set(ir.Labels)) && field.Matches(fields), nil
	})
}

// ImageRepositoryToSelectableFields returns a label set that represents the object.
func ImageRepositoryToSelectableFields(ir *api.ImageRepository) labels.Set {
	return labels.Set{
		"name":                         ir.Name,
		"dockerImageRepository":        ir.DockerImageRepository,
		"status.dockerImageRepository": ir.Status.DockerImageRepository,
	}
}

// DefaultRegistry returns the default Docker registry (host or host:port), or false if it is not available.
type DefaultRegistry interface {
	DefaultRegistry() (string, bool)
}

// DefaultRegistryFunc implements DefaultRegistry for a simple function.
type DefaultRegistryFunc func() (string, bool)

// DefaultRegistry implements the DefaultRegistry interface for a function.
func (fn DefaultRegistryFunc) DefaultRegistry() (string, bool) {
	return fn()
}
