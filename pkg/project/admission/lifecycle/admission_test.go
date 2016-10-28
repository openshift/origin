package lifecycle

import (
	"fmt"
	"testing"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/cache"
	clientsetfake "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	"k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"

	buildapi "github.com/openshift/origin/pkg/build/api"
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

	err := handler.Admit(admission.NewAttributesRecord(unknown, nil, kapi.Kind("kind").WithVersion("version"), "namespace", "name", kapi.Resource("resource").WithVersion("version"), "subresource", "CREATE", nil))
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

	mockClientset := clientsetfake.NewSimpleClientset()
	handler := &lifecycle{client: mockClientset}
	handler.SetProjectCache(cache)
	build := &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{Name: "buildid"},
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
	err := handler.Admit(admission.NewAttributesRecord(build, nil, kapi.Kind("Build").WithVersion("v1"), "namespace", "name", kapi.Resource("builds").WithVersion("v1"), "", "CREATE", nil))
	if err == nil {
		t.Errorf("Expected an error because namespace does not exist")
	}
}

func TestSAR(t *testing.T) {
	store := projectcache.NewCacheStore(cache.IndexFuncToKeyFuncAdapter(cache.MetaNamespaceIndexFunc))
	mockClient := &testclient.Fake{}
	mockClient.AddReactor("get", "namespaces", func(action testclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("shouldn't get here")
	})
	cache := projectcache.NewFake(mockClient.Namespaces(), store, "")

	mockClientset := clientsetfake.NewSimpleClientset()
	handler := &lifecycle{client: mockClientset, creatableResources: recommendedCreatableResources}
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
		err := handler.Admit(admission.NewAttributesRecord(nil, nil, kapi.Kind(v.kind).WithVersion("v1"), "foo", "name", kapi.Resource(v.resource).WithVersion("v1"), "", "CREATE", nil))
		if err != nil {
			t.Errorf("Unexpected error for %s returned from admission handler: %v", k, err)
		}
	}
}
