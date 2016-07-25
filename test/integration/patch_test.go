package integration

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/pborman/uuid"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/apiserver"
	"k8s.io/kubernetes/pkg/client/restclient"

	templatesapi "github.com/openshift/origin/pkg/template/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestPatchConflicts(t *testing.T) {

	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)

	_, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()

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

	objName := "myobj"
	ns := "patch-namespace"

	if _, err := clusterAdminKubeClient.Namespaces().Create(&kapi.Namespace{ObjectMeta: kapi.ObjectMeta{Name: ns}}); err != nil {
		t.Fatalf("Error creating namespace:%v", err)
	}
	if _, err := clusterAdminKubeClient.Secrets(ns).Create(&kapi.Secret{ObjectMeta: kapi.ObjectMeta{Name: objName}}); err != nil {
		t.Fatalf("Error creating k8s resource:%v", err)
	}
	if _, err := clusterAdminClient.Templates(ns).Create(&templatesapi.Template{ObjectMeta: kapi.ObjectMeta{Name: objName}}); err != nil {
		t.Fatalf("Error creating origin resource:%v", err)
	}

	testcases := []struct {
		client   *restclient.RESTClient
		resource string
	}{
		{
			client:   clusterAdminKubeClient.RESTClient,
			resource: "secrets",
		},
		{
			client:   clusterAdminClient.RESTClient,
			resource: "templates",
		},
	}

	for _, tc := range testcases {
		successes := int32(0)

		// Force patch to deal with resourceVersion conflicts applying non-conflicting patches
		// ensure it handles reapplies without internal errors
		wg := sync.WaitGroup{}
		for i := 0; i < (2 * apiserver.MaxPatchConflicts); i++ {
			wg.Add(1)
			go func(labelName string) {
				defer wg.Done()
				labelValue := uuid.NewRandom().String()

				obj, err := tc.client.Patch(kapi.StrategicMergePatchType).
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

		if successes < apiserver.MaxPatchConflicts {
			t.Errorf("Expected at least %d successful patches for %s, got %d", apiserver.MaxPatchConflicts, tc.resource, successes)
		} else {
			t.Logf("Got %d successful patches for %s", successes, tc.resource)
		}
	}
}
