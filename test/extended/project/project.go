package project

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/davecgh/go-spew/spew"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/openshift/api/annotations"
	authorizationv1 "github.com/openshift/api/authorization/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	projectv1 "github.com/openshift/api/project/v1"
	"github.com/openshift/apiserver-library-go/pkg/authorization/scope"
	projectv1client "github.com/openshift/client-go/project/clientset/versioned/typed/project/v1"
	"github.com/openshift/origin/test/extended/authorization"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-auth][Feature:ProjectAPI] ", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("project-api")
	ctx := context.Background()

	g.Describe("TestProjectIsNamespace", func() {
		g.It(fmt.Sprintf("should succeed [apigroup:project.openshift.io]"), func() {
			t := g.GinkgoT()

			// create a namespace
			namespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: "integration-test-" + oc.Namespace()},
			}
			namespaceResult, err := oc.AdminKubeClient().CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			oc.AddResourceToDelete(corev1.SchemeGroupVersion.WithResource("namespaces"), namespaceResult)

			// now try to get the project with the same name and ensure it is our namespace
			project, err := oc.AdminProjectClient().ProjectV1().Projects().Get(ctx, namespaceResult.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if project.Name != namespace.Name {
				t.Fatalf("Project name did not match namespace name, project %v, namespace %v", project.Name, namespace.Name)
			}

			// now create a project
			project = &projectv1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "new-project-" + oc.Namespace(),
					Annotations: map[string]string{
						annotations.OpenShiftDisplayName: "Hello World",
						projectv1.ProjectNodeSelector:    "env=test",
					},
				},
			}
			projectResult, err := oc.AdminProjectClient().ProjectV1().Projects().Create(ctx, project, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			oc.AddResourceToDelete(projectv1.GroupVersion.WithResource("projects"), projectResult)

			// now get the namespace for that project
			namespace, err = oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, projectResult.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if project.Name != namespace.Name {
				t.Fatalf("Project name did not match namespace name, project %v, namespace %v", project.Name, namespace.Name)
			}
			if project.Annotations[annotations.OpenShiftDisplayName] != namespace.Annotations[annotations.OpenShiftDisplayName] {
				t.Fatalf("Project display name did not match namespace annotation, project %v, namespace %v", project.Annotations[annotations.OpenShiftDisplayName], namespace.Annotations[annotations.OpenShiftDisplayName])
			}
			if project.Annotations[projectv1.ProjectNodeSelector] != namespace.Annotations[projectv1.ProjectNodeSelector] {
				t.Fatalf("Project node selector did not match namespace node selector, project %v, namespace %v", project.Annotations[projectv1.ProjectNodeSelector], namespace.Annotations[projectv1.ProjectNodeSelector])
			}
		})
	})
})

