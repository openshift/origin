package project

import (
	"context"
	"fmt"
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
				o.Expect(ok).To(o.Equal(skipProject))

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
