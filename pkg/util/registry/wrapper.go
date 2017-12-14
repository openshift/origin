package registry

import (
	"k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/openshift/origin/pkg/util/errors"
)

type NoWatchStorage interface {
	rest.Getter
	rest.Lister
	rest.CreaterUpdater
	rest.GracefulDeleter
}

// WrapNoWatchStorageError uses SyncStatusError to inject the correct group
// resource info into the errors that are returned by the delegated storage
func WrapNoWatchStorageError(delegate NoWatchStorage) NoWatchStorage {
	return &noWatchStorageErrWrapper{delegate: delegate}
}

var _ = NoWatchStorage(&noWatchStorageErrWrapper{})

type noWatchStorageErrWrapper struct {
	delegate NoWatchStorage
}

func (s *noWatchStorageErrWrapper) Get(ctx request.Context, name string, options *v1.GetOptions) (runtime.Object, error) {
	obj, err := s.delegate.Get(ctx, name, options)
	return obj, errors.SyncStatusError(ctx, err)
}

func (s *noWatchStorageErrWrapper) List(ctx request.Context, options *internalversion.ListOptions) (runtime.Object, error) {
	obj, err := s.delegate.List(ctx, options)
	return obj, errors.SyncStatusError(ctx, err)
}

func (s *noWatchStorageErrWrapper) Create(ctx request.Context, in runtime.Object, createValidation rest.ValidateObjectFunc, includeUninitialized bool) (runtime.Object, error) {
	obj, err := s.delegate.Create(ctx, in, createValidation, includeUninitialized)
	return obj, errors.SyncStatusError(ctx, err)
}

func (s *noWatchStorageErrWrapper) Update(ctx request.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc) (runtime.Object, bool, error) {
	obj, created, err := s.delegate.Update(ctx, name, objInfo, createValidation, updateValidation)
	return obj, created, errors.SyncStatusError(ctx, err)
}

func (s *noWatchStorageErrWrapper) Delete(ctx request.Context, name string, options *v1.DeleteOptions) (runtime.Object, bool, error) {
	obj, deleted, err := s.delegate.Delete(ctx, name, options)
	return obj, deleted, errors.SyncStatusError(ctx, err)
}

func (s *noWatchStorageErrWrapper) New() runtime.Object {
	return s.delegate.New()
}

func (s *noWatchStorageErrWrapper) NewList() runtime.Object {
	return s.delegate.NewList()
}
