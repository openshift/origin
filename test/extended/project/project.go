package project

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"
	"github.com/openshift/api/annotations"
	authorizationv1 "github.com/openshift/api/authorization/v1"
	configv1 "github.com/openshift/api/config/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	projectv1 "github.com/openshift/api/project/v1"
	"github.com/openshift/apiserver-library-go/pkg/authorization/scope"
	projectv1client "github.com/openshift/client-go/project/clientset/versioned/typed/project/v1"
	"github.com/openshift/origin/test/extended/authorization"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/test/e2e/framework"
	imageutils "k8s.io/kubernetes/test/utils/image"
	"sigs.k8s.io/yaml"
)

var (
	commonResourceTypes = []string{"deployments", "pods", "services", "configmaps", "secrets"}
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

			oneTwoBobConfig, err := GetScopedClientForUser(ctx, oc, bobName, []string{
				scope.UserListScopedProjects,
				scope.ClusterRoleIndicator + "view:" + oneName,
				scope.ClusterRoleIndicator + "view:" + twoName,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			oneTwoBobClient := projectv1client.NewForConfigOrDie(oneTwoBobConfig)

			twoThreeBobConfig, err := GetScopedClientForUser(ctx, oc, bobName, []string{
				scope.UserListScopedProjects,
				scope.ClusterRoleIndicator + "view:" + twoName,
				scope.ClusterRoleIndicator + "view:" + threeName,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			twoThreeBobClient := projectv1client.NewForConfigOrDie(twoThreeBobConfig)

			allBobConfig, err := GetScopedClientForUser(ctx, oc, bobName, []string{
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

var _ = g.Describe("[sig-auth][Feature:ProjectAPI] ", func() {
	oc := exutil.NewCLIWithoutNamespace("project-api")

	// Test that custom project request templates can automatically apply ResourceQuotas and LimitRanges
	// to newly created projects. This validates that:
	// 1. Projects created BEFORE template is configured don't get the resources
	// 2. Projects created AFTER template is configured automatically get the resources
	// 3. The template configuration propagates through openshift-apiserver properly
	g.It("[Serial][Slow][OTP] should apply a customized project request template with ResourceQuota and LimitRange [apigroup:project.openshift.io][apigroup:config.openshift.io][apigroup:template.openshift.io]", ote.Informing(), func(ctx g.SpecContext) {
		const caseID = "project-request-template"
		suffix := rand.String(5)
		templateName := "project-request-template-request-" + suffix
		dirname, err := os.MkdirTemp("", caseID+"-")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(dirname)

		templateYamlFile := dirname + "/template.yaml"
		projectRequestLimitsQuotaFixture := exutil.FixturePath("testdata", "project", "project-request-limits-quota.yaml")
		project1 := caseID + "-before-" + suffix
		project2 := caseID + "-after-" + suffix

		// Save current cluster project config to restore at the end
		entryProject, err := oc.AdminConfigClient().ConfigV1().Projects().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		restoreProjectSpec := entryProject.Spec.DeepCopy()

		defer cleanupProjectRequestTemplateTest(oc, templateName, project1, project2, *restoreProjectSpec)

		// BEFORE: Create a project before template is configured - should NOT have quota/limits
		_, err = oc.AdminProjectClient().ProjectV1().ProjectRequests().Create(ctx, &projectv1.ProjectRequest{
			ObjectMeta: metav1.ObjectMeta{Name: project1},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		// Verify project1 has no custom template resources
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("limitrange,resourcequota", "-n", project1, "--ignore-not-found").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).NotTo(o.ContainSubstring(project1 + "-quota"))
		o.Expect(output).NotTo(o.ContainSubstring(project1 + "-limits"))

		// Create a custom project template with ResourceQuota and LimitRange
		templateContent, err := oc.AsAdmin().WithoutNamespace().Run("adm").Args("create-bootstrap-project-template", "-o", "yaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Parse the bootstrap template
		var template unstructured.Unstructured
		err = yaml.Unmarshal([]byte(templateContent), &template)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Change template name
		template.SetName(templateName)

		// Load and parse the quota/limit objects to inject
		quotaLimitsYAML, err := os.ReadFile(projectRequestLimitsQuotaFixture)
		o.Expect(err).NotTo(o.HaveOccurred())

		var additionalObjects []interface{}
		err = yaml.Unmarshal(quotaLimitsYAML, &additionalObjects)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Append the quota and limit objects to the template's objects array
		objects, found, err := unstructured.NestedSlice(template.Object, "objects")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(found).To(o.BeTrue())
		objects = append(objects, additionalObjects...)
		err = unstructured.SetNestedSlice(template.Object, objects, "objects")
		o.Expect(err).NotTo(o.HaveOccurred())

		// Write the modified template
		modifiedTemplateYAML, err := yaml.Marshal(template.Object)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = os.WriteFile(templateYamlFile, modifiedTemplateYAML, 0o644)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Upload the template to openshift-config namespace
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", templateYamlFile, "-n", "openshift-config").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Configure cluster to use the custom project request template
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			project, err := oc.AdminConfigClient().ConfigV1().Projects().Get(ctx, "cluster", metav1.GetOptions{})
			if err != nil {
				return err
			}
			project.Spec.ProjectRequestTemplate = configv1.TemplateReference{Name: templateName}
			_, err = oc.AdminConfigClient().ConfigV1().Projects().Update(ctx, project, metav1.UpdateOptions{})
			return err
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		// Wait for the template configuration to propagate to openshift-apiserver's observed config
		err = wait.PollUntilContextTimeout(ctx, 30*time.Second, 4*time.Minute, false, func(pollCtx context.Context) (bool, error) {
			project, err := oc.AdminConfigClient().ConfigV1().Projects().Get(pollCtx, "cluster", metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			if project.Spec.ProjectRequestTemplate.Name != templateName {
				return false, nil
			}

			observedTemplate, err := openshiftAPIServerObservedProjectRequestTemplate(pollCtx, oc)
			if err != nil {
				return false, err
			}
			return strings.Contains(observedTemplate, templateName), nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		// Wait for openshift-apiserver operator to roll out the config change
		waitCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
		// First wait for operator to start progressing (optional - logs warning if it doesn't)
		if err := exutil.WaitForOperatorProgressingTrue(waitCtx, oc.AdminConfigClient(), "openshift-apiserver"); err != nil {
			framework.Logf("warning: failed to wait for openshift-apiserver to start progressing: %v", err)
		}
		// Then wait for operator to become stable (Available=True, Progressing=False, Degraded=False)
		err = wait.PollUntilContextCancel(waitCtx, 30*time.Second, true, func(pollCtx context.Context) (bool, error) {
			co, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(pollCtx, "openshift-apiserver", metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			var available, progressing, degraded bool
			for _, c := range co.Status.Conditions {
				if c.Type == configv1.OperatorAvailable {
					available = c.Status == configv1.ConditionTrue
				} else if c.Type == configv1.OperatorProgressing {
					progressing = c.Status == configv1.ConditionTrue
				} else if c.Type == configv1.OperatorDegraded {
					degraded = c.Status == configv1.ConditionTrue
				}
			}
			if degraded {
				return false, fmt.Errorf("openshift-apiserver operator is degraded")
			}
			return available && !progressing, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		// AFTER: Create a project after template is configured - should automatically get quota/limits
		_, err = oc.AdminProjectClient().ProjectV1().ProjectRequests().Create(ctx, &projectv1.ProjectRequest{
			ObjectMeta: metav1.ObjectMeta{Name: project2},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		// Verify project2 has the custom template resources automatically applied
		o.Eventually(func(gomega o.Gomega) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("limitrange,resourcequota", "-n", project2).Output()
			gomega.Expect(err).NotTo(o.HaveOccurred())
			gomega.Expect(output).To(o.ContainSubstring(project2 + "-limits"))
			gomega.Expect(output).To(o.ContainSubstring(project2 + "-quota"))
		}).WithTimeout(1 * time.Minute).WithPolling(3 * time.Second).Should(o.Succeed())

		// Re-verify project1 still doesn't have the resources (retroactive application doesn't happen)
		output, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("limitrange,resourcequota", "-n", project1, "--ignore-not-found").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).NotTo(o.ContainSubstring(project1 + "-quota"))
		o.Expect(output).NotTo(o.ContainSubstring(project1 + "-limits"))
	})

	// Test that deleting a project cascades and removes all resources within it,
	// and that recreating a project with the same name starts fresh with no leftover resources
	g.It("[Serial][OTP] should delete all resources when the project is deleted [apigroup:project.openshift.io][apigroup:apps.openshift.io]", ote.Informing(), func(ctx g.SpecContext) {
		const caseID = "project-cascading-delete"
		suffix := rand.String(5)
		projectName := caseID + "-" + suffix
		resourcePrefix := caseID + "-" + suffix
		testImage := imageutils.GetE2EImage(imageutils.Agnhost)

		defer func() {
			if err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("project", projectName, "--ignore-not-found").Execute(); err != nil {
				framework.Logf("cleanup: failed to delete project %q: %v", projectName, err)
			}
		}()

		// Create project and populate it with various resource types
		err := oc.AsAdmin().WithoutNamespace().Run("new-project").Args(projectName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.AsAdmin().WithoutNamespace().Run("new-app").Args(
			"--name=hello-openshift", testImage, "-n", projectName, "--import-mode=PreserveOriginal",
		).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", projectName, "configmap", resourcePrefix+"-cm", "--from-literal=key=value").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", projectName, "secret", "generic", resourcePrefix+"-secret", "--from-literal=user=Bob").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Wait for pods to be running (not just created)
		err = wait.PollUntilContextTimeout(ctx, 10*time.Second, 3*time.Minute, false, func(pollCtx context.Context) (bool, error) {
			podOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", projectName, "--no-headers").Output()
			if err != nil {
				return false, err
			}
			if matched, _ := regexp.MatchString(`(ContainerCreating|Init|Pending)`, podOutput); matched {
				return false, nil
			}
			return strings.TrimSpace(podOutput) != "", nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "pods in project %s did not become ready", projectName)

		// Verify all expected resource types exist in the project
		for _, resource := range commonResourceTypes {
			out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(resource, "-n", projectName, "-o=jsonpath={.items[*].metadata.name}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(len(strings.TrimSpace(out))).To(o.BeNumerically(">", 0), "expected %s in project %s", resource, projectName)
		}

		// Delete the project and wait for complete removal
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("project", projectName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Wait for the project namespace to be fully deleted
		err = wait.PollUntilContextTimeout(ctx, 20*time.Second, 5*time.Minute, false, func(pollCtx context.Context) (bool, error) {
			out, getErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("project", projectName).Output()
			if getErr != nil {
				matched, _ := regexp.MatchString("not found", getErr.Error())
				return matched, nil
			}
			matched, _ := regexp.MatchString("namespaces .* not found", out)
			return matched, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "project %s was not fully deleted", projectName)

		// Verify all resources were cascading-deleted with the project
		for _, resource := range commonResourceTypes {
			out, getErr := oc.AsAdmin().WithoutNamespace().Run("get").Args(
				resource, "-n", projectName, "-o=jsonpath={.items[*].metadata.name}", "--ignore-not-found",
			).Output()
			if getErr != nil && strings.Contains(getErr.Error(), "not found") {
				continue
			}
			o.Expect(getErr).NotTo(o.HaveOccurred())
			o.Expect(strings.TrimSpace(out)).To(o.BeEmpty(), "expected no %s remaining after project deletion", resource)
		}

		// Recreate project with same name and verify it's a clean slate
		err = oc.AsAdmin().WithoutNamespace().Run("new-project").Args(projectName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			if err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("project", projectName, "--ignore-not-found").Execute(); err != nil {
				framework.Logf("cleanup: failed to delete project %q: %v", projectName, err)
			}
		}()

		// Verify the new project has no leftover resources from the deleted project
		// Check for specific resources that were created in the first project incarnation
		resourceChecks := map[string][]string{
			"deployment": {"hello-openshift"},
			"service":    {"hello-openshift"},
			"configmap":  {resourcePrefix + "-cm"},
			"secret":     {resourcePrefix + "-secret"},
			"pods":       {"-l", "app=hello-openshift"},
		}

		for resourceType, args := range resourceChecks {
			checkArgs := append([]string{resourceType}, args...)
			checkArgs = append(checkArgs, "-n", projectName, "-o=name", "--ignore-not-found")
			out, getErr := oc.AsAdmin().WithoutNamespace().Run("get").Args(checkArgs...).Output()
			if getErr != nil {
				continue
			}
			o.Expect(strings.TrimSpace(out)).To(o.BeEmpty(), "expected no leftover %s from deleted project, found: %s", resourceType, out)
		}
	})
})

func cleanupProjectRequestTemplateTest(oc *exutil.CLI, templateName, project1, project2 string, restoreSpec configv1.ProjectSpec) {
	// Clean up test projects
	for _, projectName := range []string{project1, project2} {
		if err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("project", projectName, "--ignore-not-found").Execute(); err != nil {
			framework.Logf("cleanup: failed to delete project %q: %v", projectName, err)
		} else {
			framework.Logf("cleanup: deleted project %q", projectName)
		}
	}

	// Restore the original cluster project configuration (before deleting template)
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		project, err := oc.AdminConfigClient().ConfigV1().Projects().Get(context.Background(), "cluster", metav1.GetOptions{})
		if err != nil {
			return err
		}
		project.Spec = restoreSpec
		_, err = oc.AdminConfigClient().ConfigV1().Projects().Update(context.Background(), project, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		framework.Logf("cleanup: failed to restore project.config.openshift.io/cluster: %v", err)
		// Continue with remaining cleanup steps even if restore failed
	}

	// Wait for the template to be cleared from openshift-apiserver observed config
	if err := wait.PollUntilContextTimeout(context.Background(), 30*time.Second, 5*time.Minute, false, func(ctx context.Context) (bool, error) {
		observedTemplate, err := openshiftAPIServerObservedProjectRequestTemplate(ctx, oc)
		if err != nil {
			return false, err
		}
		return !strings.Contains(observedTemplate, templateName), nil
	}); err != nil {
		framework.Logf("cleanup: failed to wait for template to clear from observed config: %v", err)
	}

	// Wait for openshift-apiserver to stabilize after config restoration
	restoreCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := waitForOpenShiftAPIServerOperatorStableWithPolling(restoreCtx, oc); err != nil {
		framework.Logf("cleanup: failed to wait for openshift-apiserver to stabilize: %v", err)
	}

	// Delete the custom template from openshift-config (only after config is restored)
	if err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("templates", templateName, "-n", "openshift-config", "--ignore-not-found").Execute(); err != nil {
		framework.Logf("cleanup: failed to delete template %q from openshift-config: %v", templateName, err)
	} else {
		framework.Logf("cleanup: deleted template %q from openshift-config", templateName)
	}
}

// openshiftAPIServerObservedProjectRequestTemplate extracts the current project request template
// from the openshift-apiserver operator's observed configuration
func openshiftAPIServerObservedProjectRequestTemplate(ctx context.Context, oc *exutil.CLI) (string, error) {
	osapi, err := oc.AdminOperatorClient().OperatorV1().OpenShiftAPIServers().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	if osapi.Spec.ObservedConfig.Raw == nil {
		return "", nil
	}
	var observedConfig map[string]interface{}
	if err := json.Unmarshal(osapi.Spec.ObservedConfig.Raw, &observedConfig); err != nil {
		return "", err
	}
	projectConfig, ok := observedConfig["projectConfig"].(map[string]interface{})
	if !ok {
		return "", nil
	}
	template, _ := projectConfig["projectRequestTemplate"].(string)
	return template, nil
}

// waitForOpenShiftAPIServerOperatorStableWithPolling polls until the openshift-apiserver
// ClusterOperator reports Available=True, Progressing=False, and Degraded=False
func waitForOpenShiftAPIServerOperatorStableWithPolling(ctx context.Context, oc *exutil.CLI) error {
	return wait.PollUntilContextCancel(ctx, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		co, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(ctx, "openshift-apiserver", metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		var available, progressing, degraded bool
		for _, c := range co.Status.Conditions {
			switch c.Type {
			case configv1.OperatorAvailable:
				available = c.Status == configv1.ConditionTrue
			case configv1.OperatorProgressing:
				progressing = c.Status == configv1.ConditionTrue
			case configv1.OperatorDegraded:
				degraded = c.Status == configv1.ConditionTrue
			}
		}

		if degraded {
			return false, fmt.Errorf("openshift-apiserver operator is degraded")
		}
		return available && !progressing, nil
	})
}

func GetScopedClientForUser(ctx context.Context, oc *exutil.CLI, username string, scopes []string) (*rest.Config, error) {
	// make sure the user exists
	user, err := oc.AdminUserClient().UserV1().Users().Get(ctx, username, metav1.GetOptions{})
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
	if _, err := oc.AdminOAuthClient().OauthV1().OAuthAccessTokens().Create(ctx, token, metav1.CreateOptions{}); err != nil {
		return nil, err
	}
	// Delete token directly to avoid logging token hash
	g.DeferCleanup(func(cleanupCtx g.SpecContext) {
		if err := oc.AdminOAuthClient().OauthV1().OAuthAccessTokens().Delete(cleanupCtx, sha256TokenStr, metav1.DeleteOptions{}); err != nil {
			g.Fail("failed to delete scoped OAuth token")
		}
	})

	scopedConfig := rest.AnonymousClientConfig(oc.AdminConfig())
	scopedConfig.BearerToken = tokenStr
	return scopedConfig, nil
}
