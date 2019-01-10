package resourceapply

import (
	"reflect"
	"sort"
	"testing"

	"github.com/ghodss/yaml"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/diff"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"

	"github.com/openshift/library-go/pkg/operator/events"
)

const (
	fakeServiceMonitor = `apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: cluster-kube-apiserver
  namespace: openshift-kube-apiserver
spec:
  endpoints:
    - bearerTokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
      interval: 30s
      metricRelabelings:
        - action: drop
          regex: etcd_(debugging|disk|request|server).*
          sourceLabels:
            - __name__
      port: https
      scheme: https
      tlsConfig:
        caFile: /var/run/secrets/kubernetes.io/serviceaccount/service-ca.crt
        serverName: apiserver.openshift-kube-apiserver.svc
  jobLabel: component
  namespaceSelector:
    matchNames:
      - openshift-kube-apiserver
  selector:
    matchLabels:
      app: openshift-kube-apiserver
`
	fakeIncompleteServiceMonitor = `apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: cluster-kube-apiserver
  namespace: openshift-kube-apiserver
spec:
  endpoints:
    - bearerTokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
      interval: 30s
      metricRelabelings:
        - action: drop
          regex: etcd_(debugging|disk|request|server).*
          sourceLabels:
            - __name__
      port: https
      scheme: https
  jobLabel: component
  namespaceSelector:
    matchNames:
      - wrong-name
  selector:
    matchLabels:
      custom: custom-label
      app: openshift-kube-apiserver
`
)

func readServiceMonitorFromBytes(monitorBytes []byte) *unstructured.Unstructured {
	monitorJSON, err := yaml.YAMLToJSON(monitorBytes)
	if err != nil {
		panic(err)
	}
	monitorObj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, monitorJSON)
	if err != nil {
		panic(err)
	}
	required, ok := monitorObj.(*unstructured.Unstructured)
	if !ok {
		panic("unexpected object")
	}
	return required
}

func TestApplyServiceMonitor(t *testing.T) {
	dynamicScheme := runtime.NewScheme()
	dynamicScheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"}, &unstructured.Unstructured{})

	dynamicClient := dynamicfake.NewSimpleDynamicClient(dynamicScheme, readServiceMonitorFromBytes([]byte(fakeServiceMonitor)))

	modified, err := ApplyServiceMonitor(dynamicClient, events.NewInMemoryRecorder("monitor-test"), []byte(fakeIncompleteServiceMonitor))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !modified {
		t.Fatalf("expected the service monitor will be modified, it was not")
	}

	if len(dynamicClient.Actions()) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(dynamicClient.Actions()))
	}

	_, isUpdate := dynamicClient.Actions()[1].(clienttesting.UpdateAction)
	if !isUpdate {
		t.Fatalf("expected second action to be update, got %+v", dynamicClient.Actions()[1])
	}

	updatedMonitorObj, err := dynamicClient.Resource(schema.GroupVersionResource{
		Group:    "monitoring.coreos.com",
		Version:  "v1",
		Resource: "servicemonitors",
	}).Namespace("openshift-kube-apiserver").Get("cluster-kube-apiserver", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected to get update monitor, got: %v", err)
	}

	labels, _, err := unstructured.NestedStringMap(updatedMonitorObj.UnstructuredContent(), "spec", "selector", "matchLabels")
	if err != nil {
		t.Fatalf("unable to get selector: %v", err)
	}

	expectedKeys := []string{"app", "custom"}
	resultKeys := []string{}
	for key := range labels {
		resultKeys = append(resultKeys, key)
	}
	sort.Strings(resultKeys)

	if !reflect.DeepEqual(resultKeys, expectedKeys) {
		t.Fatalf("expected %#v selectors, got %#v", expectedKeys, resultKeys)
	}

	requiredMonitorSpec, _, _ := unstructured.NestedMap(readServiceMonitorFromBytes([]byte(fakeServiceMonitor)).UnstructuredContent(), "spec", "endpoints")
	existingMonitorSpec, _, _ := unstructured.NestedMap(updatedMonitorObj.UnstructuredContent(), "spec", "endpoints")

	if !equality.Semantic.DeepEqual(requiredMonitorSpec, existingMonitorSpec) {
		t.Fatalf("expected resulting service monitor spec endpoints to match required spec: %s", diff.ObjectDiff(requiredMonitorSpec, existingMonitorSpec))
	}

}
