package lifecycle

import (
	"fmt"
	"strings"
	"testing"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/unversioned/testclient"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"
	"k8s.io/kubernetes/pkg/runtime"
	etcdstorage "k8s.io/kubernetes/pkg/storage/etcd"
	"k8s.io/kubernetes/pkg/util/sets"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	projectcache "github.com/openshift/origin/pkg/project/cache"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
)

type UnknownObject struct {
	unversioned.TypeMeta
}

func (obj *UnknownObject) GetObjectKind() unversioned.ObjectKind { return &obj.TypeMeta }

// TestIgnoreThatWhichCannotBeKnown verifies that the plug-in does not reject objects that are unknown to RESTMapper
func TestIgnoreThatWhichCannotBeKnown(t *testing.T) {
	handler := &lifecycle{}
	unknown := &UnknownObject{}

	err := handler.Admit(admission.NewAttributesRecord(unknown, kapi.Kind("kind"), "namespace", "name", kapi.Resource("resource"), "subresource", "CREATE", nil))
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

	cache := projectcache.NewFake(mockClient.Namespaces(), projectcache.NewCacheStore(cache.MetaNamespaceKeyFunc), "")
	handler := &lifecycle{client: mockClient}
	handler.SetProjectCache(cache)
	build := &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{Name: "buildid"},
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
	err := handler.Admit(admission.NewAttributesRecord(build, kapi.Kind("Build"), "namespace", "name", kapi.Resource("builds"), "", "CREATE", nil))
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
	store := projectcache.NewCacheStore(cache.IndexFuncToKeyFuncAdapter(cache.MetaNamespaceIndexFunc))
	store.Add(namespaceObj)
	mockClient := &testclient.Fake{}
	cache := projectcache.NewFake(mockClient.Namespaces(), store, "")
	handler := &lifecycle{client: mockClient}
	handler.SetProjectCache(cache)
	build := &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{Name: "buildid", Namespace: "other"},
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
	err := handler.Admit(admission.NewAttributesRecord(build, kapi.Kind("Build"), build.Namespace, "name", kapi.Resource("builds"), "", "CREATE", nil))
	if err != nil {
		t.Errorf("Unexpected error returned from admission handler: %v", err)
	}

	// change namespace state to terminating
	namespaceObj.Status.Phase = kapi.NamespaceTerminating
	store.Add(namespaceObj)

	// verify create operations in the namespace cause an error
	err = handler.Admit(admission.NewAttributesRecord(build, kapi.Kind("Build"), build.Namespace, "name", kapi.Resource("builds"), "", "CREATE", nil))
	if err == nil {
		t.Errorf("Expected error rejecting creates in a namespace when it is terminating")
	}

	// verify update operations in the namespace can proceed
	err = handler.Admit(admission.NewAttributesRecord(build, kapi.Kind("Build"), build.Namespace, "name", kapi.Resource("builds"), "", "UPDATE", nil))
	if err != nil {
		t.Errorf("Unexpected error returned from admission handler: %v", err)
	}

	// verify delete operations in the namespace can proceed
	err = handler.Admit(admission.NewAttributesRecord(nil, kapi.Kind("Build"), build.Namespace, "name", kapi.Resource("builds"), "", "DELETE", nil))
	if err != nil {
		t.Errorf("Unexpected error returned from admission handler: %v", err)
	}

}

// TestCreatesAllowedDuringNamespaceDeletion checks to make sure that the resources in the whitelist are allowed
func TestCreatesAllowedDuringNamespaceDeletion(t *testing.T) {
	config := &origin.MasterConfig{
		KubeletClientConfig: &kubeletclient.KubeletClientConfig{},
		EtcdHelper:          etcdstorage.NewEtcdStorage(nil, nil, ""),
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
	store := projectcache.NewCacheStore(cache.IndexFuncToKeyFuncAdapter(cache.MetaNamespaceIndexFunc))
	mockClient := &testclient.Fake{}
	mockClient.AddReactor("get", "namespaces", func(action testclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("shouldn't get here")
	})
	cache := projectcache.NewFake(mockClient.Namespaces(), store, "")
	handler := &lifecycle{client: mockClient}
	handler.SetProjectCache(cache)

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
		err := handler.Admit(admission.NewAttributesRecord(nil, kapi.Kind(v.kind), "foo", "name", kapi.Resource(v.resource), "", "CREATE", nil))
		if err != nil {
			t.Errorf("Unexpected error for %s returned from admission handler: %v", k, err)
		}
	}
}
