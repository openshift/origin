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

func createNetworkObject(name string) *configv1.Network {
	return &configv1.Network{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "config.openshift.io/v1",
			Kind:       "Infrastructure",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{},
		},
		Spec: configv1.NetworkSpec{
			ClusterNetwork: []configv1.ClusterNetworkEntry{
				{
					CIDR:       "10.128.0.0/14",
					HostPrefix: 23,
				},
			},
			NetworkType:    "OVNKubernetes",
			ServiceNetwork: []string{"172.30.0.0/16"},
		},
		Status: configv1.NetworkStatus{
			ClusterNetwork: []configv1.ClusterNetworkEntry{
				{
					CIDR:       "10.128.0.0/14",
					HostPrefix: 23,
				},
			},
			ClusterNetworkMTU: 8901,
			NetworkType:       "OVNKubernetes",
			ServiceNetwork:    []string{"172.30.0.0/16"},
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

	staticObject := createInfrastructureObject("staticObject")
	staticObject.Labels["deleteLabel"] = "somevalue"
	realObject := createInfrastructureObject("realObject")
	realObject.Labels["deleteLabel"] = "somevalue2"

	configClient := fakeconfigv1client.NewSimpleClientset(
		realObject,
	)

	client := NewConfigClientShim(
		configClient,
		[]runtime.Object{staticObject},
	)

	_, err := client.ConfigV1().Infrastructures().Get(context.TODO(), staticObject.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected no error for a Get request, got %q instead", err)
	}

	_, err = client.ConfigV1().Infrastructures().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Expected no error for a List request, got %q instead", err)
	}

	_, err = client.ConfigV1().Infrastructures().Update(context.TODO(), staticObject, metav1.UpdateOptions{})
	if err == nil || err.Error() != updateNotPermitted.Error() {
		t.Fatalf("Expected %q error for an Update request, got %q instead", updateNotPermitted.Error(), err)
	}

	_, err = client.ConfigV1().Infrastructures().Update(context.TODO(), realObject, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Expected no error for an Update request, got %q instead", err)
	}

	_, err = client.ConfigV1().Infrastructures().UpdateStatus(context.TODO(), staticObject, metav1.UpdateOptions{})
	if err == nil || err.Error() != updatestatusNotPermitted.Error() {
		t.Fatalf("Expected %q error for an UpdateStatus request, got %q instead", updatestatusNotPermitted.Error(), err)
	}

	_, err = client.ConfigV1().Infrastructures().UpdateStatus(context.TODO(), realObject, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Expected no error for an UpdateStatus request, got %q instead", err)
	}

	oldData, err := json.Marshal(staticObject)
	if err != nil {
		t.Fatalf("Unable to marshal an staticObject: %v", err)
	}

	staticObject.Labels["key"] = "value"
	newData, err := json.Marshal(staticObject)
	if err != nil {
		t.Fatalf("Unable to marshal an object: %v", err)
	}
	delete(staticObject.Labels, "key")

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, &configv1.Infrastructure{})
	if err != nil {
		t.Fatalf("Unable to create a patch: %v", err)
	}

	_, err = client.ConfigV1().Infrastructures().Patch(context.TODO(), staticObject.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err == nil || err.Error() != patchNotPermitted.Error() {
		t.Fatalf("Expected %q error for a Patch request, got %q instead", patchNotPermitted.Error(), err)
	}

	_, err = client.ConfigV1().Infrastructures().Patch(context.TODO(), realObject.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		t.Fatalf("Expected no error for a Patch request, got %q instead", err)
	}

	applyConfig, err := applyconfigv1.ExtractInfrastructure(staticObject, "test-mgr")
	if err != nil {
		t.Fatalf("Unable to construct an apply config for %v: %v", staticObject.Name, err)
	}
	_, err = client.ConfigV1().Infrastructures().Apply(context.TODO(), applyConfig, metav1.ApplyOptions{FieldManager: "test-mgr", Force: true})
	if err == nil || err.Error() != applyNotPermitted.Error() {
		t.Fatalf("Expected %q error for an Apply request, got %q instead", applyNotPermitted.Error(), err)
	}

	applyConfig2, err := applyconfigv1.ExtractInfrastructure(realObject, "test-mgr")
	if err != nil {
		t.Fatalf("Unable to construct an apply config for %v: %v", realObject.Name, err)
	}
	_, err = client.ConfigV1().Infrastructures().Apply(context.TODO(), applyConfig2, metav1.ApplyOptions{FieldManager: "test-mgr", Force: true})
	if err != nil {
		t.Fatalf("Expected no error for an Apply request, got %q instead", err)
	}

	applyStatusConfig, err := applyconfigv1.ExtractInfrastructureStatus(staticObject, "test-mgr")
	if err != nil {
		t.Fatalf("Unable to construct an apply status config for %v: %v", staticObject.Name, err)
	}
	_, err = client.ConfigV1().Infrastructures().ApplyStatus(context.TODO(), applyStatusConfig, metav1.ApplyOptions{FieldManager: "test-mgr", Force: true})
	if err == nil || err.Error() != applyStatusNotPermitted.Error() {
		t.Fatalf("Expected %q error for an ApplyStatus request, got %q instead", applyStatusNotPermitted.Error(), err)
	}

	applyStatusConfig2, err := applyconfigv1.ExtractInfrastructureStatus(realObject, "test-mgr")
	if err != nil {
		t.Fatalf("Unable to construct an apply status config for %v: %v", realObject.Name, err)
	}
	_, err = client.ConfigV1().Infrastructures().ApplyStatus(context.TODO(), applyStatusConfig2, metav1.ApplyOptions{FieldManager: "test-mgr", Force: true})
	if err != nil {
		t.Fatalf("Expected no error for an ApplyStatus request, got %q instead", err)
	}

	err = client.ConfigV1().Infrastructures().Delete(context.TODO(), staticObject.Name, metav1.DeleteOptions{})
	if err == nil || err.Error() != deleteNotPermitted.Error() {
		t.Fatalf("Expected %q error for a Delete request, got %q instead", deleteNotPermitted.Error(), err)
	}

	err = client.ConfigV1().Infrastructures().Delete(context.TODO(), realObject.Name, metav1.DeleteOptions{})
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
		fieldSelector       string
		watch               func(client *ConfigClientShim, ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
		expectedWatchEvents []watch.Event
	}{
		{
			name: "merging static and real infrastructure objects, not object override",
			staticObjects: []runtime.Object{
				createInfrastructureObject("staticObject"),
			},
			realObjects: []runtime.Object{
				createInfrastructureObject("realObject"),
				createInfrastructureObject("realObject2"),
			},
			watch: func(client *ConfigClientShim, ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
				return client.ConfigV1().Infrastructures().Watch(ctx, opts)
			},
			expectedWatchEvents: []watch.Event{
				{Type: watch.Added, Object: createInfrastructureObject("staticObject")},
				{Type: watch.Added, Object: createInfrastructureObject("realObject")},
				{Type: watch.Added, Object: createInfrastructureObject("realObject2")},
			},
		},
		{
			name: "merging static and real infrastructure objects, static object override",
			staticObjects: []runtime.Object{
				createInfrastructureObject("staticObject"),
			},
			realObjects: []runtime.Object{
				createInfrastructureObject("staticObject"),
				createInfrastructureObject("realObject"),
			},
			watch: func(client *ConfigClientShim, ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
				return client.ConfigV1().Infrastructures().Watch(ctx, opts)
			},
			expectedWatchEvents: []watch.Event{
				{Type: watch.Added, Object: createInfrastructureObject("staticObject")},
				{Type: watch.Added, Object: createInfrastructureObject("realObject")},
			},
		},
		{
			name: "merging static and real infrastructure objects, field selector match",
			staticObjects: []runtime.Object{
				createInfrastructureObject("staticObject"),
			},
			realObjects: []runtime.Object{
				createInfrastructureObject("staticObject"),
			},
			watch: func(client *ConfigClientShim, ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
				return client.ConfigV1().Infrastructures().Watch(ctx, opts)
			},
			fieldSelector: "metadata.name==staticObject",
			expectedWatchEvents: []watch.Event{
				{Type: watch.Added, Object: createInfrastructureObject("staticObject")},
			},
		},
		{
			name: "merging static and real infrastructure objects, field selector no match",
			staticObjects: []runtime.Object{
				createInfrastructureObject("staticObject"),
			},
			realObjects: []runtime.Object{
				createInfrastructureObject("staticObject"),
			},
			watch: func(client *ConfigClientShim, ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
				return client.ConfigV1().Infrastructures().Watch(ctx, opts)
			},
			fieldSelector:       "metadata.name=!staticObject",
			expectedWatchEvents: []watch.Event{},
		},
		{
			name: "merging static and real network objects, not object override",
			staticObjects: []runtime.Object{
				createNetworkObject("staticObject"),
			},
			realObjects: []runtime.Object{
				createNetworkObject("realObject"),
				createNetworkObject("realObject2"),
			},
			watch: func(client *ConfigClientShim, ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
				return client.ConfigV1().Networks().Watch(ctx, opts)
			},
			expectedWatchEvents: []watch.Event{
				{Type: watch.Added, Object: createNetworkObject("staticObject")},
				{Type: watch.Added, Object: createNetworkObject("realObject")},
				{Type: watch.Added, Object: createNetworkObject("realObject2")},
			},
		},
		{
			name: "merging static and real network objects, static object override",
			staticObjects: []runtime.Object{
				createNetworkObject("staticObject"),
			},
			realObjects: []runtime.Object{
				createNetworkObject("staticObject"),
				createNetworkObject("realObject"),
			},
			watch: func(client *ConfigClientShim, ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
				return client.ConfigV1().Networks().Watch(ctx, opts)
			},
			expectedWatchEvents: []watch.Event{
				{Type: watch.Added, Object: createNetworkObject("staticObject")},
				{Type: watch.Added, Object: createNetworkObject("realObject")},
			},
		},
		{
			name: "merging static and real network objects, field selector match",
			staticObjects: []runtime.Object{
				createNetworkObject("staticObject"),
			},
			realObjects: []runtime.Object{
				createNetworkObject("staticObject"),
			},
			watch: func(client *ConfigClientShim, ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
				return client.ConfigV1().Networks().Watch(ctx, opts)
			},
			fieldSelector: "metadata.name==staticObject",
			expectedWatchEvents: []watch.Event{
				{Type: watch.Added, Object: createNetworkObject("staticObject")},
			},
		},
		{
			name: "merging static and real network objects, field selector no match",
			staticObjects: []runtime.Object{
				createNetworkObject("staticObject"),
			},
			realObjects: []runtime.Object{
				createNetworkObject("staticObject"),
			},
			watch: func(client *ConfigClientShim, ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
				return client.ConfigV1().Networks().Watch(ctx, opts)
			},
			fieldSelector:       "metadata.name=!staticObject",
			expectedWatchEvents: []watch.Event{},
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
			resultChan, err := test.watch(client, context.TODO(), metav1.ListOptions{FieldSelector: test.fieldSelector})
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
			eventsSize := len(resultChan.ResultChan())
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
				t.Errorf("Expected %v watch events, got %v instead", size, eventCounter)
			}
			if eventsSize > size {
				t.Errorf("Expected %v watch events, got %v instead", size, eventsSize)
			}
		})
	}
}

