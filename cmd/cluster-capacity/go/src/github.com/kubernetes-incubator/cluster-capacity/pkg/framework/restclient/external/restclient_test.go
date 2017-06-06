/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package external

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/apis/extensions/v1beta1"

	ccapi "github.com/kubernetes-incubator/cluster-capacity/pkg/api"
	"github.com/kubernetes-incubator/cluster-capacity/pkg/framework/store"
	"github.com/kubernetes-incubator/cluster-capacity/pkg/test"
)

func testPodsData() []*v1.Pod {
	pods := make([]*v1.Pod, 0, 10)
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("pod%v", i)
		item := test.PodExample(name)
		pods = append(pods, &item)
	}
	return pods
}

func testServicesData() []*v1.Service {
	svcs := make([]*v1.Service, 0, 10)
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("service%v", i)
		item := test.ServiceExample(name)
		svcs = append(svcs, &item)
	}
	return svcs
}

func testReplicationControllersData() []*v1.ReplicationController {
	rcs := make([]*v1.ReplicationController, 0, 10)
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("rc%v", i)
		item := test.ReplicationControllerExample(name)
		rcs = append(rcs, &item)
	}
	return rcs
}

func testPersistentVolumesData() []*v1.PersistentVolume {
	pvs := make([]*v1.PersistentVolume, 0, 10)
	for i := 0; i < 1; i++ {
		name := fmt.Sprintf("pv%v", i)
		item := test.PersistentVolumeExample(name)
		pvs = append(pvs, &item)
	}
	return pvs
}

func testPersistentVolumeClaimsData() []*v1.PersistentVolumeClaim {
	pvcs := make([]*v1.PersistentVolumeClaim, 0, 10)
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("pvc%v", i)
		item := test.PersistentVolumeClaimExample(name)
		pvcs = append(pvcs, &item)
	}
	return pvcs
}

func testNodesData() []*v1.Node {
	nodes := make([]*v1.Node, 0, 10)
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("node%v", i)
		item := test.NodeExample(name)
		nodes = append(nodes, &item)
	}
	return nodes
}

func testReplicaSetsData() []*v1beta1.ReplicaSet {
	rss := make([]*v1beta1.ReplicaSet, 0, 10)
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("replicaset%v", i)
		item := v1beta1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test", ResourceVersion: "125"},
			Spec: v1beta1.ReplicaSetSpec{
				Replicas: &[]int32{3}[0],
			},
		}
		rss = append(rss, &item)
	}
	return rss
}

func newTestListRestClient() *RESTClient {

	resourceStore := &store.FakeResourceStore{
		PodsData: func() []*v1.Pod {
			return testPodsData()
		},
		ServicesData: func() []*v1.Service {
			return testServicesData()
		},
		ReplicationControllersData: func() []*v1.ReplicationController {
			return testReplicationControllersData()
		},
		PersistentVolumesData: func() []*v1.PersistentVolume {
			return testPersistentVolumesData()
		},
		PersistentVolumeClaimsData: func() []*v1.PersistentVolumeClaim {
			return testPersistentVolumeClaimsData()
		},
		NodesData: func() []*v1.Node {
			return testNodesData()
		},
	}

	client := &RESTClient{
		NegotiatedSerializer: api.Codecs,
		resourceStore:        resourceStore,
	}

	return client
}

func compareItems(expected, actual interface{}) bool {
	if reflect.TypeOf(expected).Kind() != reflect.Slice {
		return false
	}

	if reflect.TypeOf(actual).Kind() != reflect.Slice {
		return false
	}

	expectedSlice := reflect.ValueOf(expected)
	expectedMap := make(map[string]interface{})
	for i := 0; i < expectedSlice.Len(); i++ {
		metaLocal := expectedSlice.Index(i).FieldByName("ObjectMeta").Interface().(metav1.ObjectMeta)
		key := strings.Join([]string{metaLocal.Namespace, metaLocal.Name, metaLocal.ResourceVersion}, "/")
		expectedMap[key] = expectedSlice.Index(i).Interface()
	}

	actualMap := make(map[string]interface{})
	actualSlice := reflect.ValueOf(actual)
	for i := 0; i < actualSlice.Len(); i++ {
		metaLocal := actualSlice.Index(i).FieldByName("ObjectMeta").Interface().(metav1.ObjectMeta)
		key := strings.Join([]string{metaLocal.Namespace, metaLocal.Name, metaLocal.ResourceVersion}, "/")
		actualMap[key] = actualSlice.Index(i).Interface()
	}

	return reflect.DeepEqual(expectedMap, actualMap)
}

func getResourceList(client cache.Getter, resource ccapi.ResourceType) runtime.Object {
	// client listerWatcher
	listerWatcher := cache.NewListWatchFromClient(client, resource.String(), metav1.NamespaceAll, fields.ParseSelectorOrDie(""))
	options := metav1.ListOptions{ResourceVersion: "0"}
	l, _ := listerWatcher.List(options)
	return l
}

