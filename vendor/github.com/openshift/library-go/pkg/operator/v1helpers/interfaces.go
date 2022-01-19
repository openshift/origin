package v1helpers

import (
	"context"

	operatorv1 "github.com/openshift/api/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

type OperatorClient interface {
	Informer() cache.SharedIndexInformer
	// GetObjectMeta return the operator metadata.
	GetObjectMeta() (meta *metav1.ObjectMeta, err error)
	// GetOperatorState returns the operator spec, status and the resource version, potentially from a lister.
	GetOperatorState() (spec *operatorv1.OperatorSpec, status *operatorv1.OperatorStatus, resourceVersion string, err error)
	// UpdateOperatorSpec updates the spec of the operator, assuming the given resource version.
	UpdateOperatorSpec(ctx context.Context, oldResourceVersion string, in *operatorv1.OperatorSpec) (out *operatorv1.OperatorSpec, newResourceVersion string, err error)
	// UpdateOperatorStatus updates the status of the operator, assuming the given resource version.
	UpdateOperatorStatus(ctx context.Context, oldResourceVersion string, in *operatorv1.OperatorStatus) (out *operatorv1.OperatorStatus, err error)
}

type StaticPodOperatorClient interface {
	OperatorClient
	// GetStaticPodOperatorState returns the static pod operator spec, status and the resource version,
	// potentially from a lister.
	GetStaticPodOperatorState() (spec *operatorv1.StaticPodOperatorSpec, status *operatorv1.StaticPodOperatorStatus, resourceVersion string, err error)
	// GetStaticPodOperatorStateWithQuorum return the static pod operator spec, status and resource version
	// directly from a server read.
	GetStaticPodOperatorStateWithQuorum(ctx context.Context) (spec *operatorv1.StaticPodOperatorSpec, status *operatorv1.StaticPodOperatorStatus, resourceVersion string, err error)
	// UpdateStaticPodOperatorStatus updates the status, assuming the given resource version.
	UpdateStaticPodOperatorStatus(ctx context.Context, resourceVersion string, in *operatorv1.StaticPodOperatorStatus) (out *operatorv1.StaticPodOperatorStatus, err error)
	// UpdateStaticPodOperatorSpec updates the spec, assuming the given resource  version.
	UpdateStaticPodOperatorSpec(ctx context.Context, resourceVersion string, in *operatorv1.StaticPodOperatorSpec) (out *operatorv1.StaticPodOperatorSpec, newResourceVersion string, err error)
}

type OperatorClientWithFinalizers interface {
	OperatorClient
	// EnsureFinalizer adds a new finalizer to the operator CR, if it does not exists. No-op otherwise.
	EnsureFinalizer(ctx context.Context, finalizer string) error
	// RemoveFinalizer removes a finalizer from the operator CR, if it is there. No-op otherwise.
	RemoveFinalizer(ctx context.Context, finalizer string) error
}