var _ = g.Describe("[sig-auth][Feature:ProjectAPI] ", func() {
	ctx := context.Background()

	defer g.GinkgoRecover()
	oc := exutil.NewCLI("project-api")

	g.Describe("TestProjectWatch", func() {
		g.It(fmt.Sprintf("should succeed [apigroup:project.openshift.io][apigroup:authorization.openshift.io][apigroup:user.openshift.io]"), func() {
			bobName := oc.CreateUser("bob-").Name
			bobConfig := oc.GetClientConfigForUser(bobName)
			bobProjectClient := projectv1client.NewForConfigOrDie(bobConfig)
			w, err := bobProjectClient.Projects().Watch(ctx, metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			ns01Name := oc.SetupProject()
			authorization.AddUserAdminToProject(oc, ns01Name, bobName)
			waitForAdd(ns01Name, w)

			// TEST FOR ADD/REMOVE ACCESS
			joeName := oc.CreateUser("joe-").Name
			ns02Name := oc.SetupProject()
			authorization.AddUserAdminToProject(oc, ns02Name, joeName)
			bobEditName := authorization.AddUserEditToProject(oc, ns02Name, bobName)
			waitForAdd(ns02Name, w)

			err = oc.AdminAuthorizationClient().AuthorizationV1().RoleBindings(ns02Name).Delete(ctx, bobEditName, metav1.DeleteOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			// this is okay: a user gets an artificial delete event when it loses access to the project
			// see: https://github.com/openshift/openshift-apiserver/blob/6159c04cbc1b3590f872c78eda3cd14bd6b1e87e/pkg/project/auth/watch.go#L139
			waitForDelete(ns02Name, w)

			// TEST FOR DELETE PROJECT
			ns03Name := oc.SetupProject()
			authorization.AddUserAdminToProject(oc, ns03Name, bobName)
			waitForAdd(ns03Name, w)

			bobProjectClient.Projects().Delete(ctx, ns03Name, metav1.DeleteOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			// wait for the delete
			waitForDelete(ns03Name, w)

			// test the "start from beginning watch"
			beginningWatch, err := bobProjectClient.Projects().Watch(ctx, metav1.ListOptions{ResourceVersion: "0"})
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForAdd(ns01Name, beginningWatch)

			// Background: in HA we have no guarantee that watch caches are synchronized and this test already broke on Azure.
			// Ref: https://bugzilla.redhat.com/show_bug.cgi?id=1744105
			time.Sleep(5 * time.Second)
			fromNowWatch, err := bobProjectClient.Projects().Watch(ctx, metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			select {
			case event := <-fromNowWatch.ResultChan():
				g.Fail(fmt.Sprintf("unexpected event %s %#v", event.Type, event.Object))

			case <-time.After(2 * time.Second):
			}
		})
	})
})

var _ = g.Describe("[sig-auth][Feature:ProjectAPI] ", func() {
	ctx := context.Background()

	defer g.GinkgoRecover()
	oc := exutil.NewCLI("project-api")

	g.Describe("TestProjectWatchWithSelectionPredicate", func() {
		g.It(fmt.Sprintf("should succeed [apigroup:project.openshift.io][apigroup:authorization.openshift.io][apigroup:user.openshift.io]"), func() {
			bobName := oc.CreateUser("bob-").Name
			bobConfig := oc.GetClientConfigForUser(bobName)
			bobProjectClient := projectv1client.NewForConfigOrDie(bobConfig)

			ns01Name := oc.SetupProject()
			w, err := bobProjectClient.Projects().Watch(ctx, metav1.ListOptions{
				FieldSelector: "metadata.name=" + ns01Name,
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			authorization.AddUserAdminToProject(oc, ns01Name, bobName)
			// we should be seeing an "ADD" watch event being emitted, since we are specifically watching this project via a field selector
			waitForAdd(ns01Name, w)

			ns03Name := oc.SetupProject()
			authorization.AddUserAdminToProject(oc, ns03Name, bobName)
			// we are only watching ns-01, we should not receive events for other projects
			waitForNoEvent(w, ns01Name)

			bobProjectClient.Projects().Delete(ctx, ns03Name, metav1.DeleteOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			// we are only watching ns-01, we should not receive events for other projects
			waitForNoEvent(w, ns01Name)

			// test the "start from beginning watch"
			beginningWatch, err := bobProjectClient.Projects().Watch(ctx, metav1.ListOptions{
				ResourceVersion: "0",
				FieldSelector:   "metadata.name=" + ns01Name,
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			// we should be seeing an "ADD" watch event being emitted, since we are specifically watching this project via a field selector
			waitForAdd(ns01Name, beginningWatch)

			fromNowWatch, err := bobProjectClient.Projects().Watch(ctx, metav1.ListOptions{
				FieldSelector: "metadata.name=" + ns01Name,
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			// since we are only watching for events from "ns-01", and no projects are being modified, we should not receive any events here
			waitForNoEvent(fromNowWatch, ns01Name)
		})
	})
})

// waitForNoEvent ensures no stray events come in.  skipProject allows modify events only for the named project
func waitForNoEvent(w watch.Interface, skipProject string) {
	g.By("waitForNoEvent skipping "+skipProject, func() {
		for {
			select {
			case event := <-w.ResultChan():
				o.Expect(event.Type).To(o.Equal(watch.Modified))
				project, ok := event.Object.(*projectv1.Project)
				o.Expect(ok).To(o.BeTrue())
				framework.Logf("got %#v %#v", event, project)
				o.Expect(project.Name).To(o.Equal(skipProject))

				continue
			case <-time.After(2 * time.Second):
				return
			}
		}
	})
}

func waitForDelete(projectName string, w watch.Interface) {
	g.By("waitForDelete "+projectName, func() {
		for {
			select {
			case event := <-w.ResultChan():
				project := event.Object.(*projectv1.Project)
				framework.Logf("got %#v %#v", event, project)
				if event.Type == watch.Deleted && project.Name == projectName {
					return
				}

			case <-time.After(10 * time.Minute):
				g.Fail(fmt.Sprintf("timeout: %v", projectName))
			}
		}
	})

}
func waitForAdd(projectName string, w watch.Interface) {
	g.By("waitForAdd "+projectName, func() {
		for {
			select {
			case event := <-w.ResultChan():
				project := event.Object.(*projectv1.Project)
				framework.Logf("got %#v %#v", event, project)
				if event.Type == watch.Added && project.Name == projectName {
					return
				}

			case <-time.After(30 * time.Second):
				g.Fail(fmt.Sprintf("timeout: %v", projectName))
			}
		}
	})

}

func waitForOnlyAdd(projectName string, w watch.Interface) {
	g.By("waitForOnlyAdd "+projectName, func() {
		for {
			select {
			case event := <-w.ResultChan():
				project := event.Object.(*projectv1.Project)
				framework.Logf("got %#v %#v", event, project)
				if project.Name == projectName {
					// the first event we see for the expected project must be an ADD
					if event.Type == watch.Added {
						return
					}
					g.Fail(fmt.Sprintf("got unexpected project ADD waiting for %s: %v", project.Name, event))
				}
				if event.Type == watch.Modified {
					// ignore modifications from other projects
					continue
				}
				g.Fail(fmt.Sprintf("got unexpected project %v", project.Name))

			case <-time.After(30 * time.Second):
				g.Fail(fmt.Sprintf("timeout: %v", projectName))
			}
		}
	})
}
func waitForOnlyDelete(projectName string, w watch.Interface) {
	g.By("waitForOnlyDelete "+projectName, func() {
		hasTerminated := sets.NewString()
		for {
			select {
			case event, ok := <-w.ResultChan():
				if !ok {
					g.Fail("watch was closed")
				}

				project, isProject := event.Object.(*projectv1.Project)
				if !isProject {
					framework.Logf("got a not-project %v %v", event.Type, spew.Sdump(event.Object))
					continue
				}
				framework.Logf("got %#v %#v", event, project)

				if project.Name == projectName {
					if event.Type == watch.Deleted {
						return
					}
					// if its an event indicating Terminated status, don't fail, but keep waiting
					if event.Type == watch.Modified {
						terminating := project.Status.Phase == corev1.NamespaceTerminating
						if !terminating && hasTerminated.Has(project.Name) {
							g.Fail(fmt.Sprintf("project %s was terminating, but then got an event where it was not terminating: %#v", project.Name, project))
						}
						if terminating {
							hasTerminated.Insert(project.Name)
						}
						continue
					}
				}
				if event.Type == watch.Modified {
					// ignore modifications for other projects
					continue
				}
				g.Fail(fmt.Sprintf("got unexpected project %v", project.Name))

			case <-time.After(10 * time.Minute): // namespace deletions can take a while during busy e2e runs
				g.Fail(fmt.Sprintf("timeout: %v", projectName))
			}
		}
	})
}

var _ = g.Describe("[sig-auth][Feature:ProjectAPI] ", func() {
	ctx := context.Background()

	defer g.GinkgoRecover()
	oc := exutil.NewCLI("project-api")

	g.Describe("TestScopedProjectAccess", func() {
		g.It(fmt.Sprintf("should succeed [apigroup:user.openshift.io][apigroup:project.openshift.io][apigroup:authorization.openshift.io]"), func() {
			t := g.GinkgoT()

			bobName := oc.CreateUser("bob-").Name
			fullBobConfig := oc.GetClientConfigForUser(bobName)
			fullBobClient := projectv1client.NewForConfigOrDie(fullBobConfig)

			oneName := oc.SetupProject()
			twoName := oc.SetupProject()
			threeName := oc.SetupProject()
			fourName := oc.SetupProject()

			oneTwoBobConfig, err := GetScopedClientForUser(oc, bobName, []string{
				scope.UserListScopedProjects,
				scope.ClusterRoleIndicator + "view:" + oneName,
				scope.ClusterRoleIndicator + "view:" + twoName,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			oneTwoBobClient := projectv1client.NewForConfigOrDie(oneTwoBobConfig)

			twoThreeBobConfig, err := GetScopedClientForUser(oc, bobName, []string{
				scope.UserListScopedProjects,
				scope.ClusterRoleIndicator + "view:" + twoName,
				scope.ClusterRoleIndicator + "view:" + threeName,
			})
			twoThreeBobClient := projectv1client.NewForConfigOrDie(twoThreeBobConfig)

			allBobConfig, err := GetScopedClientForUser(oc, bobName, []string{
				scope.UserListScopedProjects,
				scope.ClusterRoleIndicator + "view:*",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			allBobClient := projectv1client.NewForConfigOrDie(allBobConfig)

			oneTwoWatch, err := oneTwoBobClient.Projects().Watch(ctx, metav1.ListOptions{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			twoThreeWatch, err := twoThreeBobClient.Projects().Watch(ctx, metav1.ListOptions{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			allWatch, err := allBobClient.Projects().Watch(ctx, metav1.ListOptions{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			authorization.AddUserAdminToProject(oc, oneName, bobName)
			t.Logf("test 1")
			waitForOnlyAdd(oneName, allWatch)
			waitForOnlyAdd(oneName, oneTwoWatch)

			authorization.AddUserAdminToProject(oc, twoName, bobName)
			t.Logf("test 2")
			waitForOnlyAdd(twoName, allWatch)
			waitForOnlyAdd(twoName, oneTwoWatch)
			waitForOnlyAdd(twoName, twoThreeWatch)

			authorization.AddUserAdminToProject(oc, threeName, bobName)
			t.Logf("test 3")
			waitForOnlyAdd(threeName, allWatch)
			waitForOnlyAdd(threeName, twoThreeWatch)

			authorization.AddUserAdminToProject(oc, fourName, bobName)
			waitForOnlyAdd(fourName, allWatch)

			if err := hasExactlyTheseProjects(oneTwoBobClient.Projects(), sets.NewString(oneName, twoName)); err != nil {
				t.Error(err)
			}

			if err := hasExactlyTheseProjects(twoThreeBobClient.Projects(), sets.NewString(twoName, threeName)); err != nil {
				t.Error(err)
			}

			if err := hasExactlyTheseProjects(allBobClient.Projects(), sets.NewString(oneName, twoName, threeName, fourName)); err != nil {
				t.Error(err)
			}

			delOptions := metav1.DeleteOptions{}

			if err := fullBobClient.Projects().Delete(ctx, fourName, delOptions); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			waitForOnlyDelete(fourName, allWatch)

			if err := fullBobClient.Projects().Delete(ctx, threeName, delOptions); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			waitForOnlyDelete(threeName, allWatch)
			waitForOnlyDelete(threeName, twoThreeWatch)

			if err := fullBobClient.Projects().Delete(ctx, twoName, delOptions); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			waitForOnlyDelete(twoName, allWatch)
			waitForOnlyDelete(twoName, oneTwoWatch)
			waitForOnlyDelete(twoName, twoThreeWatch)

			if err := fullBobClient.Projects().Delete(ctx, oneName, delOptions); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			waitForOnlyDelete(oneName, allWatch)
			waitForOnlyDelete(oneName, oneTwoWatch)
		})
	})
})

var _ = g.Describe("[sig-auth][Feature:ProjectAPI] ", func() {
	ctx := context.Background()

	defer g.GinkgoRecover()
	oc := exutil.NewCLI("project-api")

	g.Describe("TestInvalidRoleRefs", func() {
		g.It(fmt.Sprintf("should succeed [apigroup:authorization.openshift.io][apigroup:user.openshift.io][apigroup:project.openshift.io]"), func() {
			clusterAdminRbacClient := oc.AdminKubeClient().RbacV1()
			clusterAdminAuthorizationClient := oc.AdminAuthorizationClient().AuthorizationV1()

			bobName := oc.CreateUser("bob-").Name
			bobConfig := oc.GetClientConfigForUser(bobName)

			aliceName := oc.CreateUser("alice-").Name
			aliceConfig := oc.GetClientConfigForUser(aliceName)

			fooName := oc.SetupProject()
			authorization.AddUserAdminToProject(oc, fooName, bobName)
			barName := oc.SetupProject()
			authorization.AddUserAdminToProject(oc, barName, aliceName)

			roleBinding := &rbacv1.RoleBinding{}
			roleBinding.GenerateName = "missing-role-"
			roleBinding.RoleRef.Kind = "ClusterRole"
			roleBinding.RoleRef.Name = "missing-role-" + oc.Namespace()

			// mess up rolebindings in "foo"
			_, err := clusterAdminRbacClient.RoleBindings(fooName).Create(ctx, roleBinding, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			// mess up rolebindings in "bar"
			_, err = clusterAdminRbacClient.RoleBindings(barName).Create(ctx, roleBinding, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			// mess up clusterrolebindings
			clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
			clusterRoleBinding.GenerateName = "missing-role-"
			clusterRoleBinding.RoleRef.Kind = "ClusterRole"
			clusterRoleBinding.RoleRef.Name = "missing-role-" + oc.Namespace()
			actual, err := clusterAdminRbacClient.ClusterRoleBindings().Create(ctx, clusterRoleBinding, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			oc.AddResourceToDelete(rbacv1.SchemeGroupVersion.WithResource("clusterrolebindings"), actual)

			// wait for evaluation errors to show up in both namespaces and at cluster scope
			err = wait.PollImmediate(100*time.Millisecond, 10*time.Second, func() (bool, error) {
				// do this 10 times to be sure that all API server instances have converged
				for i := 0; i < 10; i++ {
					review := &authorizationv1.ResourceAccessReview{Action: authorizationv1.Action{Verb: "get", Resource: "pods"}}
					review.Action.Namespace = fooName
					if resp, err := clusterAdminAuthorizationClient.ResourceAccessReviews().Create(ctx, review, metav1.CreateOptions{}); err != nil || resp.EvaluationError == "" {
						return false, err
					}
					review.Action.Namespace = barName
					if resp, err := clusterAdminAuthorizationClient.ResourceAccessReviews().Create(ctx, review, metav1.CreateOptions{}); err != nil || resp.EvaluationError == "" {
						return false, err
					}
					review.Action.Namespace = ""
					if resp, err := clusterAdminAuthorizationClient.ResourceAccessReviews().Create(ctx, review, metav1.CreateOptions{}); err != nil || resp.EvaluationError == "" {
						return false, err
					}
				}
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			// Make sure bob still sees his project (and only his project)
			err = hasExactlyTheseProjects(projectv1client.NewForConfigOrDie(bobConfig).Projects(), sets.NewString(fooName))
			o.Expect(err).NotTo(o.HaveOccurred())

			// Make sure alice still sees her project (and only her project)
			err = hasExactlyTheseProjects(projectv1client.NewForConfigOrDie(aliceConfig).Projects(), sets.NewString(barName))
			o.Expect(err).NotTo(o.HaveOccurred())

			// Make sure cluster admin still sees all projects, we sometimes appear to race, so wait for a second for caches
			time.Sleep(1 * time.Second)
			projects, err := oc.AdminProjectClient().ProjectV1().Projects().List(ctx, metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			projectNames := sets.NewString()
			for _, project := range projects.Items {
				projectNames.Insert(project.Name)
			}
			expected := []string{fooName, barName, "openshift-infra", "openshift", "default"}
			if !projectNames.HasAll(expected...) {
				g.Fail(fmt.Sprintf("Expected projects %v among %v", expected, projectNames.List()))
			}
		})
	})
})

func hasExactlyTheseProjects(lister projectv1client.ProjectInterface, projects sets.String) error {
	var lastErr error
	if err := wait.PollImmediate(100*time.Millisecond, 10*time.Second, func() (bool, error) {
		list, err := lister.List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		if len(list.Items) != len(projects) {
			lastErr = fmt.Errorf("expected %v, got %v", projects.List(), list.Items)
			return false, nil
		}
		for _, project := range list.Items {
			if !projects.Has(project.Name) {
				lastErr = fmt.Errorf("expected %v, got %v", projects.List(), list.Items)
				return false, nil
			}
		}
		return true, nil
	}); err != nil {
		return fmt.Errorf("hasExactlyTheseProjects failed with %v and %v", err, lastErr)
	}
	return nil
}

// cacheRaceTestConfig holds tuning parameters for TestProjectAuthCacheRaceCondition.
//
// These values are calibrated to trigger the concurrent map race (OCPBUGS-57474) in the
// openshift-apiserver authorization cache. The cache does a full rebuild every 15 seconds,
// so 3 rounds × ~30 seconds each spans multiple TTL cycles. 30 concurrent List readers
// (readersPerUser × userCount) plus 10 Watch readers maximize the chance of a List() call
// iterating a subjectRecord.namespaces map while synchronize() mutates it in place.
type cacheRaceTestConfig struct {
	namespaceCount   int
	userCount        int
	readersPerUser   int
	setupConcurrency int
	maxListLatency   time.Duration
	deleteFraction   int
	churnRounds      int
}

var defaultCacheRaceConfig = cacheRaceTestConfig{
	namespaceCount:   1000,
	userCount:        10,
	readersPerUser:   3,
	setupConcurrency: 100,
	maxListLatency:   2 * time.Second,
	deleteFraction:   3,
	churnRounds:      3,
}

type cacheRaceUser struct {
	name   string
	config *rest.Config
}

type cacheRaceReaders struct {
	wg              sync.WaitGroup
	listErrors      atomic.Int64
	watchErrors     atomic.Int64
	maxLatencyNs    atomic.Int64
	regressionCount atomic.Int64
	listCalls       atomic.Int64
	highWaterMarks  []atomic.Int64
	monotonicDone   chan struct{}
	stopCh          chan struct{}
}

type rbRef struct {
	namespace string
	name      string
	user      string
}

func createTestNamespaces(ctx context.Context, oc *exutil.CLI, count, concurrency int) []string {
	names := make([]string, 0, count)
	var mu sync.Mutex
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ns, err := oc.AdminKubeClient().CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{GenerateName: "cache-race-"},
			}, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to create namespace")
			oc.AddResourceToDelete(corev1.SchemeGroupVersion.WithResource("namespaces"), ns)
			mu.Lock()
			names = append(names, ns.Name)
			mu.Unlock()
		}()
	}
	wg.Wait()
	return names
}

func startReaders(ctx context.Context, users []cacheRaceUser, cfg cacheRaceTestConfig) *cacheRaceReaders {
	r := &cacheRaceReaders{
		highWaterMarks: make([]atomic.Int64, len(users)),
		monotonicDone:  make(chan struct{}),
		stopCh:         make(chan struct{}),
	}

	for i, u := range users {
		for range cfg.readersPerUser {
			r.wg.Add(1)
			go func(userIdx int, restCfg *rest.Config) {
				defer r.wg.Done()
				client := projectv1client.NewForConfigOrDie(restCfg)
				for {
					select {
					case <-r.stopCh:
						return
					default:
					}

					start := time.Now()
					projects, err := client.Projects().List(ctx, metav1.ListOptions{})
					elapsed := time.Since(start)
					r.listCalls.Add(1)

					if elapsed.Nanoseconds() > r.maxLatencyNs.Load() {
						r.maxLatencyNs.Store(elapsed.Nanoseconds())
					}
					if err != nil {
						framework.Logf("Projects().List() error: %v", err)
						r.listErrors.Add(1)
						continue
					}

					// during the add-only phase, project count must never decrease
					select {
					case <-r.monotonicDone:
					default:
						count := int64(len(projects.Items))
						for {
							prev := r.highWaterMarks[userIdx].Load()
							if count < prev {
								framework.Logf("REGRESSION: user %d saw %d projects, previously saw %d", userIdx, count, prev)
								r.regressionCount.Add(1)
								break
							}
							if count == prev || r.highWaterMarks[userIdx].CompareAndSwap(prev, count) {
								break
							}
						}
					}
				}
			}(i, u.config)
		}

		r.wg.Add(1)
		go func(restCfg *rest.Config) {
			defer r.wg.Done()
			client := projectv1client.NewForConfigOrDie(restCfg)
			for {
				select {
				case <-r.stopCh:
					return
				default:
				}
				w, err := client.Projects().Watch(ctx, metav1.ListOptions{})
				if err != nil {
					framework.Logf("Projects().Watch() error: %v", err)
					r.watchErrors.Add(1)
					continue
				}
				func() {
					defer w.Stop()
					for {
						select {
						case <-r.stopCh:
							return
						case event, ok := <-w.ResultChan():
							if !ok {
								return
							}
							if event.Type == watch.Error {
								framework.Logf("Watch error event: %#v", event.Object)
								r.watchErrors.Add(1)
							}
						}
					}
				}()
			}
		}(u.config)
	}

	return r
}

// runChurnRounds creates and deletes rolebindings in repeated rounds to generate
// sustained cache invalidation pressure. It returns the set of namespaces where
// the verify user (users[0]) lost access in the final round.
func runChurnRounds(ctx context.Context, oc *exutil.CLI, users []cacheRaceUser, namespaces []string, cfg cacheRaceTestConfig, readers *cacheRaceReaders) sets.Set[string] {
	var setupErrors atomic.Int64
	deletedForVerifyUser := sets.New[string]()

	for round := 0; round < cfg.churnRounds; round++ {
		framework.Logf("=== churn round %d/%d (listCalls so far: %d) ===", round+1, cfg.churnRounds, readers.listCalls.Load())

		// create rolebindings — we build them directly instead of using
		// authorization.AddUserViewToProject because we need the created
		// object's name for the delete phase
		roundRBs := createRoleBindings(ctx, oc, users, namespaces, cfg.setupConcurrency, &setupErrors)
		framework.Logf("round %d create done: %d rolebindings", round+1, len(roundRBs))

		if round == 0 {
			close(readers.monotonicDone)
		}

		// clear per-round tracking — a namespace deleted in a previous round
		// gets a fresh rolebinding in this round, so only the final round's
		// deletions matter for the correctness check
		deletedForVerifyUser = sets.New[string]()
		deleteRoleBindings(ctx, oc, roundRBs, users[0].name, cfg, &deletedForVerifyUser)
		framework.Logf("round %d delete done", round+1)
	}

	framework.Logf("all churn rounds complete (total listCalls: %d, setupErrors: %d)", readers.listCalls.Load(), setupErrors.Load())
	o.Expect(setupErrors.Load()).To(o.BeZero(), fmt.Sprintf("%d rolebinding create errors occurred", setupErrors.Load()))

	return deletedForVerifyUser
}

func createRoleBindings(ctx context.Context, oc *exutil.CLI, users []cacheRaceUser, namespaces []string, concurrency int, errors *atomic.Int64) []rbRef {
	refs := make(chan rbRef, len(users)*len(namespaces))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for _, nsName := range namespaces {
		for _, u := range users {
			wg.Add(1)
			go func(ns, user string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				rb := &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "cache-race-view-",
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: rbacv1.GroupName,
						Kind:     "ClusterRole",
						Name:     "view",
					},
					Subjects: []rbacv1.Subject{
						{Kind: "User", Name: user},
					},
				}
				created, err := oc.AdminKubeClient().RbacV1().RoleBindings(ns).Create(ctx, rb, metav1.CreateOptions{})
				if err != nil {
					errors.Add(1)
					return
				}
				refs <- rbRef{namespace: ns, name: created.Name, user: user}
			}(nsName, u.name)
		}
	}
	wg.Wait()
	close(refs)

	var result []rbRef
	for ref := range refs {
		result = append(result, ref)
	}
	return result
}

func deleteRoleBindings(ctx context.Context, oc *exutil.CLI, rbs []rbRef, verifyUser string, cfg cacheRaceTestConfig, deletedForVerifyUser *sets.Set[string]) {
	sem := make(chan struct{}, cfg.setupConcurrency)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i, ref := range rbs {
		if i%cfg.deleteFraction != 0 {
			continue
		}
		wg.Add(1)
		go func(r rbRef) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := oc.AdminKubeClient().RbacV1().RoleBindings(r.namespace).Delete(ctx, r.name, metav1.DeleteOptions{}); err != nil {
				return
			}
			if r.user == verifyUser {
				mu.Lock()
				deletedForVerifyUser.Insert(r.namespace)
				mu.Unlock()
			}
		}(ref)
	}
	wg.Wait()
}

func assertNoRestarts(ctx context.Context, oc *exutil.CLI, baseline map[string]int32) {
	current := getPodRestartCounts(ctx, oc, "openshift-apiserver")
	restarted := false
	for podName, base := range baseline {
		cur, exists := current[podName]
		if !exists {
			framework.Logf("pod %s no longer exists (may have been rescheduled)", podName)
			continue
		}
		if cur != base {
			framework.Logf("pod %s restarted during test (before=%d, after=%d)", podName, base, cur)
			restarted = true
		}
	}
	if restarted {
		dumpApiserverCrashEvidence(ctx, oc, "openshift-apiserver")
	}
	for podName, base := range baseline {
		cur, exists := current[podName]
		if !exists {
			continue
		}
		o.Expect(cur).To(o.Equal(base), fmt.Sprintf("pod %s restarted during test (before=%d, after=%d)", podName, base, cur))
	}
}

var _ = g.Describe("[sig-auth][Feature:ProjectAPI] ", func() {
	ctx := context.Background()

	defer g.GinkgoRecover()
	oc := exutil.NewCLI("project-api")

	g.Describe("TestProjectAuthCacheRaceCondition", func() {
		g.It("should not crash or block when listing projects under concurrent cache churn [apigroup:project.openshift.io][apigroup:authorization.openshift.io][apigroup:user.openshift.io]", func() {
			cfg := defaultCacheRaceConfig

			g.By(fmt.Sprintf("creating %d test users", cfg.userCount))
			users := make([]cacheRaceUser, 0, cfg.userCount)
			for i := range cfg.userCount {
				userName := oc.CreateUser(fmt.Sprintf("racetest-%d-", i)).Name
				users = append(users, cacheRaceUser{
					name:   userName,
					config: oc.GetClientConfigForUser(userName),
				})
			}

			g.By(fmt.Sprintf("creating %d namespaces", cfg.namespaceCount))
			namespaceNames := createTestNamespaces(ctx, oc, cfg.namespaceCount, cfg.setupConcurrency)

			g.By("recording openshift-apiserver pod restart counts")
			baselineRestarts := getPodRestartCounts(ctx, oc, "openshift-apiserver")

			g.By(fmt.Sprintf("starting %d List readers + %d Watch readers", cfg.readersPerUser*cfg.userCount, cfg.userCount))
			readers := startReaders(ctx, users, cfg)

			g.By(fmt.Sprintf("running %d rounds of create/delete churn across %d users × %d namespaces", cfg.churnRounds, cfg.userCount, cfg.namespaceCount))
			deletedForVerifyUser := runChurnRounds(ctx, oc, users, namespaceNames, cfg, readers)

			close(readers.stopCh)
			readers.wg.Wait()

			g.By("checking openshift-apiserver pods did not restart")
			assertNoRestarts(ctx, oc, baselineRestarts)

			g.By("verifying no List errors occurred")
			o.Expect(readers.listErrors.Load()).To(o.BeZero(), fmt.Sprintf("%d Projects().List() errors occurred", readers.listErrors.Load()))

			g.By("verifying no project list regressions occurred during the add phase")
			framework.Logf("project count regressions detected: %d", readers.regressionCount.Load())
			o.Expect(readers.regressionCount.Load()).To(o.BeZero(), fmt.Sprintf("%d regressions: users saw fewer projects than previously observed", readers.regressionCount.Load()))

			g.By("verifying no List calls were blocked")
			maxLatency := time.Duration(readers.maxLatencyNs.Load())
			framework.Logf("max Projects().List() latency: %s", maxLatency)
			o.Expect(maxLatency < cfg.maxListLatency).To(o.BeTrue(), fmt.Sprintf("Projects().List() latency %s exceeded threshold %s", maxLatency, cfg.maxListLatency))

			g.By("verifying the project list is correct")
			expectedNames := sets.New[string](namespaceNames...).Difference(deletedForVerifyUser)
			framework.Logf("expecting %d projects visible (%d total - %d deleted)", expectedNames.Len(), len(namespaceNames), deletedForVerifyUser.Len())
			verifyClient := projectv1client.NewForConfigOrDie(users[0].config)
			err := wait.PollImmediate(1*time.Second, 2*time.Minute, func() (bool, error) {
				projects, listErr := verifyClient.Projects().List(ctx, metav1.ListOptions{})
				if listErr != nil {
					return false, listErr
				}
				visible := sets.New[string]()
				for _, p := range projects.Items {
					visible.Insert(p.Name)
				}
				return visible.HasAll(expectedNames.UnsortedList()...), nil
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "project list does not contain all expected namespaces after race phase")
		})
	})
})

func getPodRestartCounts(ctx context.Context, oc *exutil.CLI, namespace string) map[string]int32 {
	pods, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to list pods in %s", namespace)
	counts := make(map[string]int32)
	for _, pod := range pods.Items {
		var total int32
		for _, status := range pod.Status.ContainerStatuses {
			total += status.RestartCount
		}
		counts[pod.Name] = total
	}
	return counts
}

func dumpApiserverCrashEvidence(ctx context.Context, oc *exutil.CLI, namespace string) {
	crashSignatures := []string{"fatal error", "concurrent map", "panic:", "runtime error", "goroutine"}

	pods, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		framework.Logf("failed to list pods in %s: %v", namespace, err)
		return
	}

	sinceSeconds := int64(600)
	for _, pod := range pods.Items {
		for _, previous := range []bool{true, false} {
			opts := &corev1.PodLogOptions{
				Container:    "openshift-apiserver",
				Previous:     previous,
				SinceSeconds: &sinceSeconds,
			}
			stream, err := oc.AdminKubeClient().CoreV1().Pods(namespace).GetLogs(pod.Name, opts).Stream(ctx)
			if err != nil {
				continue
			}

			var evidence []string
			scanner := bufio.NewScanner(stream)
			for scanner.Scan() {
				line := scanner.Text()
				for _, sig := range crashSignatures {
					if strings.Contains(strings.ToLower(line), sig) {
						evidence = append(evidence, line)
						break
					}
				}
			}
			stream.Close()

			if len(evidence) > 0 {
				label := "current"
				if previous {
					label = "previous"
				}
				framework.Logf("=== crash evidence from %s (%s container) ===", pod.Name, label)
				for _, line := range evidence {
					framework.Logf("  %s", line)
				}
				framework.Logf("=== end crash evidence ===")
			}
		}
	}
}

func GetScopedClientForUser(oc *exutil.CLI, username string, scopes []string) (*rest.Config, error) {
	// make sure the user exists
	user, err := oc.AdminUserClient().UserV1().Users().Get(context.Background(), username, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	tokenStr, sha256TokenStr := exutil.GenerateOAuthTokenPair()
	token := &oauthv1.OAuthAccessToken{
		ObjectMeta:  metav1.ObjectMeta{Name: sha256TokenStr},
		ClientName:  "openshift-challenging-client",
		ExpiresIn:   86400,
		Scopes:      scopes,
		RedirectURI: "https://127.0.0.1:12000/oauth/token/implicit",
		UserName:    user.Name,
		UserUID:     string(user.UID),
	}
	if _, err := oc.AdminOAuthClient().OauthV1().OAuthAccessTokens().Create(context.Background(), token, metav1.CreateOptions{}); err != nil {
		return nil, err
	}
	oc.AddResourceToDelete(oauthv1.GroupVersion.WithResource("oauthaccesstokens"), token)

	scopedConfig := rest.AnonymousClientConfig(oc.AdminConfig())
	scopedConfig.BearerToken = tokenStr
	return scopedConfig, nil
}
