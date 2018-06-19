package registry

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/openshift/origin/pkg/util/errors"
)

type NoWatchStorage interface {
	rest.Getter
	rest.Lister
	rest.TableConvertor
	rest.CreaterUpdater
	rest.GracefulDeleter
	rest.Scoper
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

func (s *noWatchStorageErrWrapper) NamespaceScoped() bool {
	return s.delegate.NamespaceScoped()
}

func (s *noWatchStorageErrWrapper) Get(ctx context.Context, name string, options *v1.GetOptions) (runtime.Object, error) {
	obj, err := s.delegate.Get(ctx, name, options)
	return obj, errors.SyncStatusError(ctx, err)
}

func (s *noWatchStorageErrWrapper) List(ctx context.Context, options *internalversion.ListOptions) (runtime.Object, error) {
	obj, err := s.delegate.List(ctx, options)
	return obj, errors.SyncStatusError(ctx, err)
}

func (s *noWatchStorageErrWrapper) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1beta1.Table, error) {
	return s.delegate.ConvertToTable(ctx, object, tableOptions)
}

func (s *noWatchStorageErrWrapper) Create(ctx context.Context, in runtime.Object, createValidation rest.ValidateObjectFunc, includeUninitialized bool) (runtime.Object, error) {
	obj, err := s.delegate.Create(ctx, in, createValidation, includeUninitialized)
	return obj, errors.SyncStatusError(ctx, err)
}

func (s *noWatchStorageErrWrapper) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc) (runtime.Object, bool, error) {
	obj, created, err := s.delegate.Update(ctx, name, objInfo, createValidation, updateValidation)
	return obj, created, errors.SyncStatusError(ctx, err)
}

func (s *noWatchStorageErrWrapper) Delete(ctx context.Context, name string, options *v1.DeleteOptions) (runtime.Object, bool, error) {
	obj, deleted, err := s.delegate.Delete(ctx, name, options)
	return obj, deleted, errors.SyncStatusError(ctx, err)
}

func (s *noWatchStorageErrWrapper) New() runtime.Object {
	return s.delegate.New()
}

func (s *noWatchStorageErrWrapper) NewList() runtime.Object {
	return s.delegate.NewList()
}
