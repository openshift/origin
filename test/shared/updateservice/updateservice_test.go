package updateservice

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net"
	"strconv"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
)

// fakeClusterIP is the cluster IP newFakeClient assigns to created Services.
const fakeClusterIP = "192.0.2.1"

// newFakeClient returns a fake clientset patched to behave like a real API
// server in the two ways Run depends on: the fake clientset neither
// implements GenerateName nor assigns Service cluster IPs.
func newFakeClient(t *testing.T) *fake.Clientset {
	t.Helper()
	clientset := fake.NewSimpleClientset()
	var generated int
	clientset.PrependReactor("create", "*", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		createAction, ok := action.(clientgotesting.CreateAction)
		if !ok {
			return false, nil, nil
		}
		obj := createAction.GetObject()
		accessor, err := meta.Accessor(obj)
		if err != nil {
			return false, nil, nil
		}
		if accessor.GetName() == "" && accessor.GetGenerateName() != "" {
			generated++
			accessor.SetName(fmt.Sprintf("%s%d", accessor.GetGenerateName(), generated))
		}
		if service, ok := obj.(*corev1.Service); ok && service.Spec.ClusterIP == "" {
			service.Spec.ClusterIP = fakeClusterIP
		}
		return false, nil, nil
	})
	return clientset
}

// failOn makes verb on resource fail with err in clientset.
func failOn(clientset *fake.Clientset, verb, resource string, err error) {
	clientset.PrependReactor(verb, resource, func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, err
	})
}

// resourceCounts counts the kinds of resource Run creates in namespace.
type resourceCounts struct {
	configMaps  int
	deployments int
	services    int
}

func countResources(ctx context.Context, t *testing.T, clientset kubernetes.Interface, namespace string) resourceCounts {
	t.Helper()
	configMapList, err := clientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("listing configmaps: %v", err)
	}
	deploymentList, err := clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("listing deployments: %v", err)
	}
	serviceList, err := clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("listing services: %v", err)
	}
	return resourceCounts{
		configMaps:  len(configMapList.Items),
		deployments: len(deploymentList.Items),
		services:    len(serviceList.Items),
	}
}

// TestRunFailureCleanup verifies Run's failure path: when Run fails after
// creating resources, it deletes them before returning so failures do not
// leak resources into the namespace, and joins cleanup failures into the
// returned error.
func TestRunFailureCleanup(t *testing.T) {
	const namespace = "test-namespace"
	injectedErr := errors.New("injected failure")
	injectedCleanupErr := errors.New("injected cleanup failure")

	cases := []struct {
		name string
		// prep injects failure reactors into the clientset.
		prep func(*fake.Clientset)
		// cancelled cancels Run's context before calling it. The fake
		// clientset ignores the context, so the resources are still
		// created and the readiness wait fails fast on the cancelled
		// context, as cancellation and timeout commonly fail real Runs.
		cancelled bool
		// wantErrContains lists substrings expected in Run's error.
		wantErrContains []string
		// wantRemaining lists the resources expected to remain in the
		// namespace after the failed Run.
		wantRemaining resourceCounts
	}{
		{
			name: "configmap creation failure",
			prep: func(c *fake.Clientset) {
				failOn(c, "create", "configmaps", injectedErr)
			},
			wantErrContains: []string{"creating update-service configmap", "injected failure"},
		},
		{
			name: "deployment creation failure",
			prep: func(c *fake.Clientset) {
				failOn(c, "create", "deployments", injectedErr)
			},
			wantErrContains: []string{"creating update-service deployment", "injected failure"},
			// The ConfigMap was created and must be deleted again.
		},
		{
			name: "service creation failure",
			prep: func(c *fake.Clientset) {
				failOn(c, "create", "services", injectedErr)
			},
			wantErrContains: []string{"creating update-service service", "injected failure"},
			// The ConfigMap and Deployment were created and must be
			// deleted again; deleting the never-created Service must be
			// a no-op (its NotFound error is ignored).
		},
		{
			name:            "deployment never ready",
			cancelled:       true,
			wantErrContains: []string{"waiting for update-service deployment", "context canceled"},
			// All three resources were created and must be deleted again.
		},
		{
			name: "cleanup failure joined into error",
			prep: func(c *fake.Clientset) {
				failOn(c, "create", "deployments", injectedErr)
				failOn(c, "delete", "configmaps", injectedCleanupErr)
			},
			wantErrContains: []string{
				"creating update-service deployment",
				"cleaning up after failed Run",
				"injected cleanup failure",
			},
			// The ConfigMap survives because its deletion failed.
			wantRemaining: resourceCounts{configMaps: 1},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clientset := newFakeClient(t)
			if tc.prep != nil {
				tc.prep(clientset)
			}
			ctx := context.Background()
			if tc.cancelled {
				cancelled, cancel := context.WithCancel(context.Background())
				cancel()
				ctx = cancelled
			}

			graph := TwoNodeGraph("stable-4.19", "4.19.0", "current-payload", "4.20.0", "target-payload")
			_, err := Run(ctx, clientset, namespace, "", graph)
			if err == nil {
				t.Fatal("expected Run to fail")
			}
			for _, want := range tc.wantErrContains {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("Run error %q does not contain %q", err, want)
				}
			}

			// Count with a fresh context: Run's cleanup context is
			// detached from Run's, which may itself be cancelled.
			if got := countResources(context.Background(), t, clientset, namespace); got != tc.wantRemaining {
				t.Errorf("resources remaining after failed Run = %+v, want %+v", got, tc.wantRemaining)
			}
		})
	}
}

