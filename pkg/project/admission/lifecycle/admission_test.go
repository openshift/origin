package admission

import (
	"fmt"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/admission"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"

	buildapi "github.com/openshift/origin/pkg/build/api"
	projectcache "github.com/openshift/origin/pkg/project/cache"
)

type UnknownObject struct{}

func (*UnknownObject) IsAnAPIObject() {}

// TestIgnoreThatWhichCannotBeKnown verifies that the plug-in does not reject objects that are unknown to RESTMapper
func TestIgnoreThatWhichCannotBeKnown(t *testing.T) {
	handler := &lifecycle{}
	unknown := &UnknownObject{}
	err := handler.Admit(admission.NewAttributesRecord(unknown, "who-cares", "unknown", "what", "CREATE", nil))
	if err != nil {
		t.Errorf("Admission control should not error if it finds an object it knows nothing about %v", err)
	}
}

// TestAdmissionExists verifies you cannot create Origin content if namespace is not known
func TestAdmissionExists(t *testing.T) {
	mockClient := &testclient.Fake{
		Err: fmt.Errorf("DOES NOT EXIST"),
	}
	projectcache.FakeProjectCache(mockClient, cache.NewStore(cache.MetaNamespaceKeyFunc), "")
	handler := &lifecycle{client: mockClient}
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
	err := handler.Admit(admission.NewAttributesRecord(build, "Build", "bogus-ns", "builds", "CREATE", nil))
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
	projectcache.FakeProjectCache(mockClient, store, "")
	handler := &lifecycle{client: mockClient}
	build := &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{Name: "buildid", Namespace: "other"},
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
	err := handler.Admit(admission.NewAttributesRecord(build, "Build", build.Namespace, "builds", "CREATE", nil))
	if err != nil {
		t.Errorf("Unexpected error returned from admission handler: %v", err)
	}

	// change namespace state to terminating
	namespaceObj.Status.Phase = kapi.NamespaceTerminating
	store.Add(namespaceObj)

	// verify create operations in the namespace cause an error
	err = handler.Admit(admission.NewAttributesRecord(build, "Build", build.Namespace, "builds", "CREATE", nil))
	if err == nil {
		t.Errorf("Expected error rejecting creates in a namespace when it is terminating")
	}

	// verify update operations in the namespace can proceed
	err = handler.Admit(admission.NewAttributesRecord(build, "Build", build.Namespace, "builds", "UPDATE", nil))
	if err != nil {
		t.Errorf("Unexpected error returned from admission handler: %v", err)
	}

	// verify delete operations in the namespace can proceed
	err = handler.Admit(admission.NewAttributesRecord(nil, "Build", build.Namespace, "builds", "DELETE", nil))
	if err != nil {
		t.Errorf("Unexpected error returned from admission handler: %v", err)
	}

}
