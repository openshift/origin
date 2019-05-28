package registry

import (
	"context"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

type NoWatchStorage interface {
	rest.Getter
	rest.Lister
	rest.TableConvertor
	rest.CreaterUpdater
	rest.GracefulDeleter
	rest.Scoper
}

// WrapNoWatchStorageError uses syncStatusError to inject the correct group
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

func (s *noWatchStorageErrWrapper) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	obj, err := s.delegate.Get(ctx, name, options)
	return obj, syncStatusError(ctx, err)
}

func (s *noWatchStorageErrWrapper) List(ctx context.Context, options *internalversion.ListOptions) (runtime.Object, error) {
	obj, err := s.delegate.List(ctx, options)
	return obj, syncStatusError(ctx, err)
}

func (s *noWatchStorageErrWrapper) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1beta1.Table, error) {
	return s.delegate.ConvertToTable(ctx, object, tableOptions)
}

func (s *noWatchStorageErrWrapper) Create(ctx context.Context, in runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	obj, err := s.delegate.Create(ctx, in, createValidation, options)
	return obj, syncStatusError(ctx, err)
}

func (s *noWatchStorageErrWrapper) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	obj, created, err := s.delegate.Update(ctx, name, objInfo, createValidation, updateValidation, forceAllowCreate, options)
	return obj, created, syncStatusError(ctx, err)
}

func (s *noWatchStorageErrWrapper) Delete(ctx context.Context, name string, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	obj, deleted, err := s.delegate.Delete(ctx, name, options)
	return obj, deleted, syncStatusError(ctx, err)
}

func (s *noWatchStorageErrWrapper) New() runtime.Object {
	return s.delegate.New()
}

func (s *noWatchStorageErrWrapper) NewList() runtime.Object {
	return s.delegate.NewList()
}

// syncStatusError makes a best effort attempt to replace the GroupResource
// info in err with the data from the request info of ctx.
func syncStatusError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	statusErr, isStatusErr := err.(apierrors.APIStatus)
	if !isStatusErr {
		return err
	}
	info, hasInfo := apirequest.RequestInfoFrom(ctx)
	if !hasInfo {
		return err
	}
	status := statusErr.Status()
	if status.Details == nil {
		return err
	}
	oldGR := (&schema.GroupResource{Group: status.Details.Group, Resource: status.Details.Kind}).String()
	newGR := (&schema.GroupResource{Group: info.APIGroup, Resource: info.Resource}).String()
	status.Message = strings.Replace(status.Message, oldGR, newGR, 1)
	status.Details.Group = info.APIGroup
	status.Details.Kind = info.Resource // Yes we set Kind field to resource.
	return &apierrors.StatusError{ErrStatus: status}
}
