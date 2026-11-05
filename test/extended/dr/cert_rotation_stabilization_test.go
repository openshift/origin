package dr

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

type certRotationPodClient struct {
	corev1client.PodInterface
	list func(context.Context, metav1.ListOptions) (*corev1.PodList, error)
}

func (client certRotationPodClient) List(ctx context.Context, options metav1.ListOptions) (*corev1.PodList, error) {
	return client.list(ctx, options)
}

func rotationPod(revision string, phase corev1.PodPhase, ready bool) corev1.Pod {
	pod := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"revision": revision}}, Status: corev1.PodStatus{Phase: phase}}
	if ready {
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
	}
	return pod
}

func TestCertRotationPodRevision(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		pods      []corev1.Pod
		ready     bool
		wantError bool
	}{
		{name: "no pods"},
		{name: "single replica", pods: []corev1.Pod{rotationPod("1", corev1.PodRunning, true)}, ready: true},
		{name: "three replicas", pods: []corev1.Pod{rotationPod("1", corev1.PodRunning, true), rotationPod("1", corev1.PodRunning, true), rotationPod("1", corev1.PodRunning, true)}, ready: true},
		{name: "not ready", pods: []corev1.Pod{rotationPod("1", corev1.PodRunning, false)}},
		{name: "pending", pods: []corev1.Pod{rotationPod("1", corev1.PodPending, false)}},
		{name: "different revisions", pods: []corev1.Pod{rotationPod("1", corev1.PodRunning, true), rotationPod("2", corev1.PodRunning, true)}},
		{name: "old failed revision", pods: []corev1.Pod{rotationPod("1", corev1.PodFailed, false), rotationPod("2", corev1.PodRunning, true)}, ready: true},
		{name: "current failed revision", pods: []corev1.Pod{rotationPod("1", corev1.PodFailed, false), rotationPod("1", corev1.PodRunning, true)}, wantError: true},
		{name: "unknown phase", pods: []corev1.Pod{rotationPod("1", corev1.PodUnknown, false)}, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, ready, err := certRotationPodRevision(test.pods)
			if ready != test.ready || (err != nil) != test.wantError {
				t.Fatalf("ready=%v err=%v, want ready=%v error=%v", ready, err, test.ready, test.wantError)
			}
		})
	}
}

func TestCertRotationPodsRetryAndResetStability(t *testing.T) {
	t.Parallel()
	for _, reset := range []string{"stable", "list timeout", "unready", "revision change"} {
		t.Run(reset, func(t *testing.T) {
			t.Parallel()
			now := time.Unix(0, 0)
			calls := 0
			client := certRotationPodClient{list: func(ctx context.Context, options metav1.ListOptions) (*corev1.PodList, error) {
				calls++
				if options.LabelSelector != "app=etcd" {
					t.Fatalf("selector=%q", options.LabelSelector)
				}
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > 30*time.Second {
					t.Fatal("list request has no bounded deadline")
				}
				revision := "1"
				if reset == "revision change" && calls >= 3 {
					revision = "2"
				}
				if calls == 3 && reset == "list timeout" {
					return nil, context.DeadlineExceeded
				}
				return &corev1.PodList{Items: []corev1.Pod{rotationPod(revision, corev1.PodRunning, calls != 3 || reset != "unready")}}, nil
			}}
			condition := certRotationPodsStable(t, client, "app=etcd", func() time.Time { return now })
			for observation := 1; observation <= 6; observation++ {
				ready, err := condition(context.Background())
				if ready || err != nil || calls != observation-1 {
					t.Fatalf("early observation: ready=%v err=%v calls=%d", ready, err, calls)
				}
				now = now.Add(time.Minute)
				ready, err = condition(context.Background())
				wantReady := observation == 6 || reset == "revision change" && observation == 5 || reset == "stable" && observation == 3
				if ready != wantReady || err != nil {
					t.Fatalf("observation=%d ready=%v err=%v, want ready=%v", observation, ready, err, wantReady)
				}
				if ready {
					break
				}
			}
		})
	}
}

func TestCertRotationPodsDeadline(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer cancel()
	now := time.Unix(0, 0)
	client := certRotationPodClient{list: func(requestCtx context.Context, _ metav1.ListOptions) (*corev1.PodList, error) {
		<-requestCtx.Done()
		return nil, requestCtx.Err()
	}}
	condition := certRotationPodsStable(t, client, "app=etcd", func() time.Time { return now })
	now = now.Add(time.Minute)
	if _, err := condition(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("request did not inherit the deadline: %v", err)
	}
	if err := waitForCertRotationPods(ctx, t, client, "app=etcd"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("poll did not honor deadline: %v", err)
	}
}

func TestCertRotationPodsCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	now := time.Unix(0, 0)
	client := certRotationPodClient{list: func(requestCtx context.Context, _ metav1.ListOptions) (*corev1.PodList, error) {
		cancel()
		<-requestCtx.Done()
		return nil, requestCtx.Err()
	}}
	condition := certRotationPodsStable(t, client, "app=etcd", func() time.Time { return now })
	now = now.Add(time.Minute)
	if _, err := condition(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("in-flight request did not honor cancellation: %v", err)
	}
	if err := waitForCertRotationPods(ctx, t, client, "app=etcd"); !errors.Is(err, context.Canceled) {
		t.Fatalf("poll did not honor cancellation: %v", err)
	}
}
