package util

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	coretyped "k8s.io/client-go/kubernetes/typed/core/v1"
)

func TestIsMicroShiftClusterWithContextCancellation(t *testing.T) {
	baseClient := fake.NewSimpleClientset()
	observedCanceledContext := false
	kubeClient := contextAwareKubeClient{
		Interface: baseClient,
		coreClient: contextAwareCoreClient{
			CoreV1Interface: baseClient.CoreV1(),
			configMaps: contextAwareConfigMaps{
				ConfigMapInterface: baseClient.CoreV1().ConfigMaps("kube-public"),
				observeContext: func(ctx context.Context) {
					observedCanceledContext = ctx.Err() != nil
				},
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	isMicroShift, err := IsMicroShiftClusterWithContext(ctx, kubeClient)

	require.NoError(t, err)
	require.False(t, isMicroShift)
	require.True(t, observedCanceledContext)
}

type contextAwareKubeClient struct {
	kubernetes.Interface
	coreClient coretyped.CoreV1Interface
}

func (c contextAwareKubeClient) CoreV1() coretyped.CoreV1Interface {
	return c.coreClient
}

type contextAwareCoreClient struct {
	coretyped.CoreV1Interface
	configMaps coretyped.ConfigMapInterface
}

func (c contextAwareCoreClient) ConfigMaps(namespace string) coretyped.ConfigMapInterface {
	return c.configMaps
}

type contextAwareConfigMaps struct {
	coretyped.ConfigMapInterface
	observeContext func(context.Context)
}

func (c contextAwareConfigMaps) Get(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.ConfigMap, error) {
	c.observeContext(ctx)
	return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, name)
}