func TestConfigClientShimList(t *testing.T) {

	staticObject := createInfrastructureObject("staticObject")
	staticObject.Labels["static"] = "true"

	realObject1 := createInfrastructureObject("staticObject")
	realObject1.Labels["static"] = "false"

	realObject2 := createInfrastructureObject("realObject")
	realObject2.Labels["static"] = "false"

	configClient := fakeconfigv1client.NewSimpleClientset(
		realObject1,
		realObject2,
	)

	client := NewConfigClientShim(
		configClient,
		[]runtime.Object{staticObject},
	)

	listItems, err := client.ConfigV1().Infrastructures().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Expected no error for a List request, got %q instead", err)
	}

	if len(listItems.Items) != 2 {
		t.Fatalf("Expected only a single item in the list, got %v instead", len(listItems.Items))
	}

	var staticObj *configv1.Infrastructure
	realObjFound := false

	for _, item := range listItems.Items {
		if item.Name == "staticObject" {
			obj := item
			staticObj = &obj
		}
		if item.Name == "realObject" {
			realObjFound = true
		}
	}

	if staticObj == nil {
		t.Fatalf("Expected to find a static object, found none")
	}

	if staticObj.Labels["static"] == "false" {
		t.Fatalf("Expected static object, not real object")
	}

	if !realObjFound {
		t.Fatalf("Unable to find a real object in the list")
	}
}

