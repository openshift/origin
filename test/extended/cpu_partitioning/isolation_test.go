package cpu_partitioning

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	o "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestShouldCheckPlatformWorkload(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name          string
		namespace     string
		allowed       map[string]string
		annotated     bool
		lookupError   error
		wantCheck     bool
		wantError     bool
		unpartitioned bool
	}{
		{name: "negative workload remains opted out", namespace: "workload-non-pinning", annotated: true},
		{name: "eligible test workload", namespace: "workload-pinning", allowed: namespaceAnnotation, annotated: true, wantCheck: true},
		{name: "eligible platform workload", namespace: "openshift-platform", allowed: namespaceAnnotation, annotated: true, wantCheck: true},
		{name: "platform namespace missing annotation", namespace: "openshift-platform", annotated: true, wantError: true},
		{name: "platform namespace allows different workload", namespace: "openshift-platform", allowed: map[string]string{namespaceAnnotationKey: "other"}, annotated: true, wantError: true},
		{name: "platform template missing annotation", namespace: "openshift-platform", allowed: namespaceAnnotation, wantError: true},
		{name: "ordinary template missing annotation", namespace: "application", allowed: namespaceAnnotation},
		{name: "e2e namespace is not a platform namespace", namespace: "openshift-e2e-workload", annotated: true},
		{name: "debug namespace without annotations", namespace: "openshift-debug-workload"},
		{name: "debug namespace remains opted out", namespace: "openshift-debug-workload", annotated: true},
		{name: "eligible debug workload", namespace: "openshift-debug-workload", allowed: namespaceAnnotation, annotated: true, wantCheck: true},
		{name: "eligible debug namespace without template annotation", namespace: "openshift-debug-workload", allowed: namespaceAnnotation},
		{name: "debug lookalike remains platform", namespace: "openshift-debugging-workload", annotated: true, wantError: true},
		{name: "debug namespace API failure", namespace: "openshift-debug-workload", annotated: true, lookupError: apierrors.NewForbidden(schema.GroupResource{Resource: "namespaces"}, "openshift-debug-workload", errors.New("denied")), wantError: true},
		{name: "multiple allowed workload types", namespace: "application", allowed: map[string]string{namespaceAnnotationKey: "other,management"}, annotated: true, wantCheck: true},
		{name: "annotation value is not a substring match", namespace: "application", allowed: map[string]string{namespaceAnnotationKey: "not-management"}, annotated: true},
		{name: "namespace lookup forbidden", namespace: "application", annotated: true, lookupError: apierrors.NewForbidden(schema.GroupResource{Resource: "namespaces"}, "application", errors.New("denied")), wantError: true},
		{name: "ordinary namespace lookup missing", namespace: "application", annotated: true, lookupError: apierrors.NewNotFound(schema.GroupResource{Resource: "namespaces"}, "application")},
		{name: "platform namespace lookup missing", namespace: "openshift-platform", annotated: true, lookupError: apierrors.NewNotFound(schema.GroupResource{Resource: "namespaces"}, "openshift-platform"), wantError: true},
		{name: "namespace lookup server error", namespace: "application", annotated: true, lookupError: apierrors.NewInternalError(errors.New("server unavailable")), wantError: true},
		{name: "platform allows multiple workload types", namespace: "openshift-platform", allowed: map[string]string{namespaceAnnotationKey: "other,management"}, annotated: true, wantCheck: true},
		{name: "None platform does not need namespace opt in", namespace: "openshift-platform", annotated: true, unpartitioned: true, wantCheck: true},
		{name: "None platform still needs template annotation", namespace: "openshift-platform", unpartitioned: true, wantError: true},
		{name: "None ordinary annotated workload", namespace: "application", annotated: true, unpartitioned: true, wantCheck: true},
		{name: "None ordinary unannotated workload", namespace: "application", unpartitioned: true},
		{name: "None debug annotated workload", namespace: "openshift-debug-workload", annotated: true, unpartitioned: true, wantCheck: true},
		{name: "None debug unannotated workload", namespace: "openshift-debug-workload", unpartitioned: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			project := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testCase.namespace, Annotations: testCase.allowed}}
			client := fake.NewSimpleClientset(project)
			if testCase.lookupError != nil {
				client.PrependReactor("get", "namespaces", func(clienttesting.Action) (bool, runtime.Object, error) {
					return true, nil, testCase.lookupError
				})
			}
			deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: testCase.namespace}}
			if testCase.annotated {
				deployment.Spec.Template.Annotations = deploymentPodAnnotation
			}
			check, err := shouldCheckPlatformWorkload(context.Background(), client.CoreV1().Namespaces(), deployment.Namespace, deployment.Spec.Template.Annotations, !testCase.unpartitioned)
			if check != testCase.wantCheck || (err != nil) != testCase.wantError {
				t.Fatalf("got check=%t, error=%v; want check=%t, error=%t", check, err, testCase.wantCheck, testCase.wantError)
			}
			if testCase.wantError && testCase.lookupError != nil && !errors.Is(err, testCase.lookupError) {
				t.Fatalf("namespace API error was not preserved: %v", err)
			}
		})
	}
}

