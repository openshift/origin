// +build integration,!no-etcd

package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/rest"
	kv1beta3 "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta3"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	namespaceetcd "github.com/GoogleCloudPlatform/kubernetes/pkg/registry/namespace/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools/etcdtest"
	"github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/admit"

	"github.com/openshift/origin/pkg/api/latest"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	projectapi "github.com/openshift/origin/pkg/project/api"
	projectregistry "github.com/openshift/origin/pkg/project/registry/project/proxy"
	testutil "github.com/openshift/origin/test/util"
)

func init() {
	testutil.RequireEtcd()
}

// TestProjectIsNamespace verifies that a project is a namespace, and a namespace is a project
func TestProjectIsNamespace(t *testing.T) {
	testutil.DeleteAllEtcdKeys()
	etcdClient := testutil.NewEtcdClient()
	etcdHelper, err := master.NewEtcdHelper(etcdClient, "", etcdtest.PathPrefix())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// create a kube and its client
	kubeInterfaces, _ := klatest.InterfacesFor(klatest.Version)
	namespaceStorage, _, _ := namespaceetcd.NewStorage(etcdHelper)
	kubeStorage := map[string]rest.Storage{
		"namespaces": namespaceStorage,
	}

	osMux := http.NewServeMux()
	server := httptest.NewServer(osMux)
	defer server.Close()
	handlerContainer := master.NewHandlerContainer(osMux)

	version := &apiserver.APIGroupVersion{
		Root:    "/api",
		Version: "v1beta3",

		Storage: kubeStorage,
		Codec:   kv1beta3.Codec,

		Mapper: klatest.RESTMapper,

		Creater:   kapi.Scheme,
		Typer:     kapi.Scheme,
		Convertor: kapi.Scheme,
		Linker:    kubeInterfaces.MetadataAccessor,

		Admit:   admit.NewAlwaysAdmit(),
		Context: kapi.NewRequestContextMapper(),
	}
	if err := version.InstallREST(handlerContainer); err != nil {
		t.Fatalf("unable to install REST: %v", err)
	}

	kubeClient, err := kclient.New(&kclient.Config{Host: server.URL, Version: "v1beta3"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// create an origin
	originInterfaces, _ := latest.InterfacesFor(latest.Version)
	originStorage := map[string]rest.Storage{
		"projects": projectregistry.NewREST(kubeClient.Namespaces(), nil),
	}
	osVersion := &apiserver.APIGroupVersion{
		Root:    "/oapi",
		Version: "v1",

		Storage: originStorage,
		Codec:   latest.Codec,

		Mapper: latest.RESTMapper,

		Creater:   kapi.Scheme,
		Typer:     kapi.Scheme,
		Convertor: kapi.Scheme,
		Linker:    originInterfaces.MetadataAccessor,

		Admit:   admit.NewAlwaysAdmit(),
		Context: kapi.NewRequestContextMapper(),
	}
	if err := osVersion.InstallREST(handlerContainer); err != nil {
		t.Fatalf("unable to install REST: %v", err)
	}

	originClient, err := client.New(&kclient.Config{Host: server.URL})
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
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
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