func TestSyncPods(t *testing.T) {

	fakeClient := newTestListRestClient()
	expected := fakeClient.Pods(fields.Everything()).Items

	list := getResourceList(fakeClient, ccapi.Pods)
	items, err := meta.ExtractList(list)
	if err != nil {
		t.Errorf("Unable to understand list result %#v (%v)", list, err)
	}

	found := make([]v1.Pod, 0, len(items))
	for _, item := range items {
		found = append(found, *((interface{})(item).(*v1.Pod)))
	}

	if !compareItems(expected, found) {
		t.Errorf("unexpected object: expected: %#v\n actual: %#v", expected, found)
	}
}

func TestSyncServices(t *testing.T) {

	fakeClient := newTestListRestClient()
	expected := fakeClient.Services(fields.Everything()).Items

	list := getResourceList(fakeClient, ccapi.Services)
	items, err := meta.ExtractList(list)
	if err != nil {
		t.Errorf("Unable to understand list result %#v (%v)", list, err)
	}

	found := make([]v1.Service, 0, len(items))
	for _, item := range items {
		found = append(found, *((interface{})(item).(*v1.Service)))
	}

	if !compareItems(expected, found) {
		t.Errorf("unexpected object: expected: %#v\n actual: %#v", expected, found)
	}
}

func TestSyncReplicationControllers(t *testing.T) {

	fakeClient := newTestListRestClient()
	expected := fakeClient.ReplicationControllers(fields.Everything()).Items

	list := getResourceList(fakeClient, ccapi.ReplicationControllers)
	items, err := meta.ExtractList(list)
	if err != nil {
		t.Errorf("Unable to understand list result %#v (%v)", list, err)
	}

	found := make([]v1.ReplicationController, 0, len(items))
	for _, item := range items {
		found = append(found, *((interface{})(item).(*v1.ReplicationController)))
	}

	if !compareItems(expected, found) {
		t.Errorf("unexpected object: expected: %#v\n actual: %#v", expected, found)
	}
}

func TestSyncPersistentVolumes(t *testing.T) {
	fakeClient := newTestListRestClient()
	expected := fakeClient.PersistentVolumes(fields.Everything()).Items

	list := getResourceList(fakeClient, ccapi.PersistentVolumes)
	items, err := meta.ExtractList(list)
	if err != nil {
		t.Errorf("Unable to understand list result %#v (%v)", list, err)
	}
	found := make([]v1.PersistentVolume, 0, len(items))
	for _, item := range items {
		found = append(found, *((interface{})(item).(*v1.PersistentVolume)))
	}

	if !compareItems(expected, found) {
		t.Errorf("unexpected object: expected: %#v\n actual: %#v", expected, found)
	}
}

func TestSyncPersistentVolumeClaims(t *testing.T) {
	fakeClient := newTestListRestClient()
	expected := fakeClient.PersistentVolumeClaims(fields.Everything()).Items

	list := getResourceList(fakeClient, ccapi.PersistentVolumeClaims)
	items, err := meta.ExtractList(list)
	if err != nil {
		t.Errorf("Unable to understand list result %#v (%v)", list, err)
	}
	found := make([]v1.PersistentVolumeClaim, 0, len(items))
	for _, item := range items {
		found = append(found, *((interface{})(item).(*v1.PersistentVolumeClaim)))
	}

	if !compareItems(expected, found) {
		t.Errorf("unexpected object: expected: %#v\n actual: %#v", expected, found)
	}
}

func TestSyncNodes(t *testing.T) {
	fakeClient := newTestListRestClient()
	expected := fakeClient.Nodes(fields.Everything()).Items

	list := getResourceList(fakeClient, ccapi.Nodes)
	items, err := meta.ExtractList(list)
	if err != nil {
		t.Errorf("Unable to understand list result %#v (%v)", list, err)
	}
	found := make([]v1.Node, 0, len(items))
	for _, item := range items {
		found = append(found, *((interface{})(item).(*v1.Node)))
	}

	if !compareItems(expected, found) {
		t.Errorf("unexpected object: expected: %#v\n actual: %#v", expected, found)
	}
}

//func testSyncReplicaSets(t *testing.T) {
//	fakeClient := newTestListRestClient()
//	expected := fakeClient.ReplicaSets().Items
//	emulator := NewClientEmulator()
//
//	err := emulator.sync(fakeClient)
//
//	if err != nil {
//		t.Fatalf("Unexpected error: %v", err)
//	}
//
//	storedItems := emulator.ReplicaSetCache.List()
//	actual := make([]extensions.ReplicaSet, 0, len(storedItems))
//	for _, value := range storedItems {
//		item, ok := value.(*extensions.ReplicaSet)
//		if !ok {
//			t.Errorf("Expected api.Service type, found different")
//		}
//		actual = append(actual, *item)
//	}
//
//	if !compareItems(expected, actual) {
//		t.Errorf("unexpected object: expected: %#v\n actual: %#v", expected, actual)
//	}
//}