func TestPlatformScansCollectErrorsAndVisitEligibleWorkloads(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"deployment", "daemonset"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			client := fake.NewSimpleClientset()
			expectedErrors := []error{
				apierrors.NewForbidden(schema.GroupResource{Resource: "namespaces"}, "openshift-first", errors.New("denied")),
				apierrors.NewInternalError(errors.New("server unavailable")),
			}
			client.PrependReactor("get", "namespaces", func(action clienttesting.Action) (bool, runtime.Object, error) {
				switch action.(clienttesting.GetAction).GetName() {
				case "openshift-first":
					return true, nil, expectedErrors[0]
				case "openshift-second":
					return true, nil, expectedErrors[1]
				default:
					return false, nil, nil
				}
			})
			var deployments []appsv1.Deployment
			var daemonsets []appsv1.DaemonSet
			for _, namespace := range []string{"openshift-first", "openshift-second", "openshift-eligible", "openshift-debug-eligible", "openshift-debug-opted-out"} {
				annotations := namespaceAnnotation
				if namespace == "openshift-debug-opted-out" {
					annotations = nil
				}
				if _, err := client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace, Annotations: annotations}}, metav1.CreateOptions{}); err != nil {
					t.Fatal(err)
				}
				metadata := metav1.ObjectMeta{Name: "workload", Namespace: namespace}
				template := corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Annotations: deploymentPodAnnotation}}
				deployments = append(deployments, appsv1.Deployment{ObjectMeta: metadata, Spec: appsv1.DeploymentSpec{Template: template}})
				daemonsets = append(daemonsets, appsv1.DaemonSet{ObjectMeta: metadata, Spec: appsv1.DaemonSetSpec{Template: template}})
				if _, err := client.CoreV1().Pods(namespace).Create(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: namespace, Annotations: deploymentPodAnnotation}}, metav1.CreateOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			var visited []string
			visit := func(namespace string) {
				pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
				if err != nil || len(pods.Items) != 1 || pods.Items[0].Annotations[workloadAnnotations] == "" {
					t.Fatalf("eligible Pod check failed in %s: pods=%v, error=%v", namespace, pods, err)
				}
				visited = append(visited, namespace)
			}
			var collected []error
			if kind == "deployment" {
				collected = scanPlatformDeployments(ctx, client.CoreV1().Namespaces(), deployments, true, func(workload appsv1.Deployment) { visit(workload.Namespace) })
			} else {
				collected = scanPlatformDaemonSets(ctx, client.CoreV1().Namespaces(), daemonsets, true, func(workload appsv1.DaemonSet) { visit(workload.Namespace) })
			}
			if len(collected) != 2 {
				t.Fatalf("not all eligibility errors collected: %v", collected)
			}
			for index, namespace := range []string{"openshift-first", "openshift-second"} {
				if !errors.Is(collected[index], expectedErrors[index]) || !strings.Contains(collected[index].Error(), kind+" "+namespace+"/workload") {
					t.Fatalf("lost workload context or API error identity: %v", collected[index])
				}
			}
			if len(visited) != 2 || visited[0] != "openshift-eligible" || visited[1] != "openshift-debug-eligible" {
				t.Fatalf("later eligible Pod checks were not preserved: %v", visited)
			}
		})
	}
}

func TestWorkloadNamespaceDisappearsAfterList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, namespace := range []string{"workload-non-pinning", "openshift-e2e-workload", "openshift-debug-workload", "openshift-platform"} {
		t.Run(namespace, func(t *testing.T) {
			client := fake.NewSimpleClientset(
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}},
				&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "workload", Namespace: namespace}, Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Annotations: deploymentPodAnnotation}}}},
			)
			deployments, err := client.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
			if err != nil || len(deployments.Items) != 1 {
				t.Fatalf("listing workload: %v, count=%d", err, len(deployments.Items))
			}
			if err := client.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{}); err != nil {
				t.Fatal(err)
			}
			deployment := deployments.Items[0]
			check, err := shouldCheckPlatformWorkload(ctx, client.CoreV1().Namespaces(), deployment.Namespace, deployment.Spec.Template.Annotations, true)
			if namespace == "openshift-platform" {
				if !apierrors.IsNotFound(err) {
					t.Fatalf("platform namespace disappearance must remain an error: %v", err)
				}
			} else if err != nil || check {
				t.Fatalf("ordinary workload selected after namespace cleanup: check=%t, error=%v", check, err)
			}
		})
	}
}

