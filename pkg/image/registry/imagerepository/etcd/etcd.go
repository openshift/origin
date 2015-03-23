package etcd

import (
	"fmt"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	etcdgeneric "github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/golang/glog"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/imagerepository"
)

// REST implements a RESTStorage for image repositories against etcd.
type REST struct {
	store                       *etcdgeneric.Etcd
	subjectAccessReviewRegistry subjectaccessreview.Registry
	defaultRegistry             imagerepository.DefaultRegistry
}

// NewREST returns a new REST.
func NewREST(h tools.EtcdHelper, defaultRegistry imagerepository.DefaultRegistry, subjectAccessReviewRegistry subjectaccessreview.Registry) (*REST, *StatusREST) {
	prefix := "/imageRepositories"
	store := etcdgeneric.Etcd{
		NewFunc:     func() runtime.Object { return &api.ImageRepository{} },
		NewListFunc: func() runtime.Object { return &api.ImageRepositoryList{} },
		KeyRootFunc: func(ctx kapi.Context) string {
			return etcdgeneric.NamespaceKeyRootFunc(ctx, prefix)
		},
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			return etcdgeneric.NamespaceKeyFunc(ctx, prefix, name)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.ImageRepository).Name, nil
		},
		EndpointName: "imageRepository",

		ReturnDeletedObject: false,
		Helper:              h,
	}

	strategy := imagerepository.NewStrategy(defaultRegistry)

	statusStore := store
	statusStore.UpdateStrategy = imagerepository.NewStatusStrategy(strategy)

	store.CreateStrategy = strategy
	store.UpdateStrategy = strategy
	store.Decorator = strategy.Decorate

	return &REST{store: &store, subjectAccessReviewRegistry: subjectAccessReviewRegistry, defaultRegistry: defaultRegistry}, &StatusREST{store: &statusStore}
}

// New returns a new object
func (r *REST) New() runtime.Object {
	return r.store.NewFunc()
}

// NewList returns a new list object
func (r *REST) NewList() runtime.Object {
	return r.store.NewListFunc()
}

// List obtains a list of image repositories with labels that match selector.
func (r *REST) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	return r.store.ListPredicate(ctx, imagerepository.MatchImageRepository(label, field))
}

// Watch begins watching for new, changed, or deleted image repositories.
func (r *REST) Watch(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return r.store.WatchPredicate(ctx, imagerepository.MatchImageRepository(label, field), resourceVersion)
}

// Get gets a specific image repository specified by its ID.
func (r *REST) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	return r.store.Get(ctx, name)
}

// Create creates a image repository based on a specification.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	repo := obj.(*api.ImageRepository)

	user, ok := kapi.UserFrom(ctx)
	if !ok {
		return nil, kerrors.NewForbidden("imageRepository", repo.Name, fmt.Errorf("Unable to create an image repository without a user on the context"))
	}
	if errors := r.validateTagAccess(repo, user.GetName()); len(errors) > 0 {
		return nil, kerrors.NewInvalid("imageRepository", repo.Name, errors)
	}

	return r.store.Create(ctx, obj)
}

func (r *REST) validateTagAccess(repo *api.ImageRepository, user string) kerrors.ValidationErrorList {
	var errors kerrors.ValidationErrorList
	if repo.Tags != nil {
		for tag, value := range repo.Tags {
			if !strings.Contains(value, "/") {
				glog.V(1).Infof("Tag %q is local; skipping access check", value)
				continue
			}
			ref, err := api.ParseDockerImageReference(value)
			if err != nil {
				errors = append(errors, kerrors.NewFieldInvalid("tags", tag, fmt.Sprintf("Unable to parse %q as a Docker image reference", value)))
			}
			glog.Infof("validating access for %s to %s", user, ref.String())
			defaultRegistry, _ := r.defaultRegistry.DefaultRegistry()
			if ref.Registry != "" && ref.Registry != defaultRegistry {
				glog.Errorf("ref.Registry=%s, defaultRegistry=%s", ref.Registry, defaultRegistry)
				//TODO should we see if we can find an IR whose spec.DockerImageRepository matches
				//ref.Registry and do an access check against that?
				glog.V(1).Infof("Tag %q points to external registry; skipping access check", value)
				continue
			}
			if ref.Namespace == repo.Namespace {
				glog.V(1).Infof("Tag %q points to a repo in the current namespace; skipping access check", value)
				continue
			}
			subjectAccessReview := authorizationapi.SubjectAccessReview{
				Verb:         "get",
				Resource:     "imageRepository",
				User:         user,
				ResourceName: ref.Name,
			}
			ctx := kapi.WithNamespace(kapi.NewContext(), ref.Namespace)
			glog.V(1).Infof("Performing SubjectAccessReview for user %s to %s/%s", user, ref.Namespace, ref.Name)
			resp, err := r.subjectAccessReviewRegistry.CreateSubjectAccessReview(ctx, &subjectAccessReview)
			if err != nil || !resp.Allowed {
				errors = append(errors, kerrors.NewFieldForbidden("tags", fmt.Sprintf("%s=%s", tag, value)))
			}
		}
	}
	return errors
}

// Update changes a image repository specification.
func (r *REST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	repo := obj.(*api.ImageRepository)

	user, ok := kapi.UserFrom(ctx)
	if !ok {
		return nil, false, kerrors.NewForbidden("imageRepository", repo.Name, fmt.Errorf("Unable to update an image repository without a user on the context"))
	}
	if errors := r.validateTagAccess(repo, user.GetName()); len(errors) > 0 {
		return nil, false, kerrors.NewInvalid("imageRepository", repo.Name, errors)
	}

	return r.store.Update(ctx, obj)
}

// Delete deletes an existing image repository specified by its ID.
func (r *REST) Delete(ctx kapi.Context, name string, options *kapi.DeleteOptions) (runtime.Object, error) {
	return r.store.Delete(ctx, name, options)
}

// StatusREST implements the REST endpoint for changing the status of an image repository.
type StatusREST struct {
	store *etcdgeneric.Etcd
}

func (r *StatusREST) New() runtime.Object {
	return &api.ImageRepository{}
}

// Update alters the status subset of an object.
func (r *StatusREST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	return r.store.Update(ctx, obj)
}
