package integration

import (
	"fmt"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/authorizer/scope"
	buildapi "github.com/openshift/origin/pkg/build/api"
	policy "github.com/openshift/origin/pkg/cmd/admin/policy"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	projectapi "github.com/openshift/origin/pkg/project/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

// TestProjectIsNamespace verifies that a project is a namespace, and a namespace is a project
func TestProjectIsNamespace(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	originClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	kubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// create a namespace
	namespace := &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{Name: "integration-test"},
	}
	namespaceResult, err := kubeClient.Namespaces().Create(namespace)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// now try to get the project with the same name and ensure it is our namespace
	project, err := originClient.Projects().Get(namespaceResult.Name)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if project.Name != namespace.Name {
		t.Fatalf("Project name did not match namespace name, project %v, namespace %v", project.Name, namespace.Name)
	}

	// now create a project
	project = &projectapi.Project{
		ObjectMeta: kapi.ObjectMeta{
			Name: "new-project",
			Annotations: map[string]string{
				projectapi.ProjectDisplayName: "Hello World",
				"openshift.io/node-selector":  "env=test",
			},
		},
	}
	projectResult, err := originClient.Projects().Create(project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// now get the namespace for that project
	namespace, err = kubeClient.Namespaces().Get(projectResult.Name)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if project.Name != namespace.Name {
		t.Fatalf("Project name did not match namespace name, project %v, namespace %v", project.Name, namespace.Name)
	}
	if project.Annotations[projectapi.ProjectDisplayName] != namespace.Annotations[projectapi.ProjectDisplayName] {
		t.Fatalf("Project display name did not match namespace annotation, project %v, namespace %v", project.Annotations[projectapi.ProjectDisplayName], namespace.Annotations[projectapi.ProjectDisplayName])
	}
	if project.Annotations["openshift.io/node-selector"] != namespace.Annotations["openshift.io/node-selector"] {
		t.Fatalf("Project node selector did not match namespace node selector, project %v, namespace %v", project.Annotations["openshift.io/node-selector"], namespace.Annotations["openshift.io/node-selector"])
	}
}

// TestProjectMustExist verifies that content cannot be added in a project that does not exist
func TestProjectMustExist(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{Name: "pod"},
		Spec: kapi.PodSpec{
			Containers:    []kapi.Container{{Name: "ctr", Image: "image", ImagePullPolicy: "IfNotPresent"}},
			RestartPolicy: kapi.RestartPolicyAlways,
			DNSPolicy:     kapi.DNSClusterFirst,
		},
	}

	_, err = clusterAdminKubeClient.Pods("test").Create(pod)
	if err == nil {
		t.Errorf("Expected an error on creation of a Kubernetes resource because namespace does not exist")
	}

	build := &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "buildid",
			Namespace: "default",
			Labels: map[string]string{
				buildapi.BuildConfigLabel:    "mock-build-config",
				buildapi.BuildRunPolicyLabel: string(buildapi.BuildRunPolicyParallel),
			},
		},
		Spec: buildapi.BuildSpec{
			CommonSpec: buildapi.CommonSpec{
				Source: buildapi.BuildSource{
					Git: &buildapi.GitBuildSource{
						URI: "http://github.com/my/repository",
					},
					ContextDir: "context",
				},
				Strategy: buildapi.BuildStrategy{
					DockerStrategy: &buildapi.DockerBuildStrategy{},
				},
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "repository/data",
					},
				},
			},
		},
		Status: buildapi.BuildStatus{
			Phase: buildapi.BuildPhaseNew,
		},
	}

	_, err = clusterAdminClient.Builds("test").Create(build)
	if err == nil {
		t.Errorf("Expected an error on creation of a Origin resource because namespace does not exist")
	}
}

