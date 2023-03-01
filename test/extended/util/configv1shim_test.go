package util

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	configv1 "github.com/openshift/api/config/v1"
	applyconfigv1 "github.com/openshift/client-go/config/applyconfigurations/config/v1"
	fakeconfigv1client "github.com/openshift/client-go/config/clientset/versioned/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/watch"
)

func createInfrastructureObject(name string) *configv1.Infrastructure {
	return &configv1.Infrastructure{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "config.openshift.io/v1",
			Kind:       "Infrastructure",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{},
		},
		Spec: configv1.InfrastructureSpec{
			PlatformSpec: configv1.PlatformSpec{
				Type: configv1.AWSPlatformType,
			},
		},
		Status: configv1.InfrastructureStatus{
			APIServerInternalURL:   "https://api-int.jchaloup-20230222.group-b.devcluster.openshift.com:6443",
			APIServerURL:           "https://api.jchaloup-20230222.group-b.devcluster.openshift.com:6443",
			ControlPlaneTopology:   configv1.HighlyAvailableTopologyMode,
			EtcdDiscoveryDomain:    "",
			InfrastructureName:     "jchaloup-20230222-cvx5s",
			InfrastructureTopology: configv1.HighlyAvailableTopologyMode,
			Platform:               configv1.AWSPlatformType,
			PlatformStatus: &configv1.PlatformStatus{
				Type: configv1.AWSPlatformType,
				AWS: &configv1.AWSPlatformStatus{
					Region: "us-east-1",
				},
			},
		},
	}
}

func TestConfigClientShimErrorOnMutation(t *testing.T) {
	updateNotPermitted := OperationNotPermitted{Action: "update"}
	updatestatusNotPermitted := OperationNotPermitted{Action: "updatestatus"}
	patchNotPermitted := OperationNotPermitted{Action: "patch"}
	applyNotPermitted := OperationNotPermitted{Action: "apply"}
	applyStatusNotPermitted := OperationNotPermitted{Action: "applystatus"}
	deleteNotPermitted := OperationNotPermitted{Action: "delete"}
	deleteCollectionNotPermitted := OperationNotPermitted{Action: "deletecollection"}

	object := createInfrastructureObject("cluster")
	object.Labels["deleteLabel"] = "somevalue"
	object2 := createInfrastructureObject("cluster2")
	object2.Labels["deleteLabel"] = "somevalue2"

	configClient := fakeconfigv1client.NewSimpleClientset(
		object2,
	)

	client := NewConfigClientShim(
		configClient,
		[]runtime.Object{object},
	)

	_, err := client.ConfigV1().Infrastructures().Get(context.TODO(), object.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected no error for a Get request, got %q instead", err)
	}

	_, err = client.ConfigV1().Infrastructures().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Expected no error for a List request, got %q instead", err)
	}

	_, err = client.ConfigV1().Infrastructures().Update(context.TODO(), object, metav1.UpdateOptions{})
	if err == nil || err.Error() != updateNotPermitted.Error() {
		t.Fatalf("Expected %q error for an Update request, got %q instead", updateNotPermitted.Error(), err)
	}

	_, err = client.ConfigV1().Infrastructures().Update(context.TODO(), object2, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Expected no error for an Update request, got %q instead", err)
	}

	_, err = client.ConfigV1().Infrastructures().UpdateStatus(context.TODO(), object, metav1.UpdateOptions{})
	if err == nil || err.Error() != updatestatusNotPermitted.Error() {
		t.Fatalf("Expected %q error for an UpdateStatus request, got %q instead", updatestatusNotPermitted.Error(), err)
	}

	_, err = client.ConfigV1().Infrastructures().UpdateStatus(context.TODO(), object2, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Expected no error for an UpdateStatus request, got %q instead", err)
	}

	oldData, err := json.Marshal(object)
	if err != nil {
		t.Fatalf("Unable to marshal an object: %v", err)
	}

	object.Labels["key"] = "value"
	newData, err := json.Marshal(object)
	if err != nil {
		t.Fatalf("Unable to marshal an object: %v", err)
	}
	delete(object.Labels, "key")

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, &configv1.Infrastructure{})
	if err != nil {
		t.Fatalf("Unable to create a patch: %v", err)
	}

	_, err = client.ConfigV1().Infrastructures().Patch(context.TODO(), object.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err == nil || err.Error() != patchNotPermitted.Error() {
		t.Fatalf("Expected %q error for a Patch request, got %q instead", patchNotPermitted.Error(), err)
	}

	_, err = client.ConfigV1().Infrastructures().Patch(context.TODO(), object2.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		t.Fatalf("Expected no error for a Patch request, got %q instead", err)
	}

	applyConfig, err := applyconfigv1.ExtractInfrastructure(object, "test-mgr")
	if err != nil {
		t.Fatalf("Unable to construct an apply config for %v: %v", object.Name, err)
	}
	_, err = client.ConfigV1().Infrastructures().Apply(context.TODO(), applyConfig, metav1.ApplyOptions{FieldManager: "test-mgr", Force: true})
	if err == nil || err.Error() != applyNotPermitted.Error() {
		t.Fatalf("Expected %q error for an Apply request, got %q instead", applyNotPermitted.Error(), err)
	}

	applyConfig2, err := applyconfigv1.ExtractInfrastructure(object2, "test-mgr")
	if err != nil {
		t.Fatalf("Unable to construct an apply config for %v: %v", object2.Name, err)
	}
	_, err = client.ConfigV1().Infrastructures().Apply(context.TODO(), applyConfig2, metav1.ApplyOptions{FieldManager: "test-mgr", Force: true})
	if err != nil {
		t.Fatalf("Expected no error for an Apply request, got %q instead", err)
	}

	applyStatusConfig, err := applyconfigv1.ExtractInfrastructureStatus(object, "test-mgr")
	if err != nil {
		t.Fatalf("Unable to construct an apply status config for %v: %v", object.Name, err)
	}
	_, err = client.ConfigV1().Infrastructures().ApplyStatus(context.TODO(), applyStatusConfig, metav1.ApplyOptions{FieldManager: "test-mgr", Force: true})
	if err == nil || err.Error() != applyStatusNotPermitted.Error() {
		t.Fatalf("Expected %q error for an ApplyStatus request, got %q instead", applyStatusNotPermitted.Error(), err)
	}

	applyStatusConfig2, err := applyconfigv1.ExtractInfrastructureStatus(object2, "test-mgr")
	if err != nil {
		t.Fatalf("Unable to construct an apply status config for %v: %v", object2.Name, err)
	}
	_, err = client.ConfigV1().Infrastructures().ApplyStatus(context.TODO(), applyStatusConfig2, metav1.ApplyOptions{FieldManager: "test-mgr", Force: true})
	if err != nil {
		t.Fatalf("Expected no error for an ApplyStatus request, got %q instead", err)
	}

	err = client.ConfigV1().Infrastructures().Delete(context.TODO(), object.Name, metav1.DeleteOptions{})
	if err == nil || err.Error() != deleteNotPermitted.Error() {
		t.Fatalf("Expected %q error for a Delete request, got %q instead", deleteNotPermitted.Error(), err)
	}

	err = client.ConfigV1().Infrastructures().Delete(context.TODO(), object2.Name, metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("Expected no error for a Delete request, got %q instead", err)
	}

	err = client.ConfigV1().Infrastructures().DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set(map[string]string{"deleteLabel": "somevalue"})).String(),
	})
	if err == nil || err.Error() != deleteCollectionNotPermitted.Error() {
		t.Fatalf("Expected %q error for a DeleteCollection request, got %q instead", deleteCollectionNotPermitted.Error(), err)
	}

	err = client.ConfigV1().Infrastructures().DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set(map[string]string{"deleteLabel": "somevalue2"})).String(),
	})
	if err != nil {
		t.Fatalf("Expected no error for a DeleteCollection request, got %q instead", err)
	}
}

