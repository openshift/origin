package integration

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/pborman/uuid"

	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	apiendpointhandlers "k8s.io/apiserver/pkg/endpoints/handlers"
	restclient "k8s.io/client-go/rest"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	templatesapi "github.com/openshift/origin/pkg/template/apis/template"
	templateclient "github.com/openshift/origin/pkg/template/generated/internalclientset"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestPatchConflicts(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminTemplateClient := templateclient.NewForConfigOrDie(clusterAdminClientConfig).Template()

	clusterAdminKubeClientset, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	objName := "myobj"
	ns := "patch-namespace"

	if _, err := clusterAdminKubeClientset.Core().Namespaces().Create(&kapi.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}); err != nil {
		t.Fatalf("Error creating namespace:%v", err)
	}
	if _, err := clusterAdminKubeClientset.Core().Secrets(ns).Create(&kapi.Secret{ObjectMeta: metav1.ObjectMeta{Name: objName}}); err != nil {
		t.Fatalf("Error creating k8s resource:%v", err)
	}
	if _, err := clusterAdminTemplateClient.Templates(ns).Create(&templatesapi.Template{ObjectMeta: metav1.ObjectMeta{Name: objName}}); err != nil {
		t.Fatalf("Error creating origin resource:%v", err)
	}

	testcases := []struct {
		client   restclient.Interface
		resource string
	}{
		{
			client:   clusterAdminKubeClientset.Core().RESTClient(),
			resource: "secrets",
		},
		{
			client:   clusterAdminTemplateClient.RESTClient(),
			resource: "templates",
		},
	}

	for _, tc := range testcases {
		successes := int32(0)

		// Force patch to deal with resourceVersion conflicts applying non-conflicting patches
		// ensure it handles reapplies without internal errors
		wg := sync.WaitGroup{}
		for i := 0; i < (2 * apiendpointhandlers.MaxRetryWhenPatchConflicts); i++ {
			wg.Add(1)
			go func(labelName string) {
				defer wg.Done()
				labelValue := uuid.NewRandom().String()

				obj, err := tc.client.Patch(types.StrategicMergePatchType).
					Namespace(ns).
					Resource(tc.resource).
					Name(objName).
					Body([]byte(fmt.Sprintf(`{"metadata":{"labels":{"%s":"%s"}}}`, labelName, labelValue))).
					Do().
					Get()

				if kapierrs.IsConflict(err) {
					t.Logf("tolerated conflict error patching %s: %v", tc.resource, err)
					return
				}
				if err != nil {
					t.Errorf("error patching %s: %v", tc.resource, err)
					return
				}

				accessor, err := meta.Accessor(obj)
				if err != nil {
					t.Errorf("error getting object from %s: %v", tc.resource, err)
					return
				}
				if accessor.GetLabels()[labelName] != labelValue {
					t.Errorf("patch of %s was ineffective, expected %s=%s, got labels %#v", tc.resource, labelName, labelValue, accessor.GetLabels())
					return
				}

				atomic.AddInt32(&successes, 1)
			}(fmt.Sprintf("label-%d", i))
		}
		wg.Wait()

		if successes < apiendpointhandlers.MaxRetryWhenPatchConflicts {
			t.Errorf("Expected at least %d successful patches for %s, got %d", apiendpointhandlers.MaxRetryWhenPatchConflicts, tc.resource, successes)
		} else {
			t.Logf("Got %d successful patches for %s", successes, tc.resource)
		}
	}
}