// reportDeploymentsReady makes the fake clientset report every Deployment
// as ready as soon as the tracker has it, so Run's readiness wait succeeds
// immediately.
func reportDeploymentsReady(clientset *fake.Clientset) {
	clientset.PrependReactor("get", "deployments", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		obj, err := clientset.Tracker().Get(action.GetResource(), action.GetNamespace(), action.(clientgotesting.GetAction).GetName())
		if err != nil {
			return false, nil, nil
		}
		deployment := obj.(*appsv1.Deployment).DeepCopy()
		deployment.Status.AvailableReplicas = 1
		return true, deployment, nil
	})
}

// TestRun covers Run's success path: the returned UpdateService points at
// the Service's graph endpoint, and Cleanup deletes everything Run created.
func TestRun(t *testing.T) {
	const namespace = "test-namespace"
	clientset := newFakeClient(t)
	reportDeploymentsReady(clientset)

	graph := TwoNodeGraph("stable-4.19", "4.19.0", "current-payload", "4.20.0", "target-payload")
	service, err := Run(context.Background(), clientset, namespace, "", graph)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	wantURL := "http://" + net.JoinHostPort(fakeClusterIP, strconv.Itoa(int(containerPort))) + "/" + graphPath
	if service.URL.String() != wantURL {
		t.Errorf("URL = %q, want %q", service.URL, wantURL)
	}
	if got := countResources(context.Background(), t, clientset, namespace); got != (resourceCounts{configMaps: 1, deployments: 1, services: 1}) {
		t.Errorf("resources after Run = %+v, want one ConfigMap, Deployment and Service", got)
	}

	if err := service.Cleanup(context.Background()); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if got := countResources(context.Background(), t, clientset, namespace); got != (resourceCounts{}) {
		t.Errorf("resources after Cleanup = %+v, want none", got)
	}
}

// TestRunLabelIsolation verifies that each Run instance labels its pods
// uniquely, so a second update service in the same namespace does not
// select the first one's pods.
func TestRunLabelIsolation(t *testing.T) {
	const namespace = "test-namespace"
	ctx := context.Background()
	clientset := newFakeClient(t)
	reportDeploymentsReady(clientset)

	graph := TwoNodeGraph("stable-4.19", "4.19.0", "current-payload", "4.20.0", "target-payload")
	for i := 0; i < 2; i++ {
		if _, err := Run(ctx, clientset, namespace, "", graph); err != nil {
			t.Fatalf("Run %d: %v", i+1, err)
		}
	}

	deploymentList, err := clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("listing deployments: %v", err)
	}
	if len(deploymentList.Items) != 2 {
		t.Fatalf("deployments = %d, want 2", len(deploymentList.Items))
	}
	for _, deployment := range deploymentList.Items {
		service, err := clientset.CoreV1().Services(namespace).Get(ctx, deployment.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("getting service %s: %v", deployment.Name, err)
		}
		if !maps.Equal(service.Spec.Selector, deployment.Spec.Template.Labels) {
			t.Errorf("service %s selector %v does not match its own deployment's pod labels %v", deployment.Name, service.Spec.Selector, deployment.Spec.Template.Labels)
		}
		for _, other := range deploymentList.Items {
			if other.Name == deployment.Name {
				continue
			}
			if maps.Equal(service.Spec.Selector, other.Spec.Template.Labels) {
				t.Errorf("service %s selector %v also matches deployment %s's pods", deployment.Name, service.Spec.Selector, other.Name)
			}
		}
	}
}
