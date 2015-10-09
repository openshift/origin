package admission

import (
	"fmt"
	"strings"
	"testing"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/latest"
	"k8s.io/kubernetes/pkg/client/cache"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"
	etcdstorage "k8s.io/kubernetes/pkg/storage/etcd"
	"k8s.io/kubernetes/pkg/util/sets"

	buildapi "github.com/openshift/origin/pkg/build/api"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	projectcache "github.com/openshift/origin/pkg/project/cache"
)

type UnknownObject struct{}

func (*UnknownObject) IsAnAPIObject() {}

// TestIgnoreThatWhichCannotBeKnown verifies that the plug-in does not reject objects that are unknown to RESTMapper
func TestIgnoreThatWhichCannotBeKnown(t *testing.T) {
	handler := &lifecycle{}
	unknown := &UnknownObject{}

	err := handler.Admit(admission.NewAttributesRecord(unknown, "kind", "namespace", "name", "resource", "subresource", "CREATE", nil))
	if err != nil {
		t.Errorf("Admission control should not error if it finds an object it knows nothing about %v", err)
	}
}

// TestAdmissionExists verifies you cannot create Origin content if namespace is not known
func TestAdmissionExists(t *testing.T) {
	mockClient := &testclient.Fake{}
	mockClient.AddReactor("*", "*", func(action testclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, &kapi.Namespace{}, fmt.Errorf("DOES NOT EXIST")
	})

	projectcache.FakeProjectCache(mockClient, cache.NewStore(cache.MetaNamespaceKeyFunc), "")
	handler := &lifecycle{client: mockClient}
	build := &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{Name: "buildid"},
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
	err := handler.Admit(admission.NewAttributesRecord(build, "Build", "namespace", "name", "builds", "", "CREATE", nil))
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
	store := cache.NewStore(cache.IndexFuncToKeyFuncAdapter(cache.MetaNamespaceIndexFunc))
	store.Add(namespaceObj)
	mockClient := &testclient.Fake{}
	projectcache.FakeProjectCache(mockClient, store, "")
	handler := &lifecycle{client: mockClient}
	build := &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{Name: "buildid", Namespace: "other"},
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
	err := handler.Admit(admission.NewAttributesRecord(build, "Build", build.Namespace, "name", "builds", "", "CREATE", nil))
	if err != nil {
		t.Errorf("Unexpected error returned from admission handler: %v", err)
	}

	// change namespace state to terminating
	namespaceObj.Status.Phase = kapi.NamespaceTerminating
	store.Add(namespaceObj)

	// verify create operations in the namespace cause an error
	err = handler.Admit(admission.NewAttributesRecord(build, "Build", build.Namespace, "name", "builds", "", "CREATE", nil))
	if err == nil {
		t.Errorf("Expected error rejecting creates in a namespace when it is terminating")
	}

	// verify update operations in the namespace can proceed
	err = handler.Admit(admission.NewAttributesRecord(build, "Build", build.Namespace, "name", "builds", "", "UPDATE", nil))
	if err != nil {
		t.Errorf("Unexpected error returned from admission handler: %v", err)
	}

	// verify delete operations in the namespace can proceed
	err = handler.Admit(admission.NewAttributesRecord(nil, "Build", build.Namespace, "name", "builds", "", "DELETE", nil))
	if err != nil {
		t.Errorf("Unexpected error returned from admission handler: %v", err)
	}

}

// TestCreatesAllowedDuringNamespaceDeletion checks to make sure that the resources in the whitelist are allowed
func TestCreatesAllowedDuringNamespaceDeletion(t *testing.T) {
	config := &origin.MasterConfig{
		KubeletClientConfig: &kclient.KubeletConfig{},
		EtcdHelper:          etcdstorage.NewEtcdStorage(nil, nil, ""),
		Options: configapi.MasterConfig{
			EtcdStorageConfig: configapi.EtcdStorageConfig{
				KubernetesStorageVersion: latest.Version,
				KubernetesStoragePrefix:  "foo",
			},
		},
	}
	storageMap := config.GetRestStorage()
	resources := sets.String{}

	for resource := range storageMap {
		resources.Insert(strings.ToLower(resource))
	}

	for resource := range recommendedCreatableResources {
		if !resources.Has(resource) {
			t.Errorf("recommendedCreatableResources has resource %v, but that resource isn't registered.", resource)
		}
	}
}

func TestSAR(t *testing.T) {
	store := cache.NewStore(cache.IndexFuncToKeyFuncAdapter(cache.MetaNamespaceIndexFunc))
	mockClient := &testclient.Fake{}
	mockClient.AddReactor("get", "namespaces", func(action testclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("shouldn't get here")
	})
	projectcache.FakeProjectCache(mockClient, store, "")
	handler := &lifecycle{client: mockClient}

	tests := map[string]struct {
		kind     string
		resource string
	}{
		"subject access review": {
			kind:     "SubjectAccessReview",
			resource: "subjectaccessreviews",
		},
		"local subject access review": {
			kind:     "LocalSubjectAccessReview",
			resource: "localsubjectaccessreviews",
		},
	}

	for k, v := range tests {
		err := handler.Admit(admission.NewAttributesRecord(nil, v.kind, "foo", "name", v.resource, "", "CREATE", nil))
		if err != nil {
			t.Errorf("Unexpected error for %s returned from admission handler: %v", k, err)
		}
	}
}
