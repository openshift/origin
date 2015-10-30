// +build integration,etcd

package integration

import (
	"fmt"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/wait"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	policy "github.com/openshift/origin/pkg/cmd/admin/policy"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	projectapi "github.com/openshift/origin/pkg/project/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func init() {
	testutil.RequireEtcd()
}

// TestProjectIsNamespace verifies that a project is a namespace, and a namespace is a project
func TestProjectIsNamespace(t *testing.T) {
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
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
				"openshift.io/display-name":  "Hello World",
				"openshift.io/node-selector": "env=test",
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
	if project.Annotations["openshift.io/display-name"] != namespace.Annotations["openshift.io/display-name"] {
		t.Fatalf("Project display name did not match namespace annotation, project %v, namespace %v", project.Annotations["openshift.io/display-name"], namespace.Annotations["openshift.io/display-name"])
	}
	if project.Annotations["openshift.io/node-selector"] != namespace.Annotations["openshift.io/node-selector"] {
		t.Fatalf("Project node selector did not match namespace node selector, project %v, namespace %v", project.Annotations["openshift.io/node-selector"], namespace.Annotations["openshift.io/node-selector"])
	}
}

// TestProjectMustExist verifies that content cannot be added in a project that does not exist
func TestProjectMustExist(t *testing.T) {
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
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
		ObjectMeta: kapi.ObjectMeta{Name: "buildid", Namespace: "default"},
		Spec: buildapi.BuildSpec{
			Source: buildapi.BuildSource{
				Type: buildapi.BuildSourceGit,
				Git: &buildapi.GitBuildSource{
					URI: "http://github.com/my/repository",
				},
				ContextDir: "context",
			},
			Strategy: buildapi.BuildStrategy{
				Type:           buildapi.DockerBuildStrategyType,
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

// TestProjectsForUser verifies that projects for users can be retrieved
func TestProjectsForUser(t *testing.T) {
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatal(err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kubeClient.Namespaces().Create(&kapi.Namespace{ObjectMeta: kapi.ObjectMeta{Name: "first"}}); err != nil {
		t.Fatal(err)
	}

	adminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}

	haroldClient, err := testserver.CreateNewProject(adminClient, *clusterAdminClientConfig, "second", "harold")
	if err != nil {
		t.Fatal(err)
	}

	addViewer := policy.RoleModificationOptions{
		RoleName:            bootstrappolicy.ViewRoleName,
		RoleBindingAccessor: policy.NewLocalRoleBindingAccessor("second", haroldClient),
		Users:               []string{"valerie"},
		Groups:              []string{"my-group"},
	}

	if err := addViewer.AddRole(); err != nil {
		t.Fatal(err)
	}

	if err := wait.PollImmediate(100*time.Millisecond, 10*time.Second, func() (bool, error) {
		p, err := adminClient.Projects().List(nil, nil)
		if err != nil {
			return false, nil
		}
		return projectNames(p).HasAll("first", "second"), nil
	}); err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		client  *client.Client
		user    string
		expects []string
		exact   []string
		errFn   func(err error) bool
	}{
		{
			client:  adminClient,
			user:    "system:admin",
			expects: []string{"first", "second"},
		},
		{
			client: adminClient,
			user:   "harold",
			exact:  []string{"second"},
		},
		{
			client: adminClient,
			user:   "valerie",
			exact:  []string{"second"},
		},
		{
			client: haroldClient,
			user:   "harold",
			errFn: func(err error) bool {
				return errors.IsForbidden(err)
			},
		},
	}
	for i, test := range testCases {
		// self
		err := wait.PollImmediate(100*time.Millisecond, 5*time.Second, func() (bool, error) {
			results, err := test.client.Projects().ListForUser(test.user, nil, nil)
			if err != nil {
				if test.errFn == nil && !test.errFn(err) {
					return false, fmt.Errorf("unexpected error: %v", err)
				}
				return true, nil
			}
			actual := projectNames(results)
			if !actual.HasAll(test.expects...) {
				t.Logf("%d: unexpected projects: %#v", i, results)
				return false, nil
			}
			if test.exact != nil && !actual.Equal(sets.NewString(test.exact...)) {
				t.Logf("%d: unexpected projects: %#v", i, results)
				return false, nil
			}
			return true, nil
		})
		if err != nil {
			t.Errorf("%d: failed: %v", i, err)
		}
	}
}

func projectNames(projects *projectapi.ProjectList) sets.String {
	names := sets.NewString()
	for _, p := range projects.Items {
		names.Insert(p.Name)
	}
	return names
}
