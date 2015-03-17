package imagerepository

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/api/validation"
)

// Strategy implements behavior for ImageRepositories.
type Strategy struct {
	runtime.ObjectTyper
	kapi.NameGenerator
	defaultRegistry DefaultRegistry
}

// Strategy is the default logic that applies when creating and updating
// ImageRepository objects via the REST API.
func NewStrategy(defaultRegistry DefaultRegistry) Strategy {
	return Strategy{kapi.Scheme, kapi.SimpleNameGenerator, defaultRegistry}
}

// NamespaceScoped is true for image repositories.
func (s Strategy) NamespaceScoped() bool {
	return true
}

// ResetBeforeCreate clears fields that are not allowed to be set by end users on creation.
func (s Strategy) ResetBeforeCreate(obj runtime.Object) {
	repo := obj.(*api.ImageRepository)
	repo.Status = api.ImageRepositoryStatus{
		DockerImageRepository: s.dockerImageRepository(repo),
		Tags: make(map[string]api.TagEventList),
	}
	tagsChanged(nil, repo)
}

// Validate validates a new image repository.
func (s Strategy) Validate(obj runtime.Object) errors.ValidationErrorList {
	ir := obj.(*api.ImageRepository)
	return validation.ValidateImageRepository(ir)
}

// AllowCreateOnUpdate is false for image repositories.
func (s Strategy) AllowCreateOnUpdate() bool {
	return false
}

// dockerImageRepository determines the docker image repository for repo.
// If repo.DockerImageRepository is set, that value is returned. Otherwise,
// if a default registry exists, the value returned is of the form
// <default registry>/<namespace>/<repo name>.
func (s Strategy) dockerImageRepository(repo *api.ImageRepository) string {
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
	ref := api.DockerImageReference{
		Registry:  registry,
		Namespace: repo.Namespace,
		Name:      repo.Name,
	}
	return ref.String()
}

// tagsChanged updates repo.Status.Tags based on the old and new image repository.
// if the old repository is nil, all tags are considered additions.
func tagsChanged(old, repo *api.ImageRepository) {
	oldTags := map[string]string{}
	if old != nil && old.Tags != nil {
		oldTags = old.Tags
	}
	for tag, value := range repo.Tags {
		if oldValue, ok := oldTags[tag]; ok && value != oldValue {
			// tag changed
			if len(value) > 0 {
				if event, err := api.TagValueToTagEvent(repo, value); err == nil {
					api.AddTagEventToImageRepository(repo, tag, *event)
				}
			}
		}
	}
	for tag, value := range repo.Tags {
		if _, ok := oldTags[tag]; !ok {
			// tag added
			if len(value) > 0 {
				if event, err := api.TagValueToTagEvent(repo, value); err == nil {
					api.AddTagEventToImageRepository(repo, tag, *event)
				}
			}
		}
	}
	// use a consistent timestamp on creation
	if old == nil && !repo.CreationTimestamp.IsZero() {
		for tag, list := range repo.Status.Tags {
			for _, event := range list.Items {
				event.Created = repo.CreationTimestamp
			}
			repo.Status.Tags[tag] = list
		}
	}
}

// ValidateUpdate is the default update validation for an end user.
func (s Strategy) ValidateUpdate(obj, old runtime.Object) errors.ValidationErrorList {
	repo := obj.(*api.ImageRepository)
	oldRepo := old.(*api.ImageRepository)

	repo.Status = oldRepo.Status
	repo.Status.DockerImageRepository = s.dockerImageRepository(repo)

	tagsChanged(oldRepo, repo)
	return validation.ValidateImageRepositoryUpdate(repo, oldRepo)
}

// Decorate decorates repo.Status.DockerImageRepository using the logic from
// dockerImageRepository().
func (s Strategy) Decorate(obj runtime.Object) error {
	ir := obj.(*api.ImageRepository)
	ir.Status.DockerImageRepository = s.dockerImageRepository(ir)
	return nil
}

type StatusStrategy struct {
	Strategy
}

// NewStatusStrategy creates a status update strategy around an existing repository
// strategy.
func NewStatusStrategy(strategy Strategy) StatusStrategy {
	return StatusStrategy{strategy}
}

func (StatusStrategy) ValidateUpdate(obj, old runtime.Object) errors.ValidationErrorList {
	// TODO: merge valid fields after update
	return validation.ValidateImageRepositoryStatusUpdate(obj.(*api.ImageRepository), old.(*api.ImageRepository))
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
