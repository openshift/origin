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

func TestDeleteProxyNamespaceRepeatedCalls(t *testing.T) {
	t.Parallel()
	const namespace = "proxy-test"
	client := kubefake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for i := 0; i < 2; i++ {
		if err := deleteProxyNamespace(ctx, client.CoreV1().Namespaces(), namespace, time.Millisecond); err != nil {
			t.Fatalf("call %d: %v", i+1, err)
		}
	}
}

func TestDeleteProxyNamespaceDeleteDeadline(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	client := kubefake.NewSimpleClientset()
	client.PrependReactor("delete", "namespaces", func(clienttesting.Action) (bool, runtime.Object, error) {
		<-ctx.Done()
		return true, nil, ctx.Err()
	})
	err := deleteProxyNamespace(ctx, client.CoreV1().Namespaces(), "proxy-test", time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want delete deadline exceeded", err)
	}
	if !strings.Contains(err.Error(), "requesting deletion") {
		t.Errorf("error does not describe the failed deletion request: %v", err)
	}
	for _, action := range client.Actions() {
		if action.GetVerb() != "delete" {
			t.Errorf("unexpected action after delete failure: %s", action.GetVerb())
		}
	}
}
