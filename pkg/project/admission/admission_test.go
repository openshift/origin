package admission

import (
	"fmt"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/admission"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// TestAdmissionExists verifies you cannot create Origin content if namespace is not known
func TestAdmissionExists(t *testing.T) {
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	mockClient := &testclient.Fake{
		Err: fmt.Errorf("DOES NOT EXIST"),
	}
	handler := &lifecycle{
		client: mockClient,
		store:  store,
	}
	build := &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{Name: "buildid"},
		Parameters: buildapi.BuildParameters{
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
				DockerImageReference: "repository/data",
			},
		},
		Status: buildapi.BuildStatusNew,
	}
	err := handler.Admit(admission.NewAttributesRecord(build, "Build", "bogus-ns", "builds", "CREATE"))
	if err == nil {
		t.Errorf("Expected an error because namespace does not exist")
	}
}

// TestAdmissionLifecycle verifies you cannot create Origin content if namespace is terminating
func TestAdmissionLifecycle(t *testing.T) {
	namespaceObj := &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "test",
			Namespace: "",
		},
		Status: kapi.NamespaceStatus{
			Phase: kapi.NamespaceActive,
		},
	}
	store := cache.NewStore(cache.MetaNamespaceIndexFunc)
	store.Add(namespaceObj)
	mockClient := &testclient.Fake{}
	handler := &lifecycle{
		client: mockClient,
		store:  store,
	}
	build := &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{Name: "buildid"},
		Parameters: buildapi.BuildParameters{
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
				DockerImageReference: "repository/data",
			},
		},
		Status: buildapi.BuildStatusNew,
	}
	err := handler.Admit(admission.NewAttributesRecord(build, "Build", namespaceObj.Namespace, "builds", "CREATE"))
	if err != nil {
		t.Errorf("Unexpected error returned from admission handler: %v", err)
	}

	// change namespace state to terminating
	namespaceObj.Status.Phase = kapi.NamespaceTerminating
	store.Add(namespaceObj)

	// verify create operations in the namespace cause an error
	err = handler.Admit(admission.NewAttributesRecord(build, "Build", namespaceObj.Namespace, "builds", "CREATE"))
	if err == nil {
		t.Errorf("Expected error rejecting creates in a namespace when it is terminating")
	}

	// verify update operations in the namespace can proceed
	err = handler.Admit(admission.NewAttributesRecord(build, "Build", namespaceObj.Namespace, "builds", "UPDATE"))
	if err != nil {
		t.Errorf("Unexpected error returned from admission handler: %v", err)
	}

	// verify delete operations in the namespace can proceed
	err = handler.Admit(admission.NewAttributesRecord(nil, "Build", namespaceObj.Namespace, "builds", "DELETE"))
	if err != nil {
		t.Errorf("Unexpected error returned from admission handler: %v", err)
	}

}