func TestConfigClientShimListInfrastructureFieldSelector(t *testing.T) {

	tests := []struct {
		name          string
		fieldSelector string
		expectedLen   int
	}{
		{
			name:          "field selector matches",
			fieldSelector: "metadata.name=staticObject",
			expectedLen:   1,
		},
		{
			name:          "field selector does not match",
			fieldSelector: "metadata.name!=staticObject",
			expectedLen:   0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			staticObject := createInfrastructureObject("staticObject")
			staticObject.Labels["static"] = "true"

			realObject := createInfrastructureObject("staticObject")
			realObject.Labels["static"] = "false"

			configClient := fakeconfigv1client.NewSimpleClientset(
				realObject,
			)

			client := NewConfigClientShim(
				configClient,
				[]runtime.Object{staticObject},
			)

			listItems, err := client.ConfigV1().Infrastructures().List(context.TODO(), metav1.ListOptions{FieldSelector: test.fieldSelector})
			if err != nil {
				t.Fatalf("Expected no error for a List request, got %q instead", err)
			}

			if len(listItems.Items) != test.expectedLen {
				t.Fatalf("Expected %v items in the list, got %v instead", test.expectedLen, len(listItems.Items))
			}

			if test.expectedLen > 0 {
				if listItems.Items[0].Labels["static"] == "false" {
					t.Fatalf("Expected static object, not real object")
				}
			}
		})
	}
}