func TestPlatformPodAnnotationsRemainRequired(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name          string
		annotations   map[string]string
		wantMatch     bool
		unpartitioned bool
	}{
		{name: "complete annotations", annotations: map[string]string{workloadAnnotations: deploymentPodAnnotation[workloadAnnotations], workloadAnnotationPrefix + "/busy-work": `{}`}, wantMatch: true},
		{name: "management annotation missing", annotations: map[string]string{workloadAnnotationPrefix + "/busy-work": `{}`}},
		{name: "resource annotation missing", annotations: deploymentPodAnnotation},
		{name: "warning is not a management annotation", annotations: map[string]string{"workload.openshift.io/warning": "namespace is not annotated with workload.openshift.io/allowed"}},
		{name: "None no management annotations", unpartitioned: true, wantMatch: true},
		{name: "None rejects complete management annotations", unpartitioned: true, annotations: map[string]string{workloadAnnotations: deploymentPodAnnotation[workloadAnnotations], workloadAnnotationPrefix + "/busy-work": `{}`}},
		{name: "None preserves existing partial annotation matcher", unpartitioned: true, annotations: deploymentPodAnnotation, wantMatch: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			matcher, _ := adjustMatcherAndMessageForCluster(!testCase.unpartitioned, o.And(o.HaveKey(workloadAnnotations), o.HaveKey(o.MatchRegexp(workloadAnnotationsRegex))))
			matched, err := matcher.Match(testCase.annotations)
			if err != nil || matched != testCase.wantMatch {
				t.Fatalf("annotation match=%t, error=%v; want %t", matched, err, testCase.wantMatch)
			}
		})
	}
}

func TestDeleteNamespaceAndWait(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name        string
		absent      bool
		deleteError error
		getError    error
		pending     bool
		delayed     bool
		deadline    bool
		wantError   bool
	}{
		{name: "deleted namespace disappears"},
		{name: "already deleted", absent: true},
		{name: "terminating namespace eventually disappears", delayed: true},
		{name: "delete forbidden", deleteError: apierrors.NewForbidden(schema.GroupResource{Resource: "namespaces"}, "workload", errors.New("denied")), wantError: true},
		{name: "read error after delete", getError: apierrors.NewInternalError(errors.New("unavailable")), wantError: true},
		{name: "accepted deletion is not completion", pending: true, wantError: true},
		{name: "pending deletion respects deadline", pending: true, deadline: true, wantError: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			client := fake.NewSimpleClientset()
			project := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "workload"}}
			if !testCase.absent {
				if err := client.Tracker().Add(project); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if testCase.deadline {
				deadlineCtx, deadlineCancel := context.WithTimeout(ctx, 20*time.Millisecond)
				defer deadlineCancel()
				ctx = deadlineCtx
			}
			if testCase.deleteError != nil || testCase.pending || testCase.delayed {
				client.PrependReactor("delete", "namespaces", func(clienttesting.Action) (bool, runtime.Object, error) {
					return true, nil, testCase.deleteError
				})
			}
			getCalls := 0
			if testCase.getError != nil || testCase.pending || testCase.delayed {
				client.PrependReactor("get", "namespaces", func(clienttesting.Action) (bool, runtime.Object, error) {
					getCalls++
					if testCase.delayed {
						if getCalls > 1 {
							return false, nil, nil
						}
						project.DeletionTimestamp = &metav1.Time{Time: time.Now()}
						if err := client.Tracker().Delete(corev1.SchemeGroupVersion.WithResource("namespaces"), "", project.Name); err != nil {
							return true, nil, err
						}
						return true, project, nil
					}
					if testCase.pending {
						if !testCase.deadline {
							cancel()
						}
						return true, project, nil
					}
					return true, nil, testCase.getError
				})
			}
			err := deleteNamespaceAndWait(ctx, client.CoreV1().Namespaces(), project.Name)
			if (err != nil) != testCase.wantError {
				t.Fatalf("got error=%v; want error=%t", err, testCase.wantError)
			}
			if testCase.pending && !testCase.deadline && !errors.Is(err, context.Canceled) {
				t.Fatalf("cleanup did not wait until cancellation: %v", err)
			}
			if testCase.deadline && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("cleanup did not respect the deadline: %v", err)
			}
			if testCase.delayed && getCalls < 2 {
				t.Fatalf("cleanup returned before observing namespace disappearance: %d reads", getCalls)
			}
			for _, expectedError := range []error{testCase.deleteError, testCase.getError} {
				if expectedError != nil && !errors.Is(err, expectedError) {
					t.Fatalf("cleanup API error was not preserved: %v", err)
				}
			}
		})
	}
}
