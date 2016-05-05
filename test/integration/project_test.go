// +build integration

package integration

import (
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/watch"

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
	_, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
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

	select {
	case event := <-w.ResultChan():
		if event.Type != watch.Added {
			t.Errorf("expected added, got %v", event)
		}
		project := event.Object.(*projectapi.Project)
		if project.Name != "ns-01" {
			t.Fatalf("expected %v, got %#v", "ns-01", project)
		}

	case <-time.After(3 * time.Second):
		t.Fatalf("timeout")
	}

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
	// wait for the add
	for {
		select {
		case event := <-w.ResultChan():
			project := event.Object.(*projectapi.Project)
			t.Logf("got %#v %#v", event, project)
			if event.Type == watch.Added && project.Name == "ns-02" {
				return
			}

		case <-time.After(3 * time.Second):
			t.Fatalf("timeout")
		}
	}

	if err := addBob.RemoveRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// wait for the delete
	for {
		select {
		case event := <-w.ResultChan():
			project := event.Object.(*projectapi.Project)
			t.Logf("got %#v %#v", event, project)
			if event.Type == watch.Deleted && project.Name == "ns-02" {
				return
			}

		case <-time.After(3 * time.Second):
			t.Fatalf("timeout")
		}
	}

	// test the "start from beginning watch"
	beginningWatch, err := bobClient.Projects().Watch(kapi.ListOptions{ResourceVersion: "0"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	select {
	case event := <-beginningWatch.ResultChan():
		if event.Type != watch.Added {
			t.Errorf("expected added, got %v", event)
		}
		project := event.Object.(*projectapi.Project)
		if project.Name != "ns-01" {
			t.Fatalf("expected %v, got %#v", "ns-01", project)
		}

	case <-time.After(3 * time.Second):
		t.Fatalf("timeout")
	}

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
