package helpers

import (
	"k8s.io/kubernetes/pkg/api"
	kubeerr "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/golang/glog"
)

type filterConverter struct {
	// The underlying REST storage.
	storage ReadAndDeleteStorage

	// Function that converts the storage object into the desired object.
	// Must be able to handle receiving objects other than the one it converts.
	// Changes the behavior of New, Get, Delete and Watch.
	objectDecoratorFunc func(obj runtime.Object) runtime.Object

	// Function that filters out a given object if it does not meet some criteria.
	// Changes the behavior of Get and Delete.
	objectFilterFunc func(ctx api.Context, obj runtime.Object) error

	// Function that converts the list version of the storage object into the desired list object.
	// Does not need to handle receiving objects other than the one it converts.
	// Changes the behavior of New, NewList and DeleteCollection.
	listDecoratorFunc func(obj runtime.Object) runtime.Object

	// Function that constrains the given ListOptions to meet some criteria.
	// Changes the behavior of List, Watch, DeleteCollection.
	listFilterFunc func(ctx api.Context, options *api.ListOptions) (*api.ListOptions, error)

	// Function that mutates the name of user's requested object to match the expected name of underlying store's object.
	// This has the same behavior as overriding the underlying storage's KeyFunc.
	// Changes the behavior of Get and Delete.
	objectNameMutatorFunc func(ctx api.Context, name string) (string, error)

	// Used to make errors denote the converted resource instead of the underlying store object's resource.
	resource unversioned.GroupResource
}

// ReadAndDeleteStorage is the set of interfaces that can perform non-mutating operations along with deletes.
type ReadAndDeleteStorage interface {
	rest.Storage
	rest.Getter
	rest.Lister
	rest.GracefulDeleter
	rest.CollectionDeleter
	rest.Watcher
}

var _ ReadAndDeleteStorage = &filterConverter{}

// NewFilterConverter returns an object the implements ReadAndDeleteStorage.
// It acts as both a conversion and filter on top of the supplied ReadAndDeleteStorage.
// See filterConverter's field documentation for each parameter's specification.
func NewFilterConverter(
	storage ReadAndDeleteStorage,
	objectDecoratorFunc func(obj runtime.Object) runtime.Object,
	objectFilterFunc func(ctx api.Context, obj runtime.Object) error,
	listDecoratorFunc func(obj runtime.Object) runtime.Object,
	listFilterFunc func(ctx api.Context, options *api.ListOptions) (*api.ListOptions, error),
	objectNameMutatorFunc func(ctx api.Context, name string) (string, error),
	resource unversioned.GroupResource,
) *filterConverter {
	return &filterConverter{
		storage:               storage,
		objectDecoratorFunc:   objectDecoratorFunc,
		objectFilterFunc:      objectFilterFunc,
		listDecoratorFunc:     listDecoratorFunc,
		listFilterFunc:        listFilterFunc,
		objectNameMutatorFunc: objectNameMutatorFunc,
		resource:              resource,
	}
}

// Implement rest.Storage using objectDecoratorFunc so that apiserver.APIInstaller.getResourceKind sees a new type.
// New converts the single object.
func (s *filterConverter) New() runtime.Object {
	return s.objectDecoratorFunc(s.storage.New())
}

// Get mutates the object's name, gets it from underlying storage, filters it and then converts it
func (s *filterConverter) Get(ctx api.Context, name string) (runtime.Object, error) {
	newName, err := s.objectNameMutatorFunc(ctx, name)
	if err != nil {
		return nil, err
	}
	obj, err := s.storage.Get(ctx, newName)
	if err != nil {
		if kubeerr.IsNotFound(err) {
			return nil, kubeerr.NewNotFound(s.resource, name)
		}
		glog.Errorf("Unexpected error durng filterConverter Get: %#v", err)
		return nil, kubeerr.NewInternalError(err)
	}
	if err := s.objectFilterFunc(ctx, obj); err != nil {
		return nil, err
	}
	return s.objectDecoratorFunc(obj), nil
}

// NewList is needed to implement rest.Lister (NewList + List)
// It converts the list object.
func (s *filterConverter) NewList() runtime.Object {
	return s.listDecoratorFunc(s.storage.NewList())
}

// List filters the query and then converts the list result.
func (s *filterConverter) List(ctx api.Context, options *api.ListOptions) (runtime.Object, error) {
	options, err := s.listFilterFunc(ctx, options)
	if err != nil {
		return nil, err
	}
	list, err := s.storage.List(ctx, options)
	if err != nil {
		return nil, err
	}
	return s.listDecoratorFunc(list), nil
}

// Delete confirms the object is gettable, mutates the name, delete it and converts the returned object (if the object is one that it can handle).
func (s *filterConverter) Delete(ctx api.Context, name string, options *api.DeleteOptions) (runtime.Object, error) {
	if _, err := s.Get(ctx, name); err != nil {
		return nil, err
	}
	newName, err := s.objectNameMutatorFunc(ctx, name)
	if err != nil {
		return nil, err
	}
	obj, err := s.storage.Delete(ctx, newName, options)
	if err != nil {
		if kubeerr.IsNotFound(err) {
			return nil, kubeerr.NewNotFound(s.resource, name)
		}
		glog.Errorf("Unexpected error durng filterConverter Delete: %#v", err)
		return nil, kubeerr.NewInternalError(err)
	}
	return s.objectDecoratorFunc(obj), nil
}

// DeleteCollection filters the query and then converts the list result.
func (s *filterConverter) DeleteCollection(ctx api.Context, options *api.DeleteOptions, listOptions *api.ListOptions) (runtime.Object, error) {
	listOptions, err := s.listFilterFunc(ctx, listOptions)
	if err != nil {
		return nil, err
	}
	list, err := s.storage.DeleteCollection(ctx, options, listOptions)
	if err != nil {
		return nil, err
	}
	return s.listDecoratorFunc(list), nil
}

// Watch filters the query and then converts each returned object (if the object is one that it can handle).
func (s *filterConverter) Watch(ctx api.Context, options *api.ListOptions) (watch.Interface, error) {
	options, err := s.listFilterFunc(ctx, options)
	if err != nil {
		return nil, err
	}
	w, err := s.storage.Watch(ctx, options)
	if err != nil {
		return nil, err
	}
	return watch.Filter(w, func(in watch.Event) (watch.Event, bool) {
		in.Object = s.objectDecoratorFunc(in.Object)
		return in, true
	}), nil
}