func TestConfigClientShimWatchRequest(t *testing.T) {
	tests := []struct {
		name                string
		staticObjects       []runtime.Object
		realObjects         []runtime.Object
		expectedWatchEvents []watch.Event
	}{
		{
			name: "merging static and real objects, not object override",
			staticObjects: []runtime.Object{
				createInfrastructureObject("cluster"),
			},
			realObjects: []runtime.Object{
				createInfrastructureObject("cluster2"),
				createInfrastructureObject("cluster3"),
			},
			expectedWatchEvents: []watch.Event{
				{Type: watch.Added, Object: createInfrastructureObject("cluster")},
				{Type: watch.Added, Object: createInfrastructureObject("cluster2")},
				{Type: watch.Added, Object: createInfrastructureObject("cluster3")},
			},
		},
		{
			name: "merging static and real objects, static object override",
			staticObjects: []runtime.Object{
				createInfrastructureObject("cluster"),
			},
			realObjects: []runtime.Object{
				createInfrastructureObject("cluster"),
				createInfrastructureObject("cluster2"),
			},
			expectedWatchEvents: []watch.Event{
				{Type: watch.Added, Object: createInfrastructureObject("cluster")},
				{Type: watch.Added, Object: createInfrastructureObject("cluster2")},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configClient := fakeconfigv1client.NewSimpleClientset()

			client := NewConfigClientShim(
				configClient,
				test.staticObjects,
			)

			// The watch request has to created first when using the fake clientset
			resultChan, err := client.ConfigV1().Infrastructures().Watch(context.TODO(), metav1.ListOptions{})
			if err != nil {
				t.Fatalf("Expected no error, got %q instead", err)
			}
			defer resultChan.Stop()

			// And only then the fake clientset can be populated to generate the watch event
			for _, obj := range test.realObjects {
				configClient.Tracker().Add(obj)
			}

			// verify the watch events
			ticker := time.NewTicker(500 * time.Millisecond)
			size := len(test.expectedWatchEvents)
			eventCounter := 0
			for i := 0; i < size; i++ {
				select {
				case item := <-resultChan.ResultChan():
					diff := cmp.Diff(test.expectedWatchEvents[i], item)
					if diff != "" {
						t.Errorf("test '%s' failed. Results are not deep equal. mismatch (-want +got):\n%s", test.name, diff)
					}
					eventCounter++
				case <-ticker.C:
					t.Errorf("failed waiting for watch event")
				}
			}
			if eventCounter < size {
				t.Errorf("Expected %v watch events, got %v instead", eventCounter, size)
			}

		})
	}
}
