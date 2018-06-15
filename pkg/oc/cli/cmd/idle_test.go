package cmd

import (
	"encoding/json"
	"fmt"
	"testing"

	unidlingapi "github.com/openshift/origin/pkg/unidling/api"

	oappsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ktypes "k8s.io/apimachinery/pkg/types"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/apis/core/install"
)

func makePod(name string, rc metav1.Object, namespace string, t *testing.T) kapi.Pod {
	pod := kapi.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	pod.OwnerReferences = append(pod.OwnerReferences,
		*metav1.NewControllerRef(rc, kapi.SchemeGroupVersion.WithKind("ReplicationController")))

	return pod
}

func makeRC(name string, dc metav1.Object, namespace string, t *testing.T) *kapi.ReplicationController {
	rc := kapi.ReplicationController{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: make(map[string]string),
		},
	}

	if dc != nil {
		rc.OwnerReferences = append(rc.OwnerReferences, *metav1.NewControllerRef(dc,
			oappsapi.SchemeGroupVersion.WithKind("DeploymentConfig")))
	}

	return &rc
}

func makePodRef(name, namespace string) *kapi.ObjectReference {
	return &kapi.ObjectReference{
		Kind:      "Pod",
		Name:      name,
		Namespace: namespace,
	}
}

func makeRCRef(name string) *metav1.OwnerReference {
	return metav1.NewControllerRef(&metav1.ObjectMeta{Name: name},
		kapi.SchemeGroupVersion.WithKind("ReplicationController"))
}

func TestFindIdlablesForEndpoints(t *testing.T) {
	endpoints := &kapi.Endpoints{
		Subsets: []kapi.EndpointSubset{
			{
				Addresses: []kapi.EndpointAddress{
					{
						TargetRef: makePodRef("somepod1", "somens1"),
					},
					{
						TargetRef: makePodRef("somepod2", "somens1"),
					},
					{
						TargetRef: &kapi.ObjectReference{
							Kind:      "Cheese",
							Name:      "cheddar",
							Namespace: "somens",
						},
					},
				},
			},
			{
				Addresses: []kapi.EndpointAddress{
					{},
					{
						TargetRef: makePodRef("somepod3", "somens1"),
					},
					{
						TargetRef: makePodRef("somepod4", "somens1"),
					},
					{
						TargetRef: makePodRef("somepod5", "somens1"),
					},
					{
						TargetRef: makePodRef("missingpod", "somens1"),
					},
				},
			},
			{
				Addresses: []kapi.EndpointAddress{
					{},
					{
						TargetRef: makePodRef("somepod1", "somens2"),
					},
				},
			},
		},
	}

	controllers := map[string]metav1.Object{
		"somens1/somerc1": makeRC("somerc1", &metav1.ObjectMeta{Name: "somedc1"}, "somens1", t),
		"somens1/somerc2": makeRC("somerc2", nil, "somens1", t),
		"somens1/somerc3": makeRC("somerc3", &metav1.ObjectMeta{Name: "somedc2"}, "somens1", t),
		"somens1/somerc4": makeRC("somerc4", &metav1.ObjectMeta{Name: "somedc2"}, "somens1", t),
		// make sure we test having multiple namespaces with identically-named RCs
		"somens2/somerc2": makeRC("somerc2", nil, "somens2", t),
	}

	pods := map[kapi.ObjectReference]kapi.Pod{
		*makePodRef("somepod1", "somens1"): makePod("somepod1", controllers["somens1/somerc1"], "somens1", t),
		*makePodRef("somepod2", "somens1"): makePod("somepod2", controllers["somens1/somerc2"], "somens1", t),
		*makePodRef("somepod3", "somens1"): makePod("somepod3", controllers["somens1/somerc1"], "somens1", t),
		*makePodRef("somepod4", "somens1"): makePod("somepod4", controllers["somens1/somerc3"], "somens1", t),
		*makePodRef("somepod5", "somens1"): makePod("somepod5", controllers["somens1/somerc4"], "somens1", t),
		*makePodRef("somepod1", "somens2"): makePod("somepod5", controllers["somens2/somerc2"], "somens2", t),
	}

	getPod := func(ref kapi.ObjectReference) (*kapi.Pod, error) {
		if pod, ok := pods[ref]; ok {
			return &pod, nil
		}
		return nil, kerrors.NewNotFound(schema.GroupResource{Group: kapi.GroupName, Resource: "Pod"}, ref.Name)
	}

	getController := func(ref namespacedOwnerReference) (metav1.Object, error) {
		if controller, ok := controllers[fmt.Sprintf("%s/%s", ref.namespace, ref.Name)]; ok {
			return controller, nil
		}

		// NB: this GroupResource declaration plays fast and loose with various distinctions
		// but is good enough for being an error in a test
		return nil, kerrors.NewNotFound(schema.GroupResource{Group: kapi.GroupName, Resource: ref.Kind}, ref.Name)

	}

	refSet, err := findScalableResourcesForEndpoints(endpoints, getPod, getController)

	if err != nil {
		t.Fatalf("Unexpected error while finding idlables: %v", err)
	}

	expectedRefs := []namespacedCrossGroupObjectReference{
		{
			CrossGroupObjectReference: unidlingapi.CrossGroupObjectReference{
				Kind:  "DeploymentConfig",
				Name:  "somedc1",
				Group: oappsapi.GroupName,
			},
			namespace: "somens1",
		},
		{
			CrossGroupObjectReference: unidlingapi.CrossGroupObjectReference{
				Kind:  "DeploymentConfig",
				Name:  "somedc2",
				Group: oappsapi.GroupName,
			},
			namespace: "somens1",
		},
		{
			CrossGroupObjectReference: unidlingapi.CrossGroupObjectReference{
				Kind:  "ReplicationController",
				Name:  "somerc2",
				Group: kapi.GroupName,
			},
			namespace: "somens1",
		},
		{
			CrossGroupObjectReference: unidlingapi.CrossGroupObjectReference{
				Kind:  "ReplicationController",
				Name:  "somerc2",
				Group: kapi.GroupName,
			},
			namespace: "somens2",
		},
	}

	if len(refSet) != len(expectedRefs) {
		t.Errorf("Expected to get somedc1, somedc2, somerc2, instead got %#v", refSet)
	}

	for _, ref := range expectedRefs {
		if _, ok := refSet[ref]; !ok {
			t.Errorf("expected scalable %q to be present, but was not in %v", ref.Name, refSet)
		}
	}
}

