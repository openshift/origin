// +build integration,!no-etcd

package integration

import (
	"net/http/httptest"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	kv1beta1 "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta1"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/namespace"
	"github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/admit"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/api/v1beta1"
	"github.com/openshift/origin/pkg/client"
	projectapi "github.com/openshift/origin/pkg/project/api"
	projectregistry "github.com/openshift/origin/pkg/project/registry/project"
)

func init() {
	requireEtcd()
}

// TestProjectIsNamespace verifies that a project is a namespace, and a namespace is a project
func TestProjectIsNamespace(t *testing.T) {
	deleteAllEtcdKeys()
	etcdClient := newEtcdClient()
	etcdHelper, err := master.NewEtcdHelper(etcdClient, "")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// create a kube and its client
	kubeInterfaces, _ := klatest.InterfacesFor(klatest.Version)
	namespaceRegistry := namespace.NewEtcdRegistry(etcdHelper)
	kubeStorage := map[string]apiserver.RESTStorage{
		"namespaces": namespace.NewREST(namespaceRegistry),
	}
	kubeServer := httptest.NewServer(apiserver.Handle(kubeStorage, kv1beta1.Codec, "/api", "v1beta1", kubeInterfaces.MetadataAccessor, admit.NewAlwaysAdmit(), kapi.NewRequestContextMapper(), klatest.RESTMapper))
	defer kubeServer.Close()
	kubeClient, err := kclient.New(&kclient.Config{Host: kubeServer.URL})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// create an origin
	originInterfaces, _ := latest.InterfacesFor(latest.Version)
	originStorage := map[string]apiserver.RESTStorage{
		"projects": projectregistry.NewREST(kubeClient.Namespaces()),
	}
	originServer := httptest.NewServer(apiserver.Handle(originStorage, v1beta1.Codec, "/osapi", "v1beta1", originInterfaces.MetadataAccessor, admit.NewAlwaysAdmit(), kapi.NewRequestContextMapper(), latest.RESTMapper))
	defer originServer.Close()
	originClient, err := client.New(&kclient.Config{Host: originServer.URL})
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
		ObjectMeta:  kapi.ObjectMeta{Name: "new-project"},
		DisplayName: "Hello World",
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
	if project.DisplayName != namespace.Annotations["displayname"] {
		t.Fatalf("Project display name did not match namespace annotation, project %v, namespace %v", project.DisplayName, namespace.Annotations["displayname"])
	}

}