func TestProjectWatch(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bobClient, _, _, err := testutil.GetClientForUser(*clusterAdminClientConfig, "bob")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	w, err := bobClient.Projects().Watch(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "ns-01", "bob"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	waitForAdd("ns-01", w, t)

	// TEST FOR ADD/REMOVE ACCESS
	joeClient, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "ns-02", "joe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	addBob := &policy.RoleModificationOptions{
		RoleNamespace:       "",
		RoleName:            bootstrappolicy.EditRoleName,
		RoleBindingAccessor: policy.NewLocalRoleBindingAccessor("ns-02", joeClient),
		Users:               []string{"bob"},
	}
	if err := addBob.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	waitForAdd("ns-02", w, t)

	if err := addBob.RemoveRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	waitForDelete("ns-02", w, t)

	// TEST FOR DELETE PROJECT
	if _, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "ns-03", "bob"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	waitForAdd("ns-03", w, t)

	if err := bobClient.Projects().Delete("ns-03"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// wait for the delete
	waitForDelete("ns-03", w, t)

	// test the "start from beginning watch"
	beginningWatch, err := bobClient.Projects().Watch(kapi.ListOptions{ResourceVersion: "0"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	waitForAdd("ns-01", beginningWatch, t)

	fromNowWatch, err := bobClient.Projects().Watch(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	select {
	case event := <-fromNowWatch.ResultChan():
		t.Fatalf("unexpected event %v", event)

	case <-time.After(3 * time.Second):
	}
}

func waitForDelete(projectName string, w watch.Interface, t *testing.T) {
	for {
		select {
		case event := <-w.ResultChan():
			project := event.Object.(*projectapi.Project)
			t.Logf("got %#v %#v", event, project)
			if event.Type == watch.Deleted && project.Name == projectName {
				return
			}

		case <-time.After(30 * time.Second):
			t.Fatalf("timeout: %v", projectName)
		}
	}
}
func waitForAdd(projectName string, w watch.Interface, t *testing.T) {
	for {
		select {
		case event := <-w.ResultChan():
			project := event.Object.(*projectapi.Project)
			t.Logf("got %#v %#v", event, project)
			if event.Type == watch.Added && project.Name == projectName {
				return
			}

		case <-time.After(30 * time.Second):
			t.Fatalf("timeout: %v", projectName)
		}
	}
}
func waitForOnlyAdd(projectName string, w watch.Interface, t *testing.T) {
	select {
	case event := <-w.ResultChan():
		project := event.Object.(*projectapi.Project)
		t.Logf("got %#v %#v", event, project)
		if event.Type == watch.Added && project.Name == projectName {
			return
		}
		t.Errorf("got unexpected project %v", project.Name)

	case <-time.After(30 * time.Second):
		t.Fatalf("timeout: %v", projectName)
	}
}
func waitForOnlyDelete(projectName string, w watch.Interface, t *testing.T) {
	select {
	case event := <-w.ResultChan():
		project := event.Object.(*projectapi.Project)
		t.Logf("got %#v %#v", event, project)
		if event.Type == watch.Deleted && project.Name == projectName {
			return
		}
		t.Errorf("got unexpected project %v", project.Name)

	case <-time.After(30 * time.Second):
		t.Fatalf("timeout: %v", projectName)
	}
}

func TestScopedProjectAccess(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fullBobClient, _, _, err := testutil.GetClientForUser(*clusterAdminClientConfig, "bob")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	oneTwoBobClient, _, _, err := testutil.GetScopedClientForUser(clusterAdminClient, *clusterAdminClientConfig, "bob", []string{
		scope.UserListScopedProjects,
		scope.ClusterRoleIndicator + "view:one",
		scope.ClusterRoleIndicator + "view:two",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	twoThreeBobClient, _, _, err := testutil.GetScopedClientForUser(clusterAdminClient, *clusterAdminClientConfig, "bob", []string{
		scope.UserListScopedProjects,
		scope.ClusterRoleIndicator + "view:two",
		scope.ClusterRoleIndicator + "view:three",
	})

	allBobClient, _, _, err := testutil.GetScopedClientForUser(clusterAdminClient, *clusterAdminClientConfig, "bob", []string{
		scope.UserListScopedProjects,
		scope.ClusterRoleIndicator + "view:*",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	oneTwoWatch, err := oneTwoBobClient.Projects().Watch(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	twoThreeWatch, err := twoThreeBobClient.Projects().Watch(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	allWatch, err := allBobClient.Projects().Watch(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "one", "bob"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	waitForOnlyAdd("one", allWatch, t)
	waitForOnlyAdd("one", oneTwoWatch, t)

	if _, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "two", "bob"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	waitForOnlyAdd("two", allWatch, t)
	waitForOnlyAdd("two", oneTwoWatch, t)
	waitForOnlyAdd("two", twoThreeWatch, t)

	if _, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "three", "bob"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	waitForOnlyAdd("three", allWatch, t)
	waitForOnlyAdd("three", twoThreeWatch, t)

	if _, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "four", "bob"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	waitForOnlyAdd("four", allWatch, t)

	oneTwoProjects, err := oneTwoBobClient.Projects().List(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := hasExactlyTheseProjects(oneTwoProjects, sets.NewString("one", "two")); err != nil {
		t.Error(err)
	}
	twoThreeProjects, err := twoThreeBobClient.Projects().List(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := hasExactlyTheseProjects(twoThreeProjects, sets.NewString("two", "three")); err != nil {
		t.Error(err)
	}
	allProjects, err := allBobClient.Projects().List(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := hasExactlyTheseProjects(allProjects, sets.NewString("one", "two", "three", "four")); err != nil {
		t.Error(err)
	}

	if err := fullBobClient.Projects().Delete("four"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	waitForOnlyDelete("four", allWatch, t)

	if err := fullBobClient.Projects().Delete("three"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	waitForOnlyDelete("three", allWatch, t)
	waitForOnlyDelete("three", twoThreeWatch, t)

	if err := fullBobClient.Projects().Delete("two"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	waitForOnlyDelete("two", allWatch, t)
	waitForOnlyDelete("two", oneTwoWatch, t)
	waitForOnlyDelete("two", twoThreeWatch, t)

	if err := fullBobClient.Projects().Delete("one"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	waitForOnlyDelete("one", allWatch, t)
	waitForOnlyDelete("one", oneTwoWatch, t)
}

func TestInvalidRoleRefs(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bobClient, _, _, err := testutil.GetClientForUser(*clusterAdminClientConfig, "bob")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	aliceClient, _, _, err := testutil.GetClientForUser(*clusterAdminClientConfig, "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "foo", "bob"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "bar", "alice"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	roleName := "missing-role"
	if _, err := clusterAdminClient.ClusterRoles().Create(&authorizationapi.ClusterRole{ObjectMeta: kapi.ObjectMeta{Name: roleName}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	modifyRole := &policy.RoleModificationOptions{RoleName: roleName, Users: []string{"someuser"}}
	// mess up rolebindings in "foo"
	modifyRole.RoleBindingAccessor = policy.NewLocalRoleBindingAccessor("foo", clusterAdminClient)
	if err := modifyRole.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// mess up rolebindings in "bar"
	modifyRole.RoleBindingAccessor = policy.NewLocalRoleBindingAccessor("bar", clusterAdminClient)
	if err := modifyRole.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// mess up clusterrolebindings
	modifyRole.RoleBindingAccessor = policy.NewClusterRoleBindingAccessor(clusterAdminClient)
	if err := modifyRole.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Orphan the rolebindings by deleting the role
	if err := clusterAdminClient.ClusterRoles().Delete(roleName); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// wait for evaluation errors to show up in both namespaces and at cluster scope
	if err := wait.PollImmediate(100*time.Millisecond, 10*time.Second, func() (bool, error) {
		review := &authorizationapi.ResourceAccessReview{Action: authorizationapi.Action{Verb: "get", Resource: "pods"}}
		review.Action.Namespace = "foo"
		if resp, err := clusterAdminClient.ResourceAccessReviews().Create(review); err != nil || resp.EvaluationError == "" {
			return false, err
		}
		review.Action.Namespace = "bar"
		if resp, err := clusterAdminClient.ResourceAccessReviews().Create(review); err != nil || resp.EvaluationError == "" {
			return false, err
		}
		review.Action.Namespace = ""
		if resp, err := clusterAdminClient.ResourceAccessReviews().Create(review); err != nil || resp.EvaluationError == "" {
			return false, err
		}
		return true, nil
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Make sure bob still sees his project (and only his project)
	if projects, err := bobClient.Projects().List(kapi.ListOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	} else if hasErr := hasExactlyTheseProjects(projects, sets.NewString("foo")); hasErr != nil {
		t.Error(hasErr)
	}
	// Make sure alice still sees her project (and only her project)
	if projects, err := aliceClient.Projects().List(kapi.ListOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	} else if hasErr := hasExactlyTheseProjects(projects, sets.NewString("bar")); hasErr != nil {
		t.Error(hasErr)
	}
	// Make sure cluster admin still sees all projects
	if projects, err := clusterAdminClient.Projects().List(kapi.ListOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	} else {
		projectNames := sets.NewString()
		for _, project := range projects.Items {
			projectNames.Insert(project.Name)
		}
		if !projectNames.HasAll("foo", "bar", "openshift-infra", "openshift", "default") {
			t.Errorf("Expected projects foo and bar, got %v", projectNames.List())
		}
	}
}

func hasExactlyTheseProjects(list *projectapi.ProjectList, projects sets.String) error {
	if len(list.Items) != len(projects) {
		return fmt.Errorf("expected %v, got %v", projects, list.Items)
	}
	for _, project := range list.Items {
		if !projects.Has(project.Name) {
			return fmt.Errorf("expected %v, got %v", projects, list.Items)
		}
	}
	return nil
}