func TestPairScalesWithIdlables(t *testing.T) {
	oldScaleRefs := []unidlingapi.RecordedScaleReference{
		{
			CrossGroupObjectReference: unidlingapi.CrossGroupObjectReference{
				Kind: "ReplicationController",
				Name: "somerc1",
			},
			Replicas: 5,
		},
		{
			CrossGroupObjectReference: unidlingapi.CrossGroupObjectReference{
				Kind: "DeploymentConfig",
				Name: "somedc1",
			},
			Replicas: 3,
		},
	}

	oldScaleRefBytes, err := json.Marshal(oldScaleRefs)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	oldAnnotations := map[string]string{
		unidlingapi.UnidleTargetAnnotation: string(oldScaleRefBytes),
	}

	newRawRefs := map[unidlingapi.CrossGroupObjectReference]struct{}{
		{
			Kind: "ReplicationController",
			Name: "somerc1",
		}: {},
		{
			Kind: "ReplicationController",
			Name: "somerc2",
		}: {},
		{
			Kind: "DeploymentConfig",
			Name: "somedc1",
		}: {},
		{
			Kind: "DeploymentConfig",
			Name: "somedc2",
		}: {},
	}

	scales := map[namespacedCrossGroupObjectReference]int32{
		{
			CrossGroupObjectReference: unidlingapi.CrossGroupObjectReference{
				Kind: "ReplicationController",
				Name: "somerc1",
			},
			namespace: "somens1",
		}: 2,
		{
			CrossGroupObjectReference: unidlingapi.CrossGroupObjectReference{
				Kind: "ReplicationController",
				Name: "somerc1",
			},
			namespace: "somens2",
		}: 3,
		{
			CrossGroupObjectReference: unidlingapi.CrossGroupObjectReference{
				Kind: "ReplicationController",
				Name: "somerc2",
			},
			namespace: "somens1",
		}: 5,
		{
			CrossGroupObjectReference: unidlingapi.CrossGroupObjectReference{
				Kind: "DeploymentConfig",
				Name: "somedc1",
			},
			namespace: "somens1",
		}: 0,
		{
			CrossGroupObjectReference: unidlingapi.CrossGroupObjectReference{
				Kind: "DeploymentConfig",
				Name: "somedc2",
			},
			namespace: "somens1",
		}: 0,
	}

	newScaleRefs, err := pairScalesWithScaleRefs(ktypes.NamespacedName{Name: "somesvc", Namespace: "somens1"}, oldAnnotations, newRawRefs, scales)

	expectedScaleRefs := map[unidlingapi.RecordedScaleReference]struct{}{
		{
			CrossGroupObjectReference: unidlingapi.CrossGroupObjectReference{
				Kind: "ReplicationController",
				Name: "somerc1",
			},
			Replicas: 2,
		}: {},
		{
			CrossGroupObjectReference: unidlingapi.CrossGroupObjectReference{
				Kind: "ReplicationController",
				Name: "somerc2",
			},
			Replicas: 5,
		}: {},
		{
			CrossGroupObjectReference: unidlingapi.CrossGroupObjectReference{
				Kind: "DeploymentConfig",
				Name: "somedc1",
			},
			Replicas: 3,
		}: {},
		{
			CrossGroupObjectReference: unidlingapi.CrossGroupObjectReference{
				Kind: "DeploymentConfig",
				Name: "somedc2",
			},
			Replicas: 1,
		}: {},
	}

	if err != nil {
		t.Fatalf("Unexpected error while generating new annotation value: %v", err)
	}

	if len(newScaleRefs) != len(expectedScaleRefs) {
		t.Fatalf("Expected new recorded scale references of %#v, got %#v", expectedScaleRefs, newScaleRefs)
	}

	for _, scaleRef := range newScaleRefs {
		if _, wasPresent := expectedScaleRefs[scaleRef]; !wasPresent {
			t.Errorf("Unexpected recorded scale reference %#v found in the output", scaleRef)
		}
	}
}