func TestConfigClientShimListNetworkFieldSelector(t *testing.T) {

	tests := []struct {
		name          string
		fieldSelector string
		expectedLen   int
	}{
		{
			name:          "field selector matches",
			fieldSelector: "metadata.name=staticObject",
			expectedLen:   1,
		},
		{
			name:          "field selector does not match",
			fieldSelector: "metadata.name!=staticObject",
			expectedLen:   0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			staticObject := createNetworkObject("staticObject")
			staticObject.Labels["static"] = "true"

			realObject := createNetworkObject("staticObject")
			realObject.Labels["static"] = "false"

			configClient := fakeconfigv1client.NewSimpleClientset(
				realObject,
			)

			client := NewConfigClientShim(
				configClient,
				[]runtime.Object{staticObject},
			)

			listItems, err := client.ConfigV1().Networks().List(context.TODO(), metav1.ListOptions{FieldSelector: test.fieldSelector})
			if err != nil {
				t.Fatalf("Expected no error for a List request, got %q instead", err)
			}

			if len(listItems.Items) != test.expectedLen {
				t.Fatalf("Expected %v items in the list, got %v instead", test.expectedLen, len(listItems.Items))
			}

			if test.expectedLen > 0 {
				if listItems.Items[0].Labels["static"] == "false" {
					t.Fatalf("Expected static object, not real object")
				}
			}
		})
	}
}
