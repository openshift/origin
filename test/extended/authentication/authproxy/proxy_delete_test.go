package authproxy

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubefake "k8s.io/client-go/kubernetes/fake"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	clienttesting "k8s.io/client-go/testing"
)

func TestDeleteProxyNamespace(t *testing.T) {
	t.Parallel()
	deleteErr := errors.New("delete failed")
	readErr := errors.New("get failed")
	for _, tc := range []struct {
		name        string
		alreadyGone bool
		deleteErr   error
		keepPresent bool
		readsToGone int
		readErr     error
		cancelOnGet bool
		wantErr     error
	}{
		{name: "already absent", alreadyGone: true},
		{name: "deleted immediately"},
		{name: "delete failure", deleteErr: deleteErr, wantErr: deleteErr},
		{name: "namespace present before removal", keepPresent: true, readsToGone: 2},
		{name: "read failure before removal", keepPresent: true, readErr: readErr, readsToGone: 2},
		{name: "namespace remains until timeout", keepPresent: true, wantErr: context.DeadlineExceeded},
		{name: "read failures until timeout", keepPresent: true, readErr: readErr, wantErr: context.DeadlineExceeded},
		{name: "canceled while waiting", keepPresent: true, cancelOnGet: true, wantErr: context.Canceled},
		{name: "canceled after read failure", keepPresent: true, readErr: readErr, cancelOnGet: true, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			const namespace = "proxy-test"
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
			client := kubefake.NewSimpleClientset()
			if !tc.alreadyGone {
				if err := client.Tracker().Add(ns); err != nil {
					t.Fatal(err)
				}
			}
			timeout := time.Second
			if tc.wantErr == context.DeadlineExceeded {
				timeout = 50 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			client.PrependReactor("delete", "namespaces", func(action clienttesting.Action) (bool, runtime.Object, error) {
				deleteAction := action.(clienttesting.DeleteAction)
				if deleteAction.GetName() != namespace {
					t.Errorf("delete targeted namespace %q", deleteAction.GetName())
				}
				policy := deleteAction.GetDeleteOptions().PropagationPolicy
				if policy == nil || *policy != metav1.DeletePropagationForeground {
					t.Error("delete must use foreground propagation")
				}
				if tc.deleteErr != nil {
					return true, nil, tc.deleteErr
				}
				return tc.keepPresent, nil, nil
			})
			reads := 0
			client.PrependReactor("get", "namespaces", func(action clienttesting.Action) (bool, runtime.Object, error) {
				reads++
				if action.(clienttesting.GetAction).GetName() != namespace {
					t.Error("get targeted the wrong namespace")
				}
				if tc.cancelOnGet {
					cancel()
				}
				if tc.readsToGone > 0 && reads >= tc.readsToGone {
					return true, nil, apierrors.NewNotFound(schema.GroupResource{Resource: "namespaces"}, namespace)
				}
				if tc.readErr != nil {
					return true, nil, tc.readErr
				}
				return false, nil, nil
			})
			err := deleteProxyNamespace(ctx, client.CoreV1().Namespaces(), namespace, time.Millisecond)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("error = %v, want %v", err, tc.wantErr)
				}
				if tc.deleteErr == nil && !strings.Contains(err.Error(), "deletion was requested but completion was not confirmed") {
					t.Errorf("error does not describe incomplete deletion: %v", err)
				}
				if tc.readErr != nil && !errors.Is(err, tc.readErr) {
					t.Errorf("error lost the last namespace read failure: %v", err)
				}
			}
			if (tc.alreadyGone || tc.deleteErr != nil) && reads != 0 {
				t.Error("polled after namespace was absent or delete failed")
			}
			if tc.readsToGone > 0 && reads < tc.readsToGone {
				t.Error("returned before namespace removal was confirmed")
			}
		})
	}
}

// The second call pins deleteProxyNamespace's absent-namespace-is-success contract,
// which Delete relies on to be registerable unconditionally in cleanup.
func TestDeleteProxyNamespaceRepeatedCalls(t *testing.T) {
	t.Parallel()
	const namespace = "proxy-test"
	client := kubefake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for i := range 2 {
		if err := deleteProxyNamespace(ctx, client.CoreV1().Namespaces(), namespace, time.Millisecond); err != nil {
			t.Fatalf("call %d: %v", i+1, err)
		}
	}
}

// deleteProxyNamespace must pass the caller's deadline to the Delete request so a
// hung API call cannot outlive the bound the caller set.
func TestDeleteProxyNamespaceDeleteDeadline(t *testing.T) {
	t.Parallel()
	deadline := time.Now().Add(time.Hour)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	namespaces := &deleteDeadlineNamespaces{}
	if err := deleteProxyNamespace(ctx, namespaces, "proxy-test", time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if !namespaces.deleteCalled {
		t.Fatal("Delete was never called")
	}
	got, ok := namespaces.deleteCtx.Deadline()
	if !ok || !got.Equal(deadline) {
		t.Errorf("Delete deadline = %v, %v; want %v, true", got, ok, deadline)
	}
}

// deleteDeadlineNamespaces records the context passed to Delete. The embedded
// interface stays nil: any method other than Delete and Get panics.
type deleteDeadlineNamespaces struct {
	coreclientv1.NamespaceInterface
	deleteCtx    context.Context
	deleteCalled bool
}

func (d *deleteDeadlineNamespaces) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	d.deleteCtx = ctx
	d.deleteCalled = true
	return nil
}

func (d *deleteDeadlineNamespaces) Get(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.Namespace, error) {
	return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "namespaces"}, name)
}
