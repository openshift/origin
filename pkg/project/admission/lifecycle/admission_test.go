package lifecycle

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	testtypes "github.com/openshift/origin/pkg/project/admission/lifecycle/testing"
	projectcache "github.com/openshift/origin/pkg/project/cache"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
)

// TestIgnoreThatWhichCannotBeKnown verifies that the plug-in does not reject objects that are unknown to RESTMapper
func TestIgnoreThatWhichCannotBeKnown(t *testing.T) {
	handler := &lifecycle{}
	unknown := &testtypes.UnknownObject{}

	err := handler.Admit(admission.NewAttributesRecord(unknown, nil, kapi.Kind("kind").WithVersion("version"), "namespace", "name", kapi.Resource("resource").WithVersion("version"), "subresource", "CREATE", nil))
	if err != nil {
		t.Errorf("Admission control should not error if it finds an object it knows nothing about %v", err)
	}
}

// TestAdmissionExists verifies you cannot create Origin content if namespace is not known
func TestAdmissionExists(t *testing.T) {
	mockClient := &fake.Clientset{}
	mockClient.AddReactor("*", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &kapi.Namespace{}, fmt.Errorf("DOES NOT EXIST")
	})

	cache := projectcache.NewFake(mockClient.Core().Namespaces(), projectcache.NewCacheStore(cache.MetaNamespaceKeyFunc), "")

	mockClientset := fake.NewSimpleClientset()
	handler := &lifecycle{client: mockClientset}
	handler.SetProjectCache(cache)
	build := &buildapi.Build{
		ObjectMeta: metav1.ObjectMeta{Name: "buildid"},
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
	mockClient := &fake.Clientset{}
	mockClient.AddReactor("get", "namespaces", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("shouldn't get here")
	})
	cache := projectcache.NewFake(mockClient.Core().Namespaces(), store, "")

	mockClientset := fake.NewSimpleClientset()
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
