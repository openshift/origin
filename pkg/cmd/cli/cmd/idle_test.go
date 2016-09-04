package cmd

import (
	"encoding/json"
	"testing"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	unidlingapi "github.com/openshift/origin/pkg/unidling/api"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	kunversioned "k8s.io/kubernetes/pkg/api/unversioned"
	kruntime "k8s.io/kubernetes/pkg/runtime"
	ktypes "k8s.io/kubernetes/pkg/types"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/api/install"
)

func makePod(name, rcName string, t *testing.T) kapi.Pod {
	// this snippet is from kube's code to set the created-by annotation
	// (which itself does not do quite what we want here)

	codec := kapi.Codecs.LegacyCodec(kunversioned.GroupVersion{Group: kapi.GroupName, Version: "v1"})

	createdByRefJson, err := kruntime.Encode(codec, &kapi.SerializedReference{
		Reference: kapi.ObjectReference{
			Kind:      "ReplicationController",
			Name:      rcName,
			Namespace: "somens",
		},
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	return kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name:      name,
			Namespace: "somens",
			Annotations: map[string]string{
				kapi.CreatedByAnnotation: string(createdByRefJson),
			},
		},
	}
}

func makeRC(name, dcName, createdByDCName string, t *testing.T) *kapi.ReplicationController {
	rc := kapi.ReplicationController{
		ObjectMeta: kapi.ObjectMeta{
			Name:        name,
			Namespace:   "somens",
			Annotations: make(map[string]string),
		},
	}

	if createdByDCName != "" {
		codec := kapi.Codecs.LegacyCodec(kunversioned.GroupVersion{Group: kapi.GroupName, Version: "v1"})
		createdByRefJson, err := kruntime.Encode(codec, &kapi.SerializedReference{
			Reference: kapi.ObjectReference{
				Kind:      "DeploymentConfig",
				Name:      createdByDCName,
				Namespace: "somens",
			},
		})

		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		rc.Annotations[kapi.CreatedByAnnotation] = string(createdByRefJson)
	}

	if dcName != "" {
		rc.Annotations[deployapi.DeploymentConfigAnnotation] = dcName
	}

	return &rc
}

func makePodRef(name string) *kapi.ObjectReference {
	return &kapi.ObjectReference{
		Kind:      "Pod",
		Name:      name,
		Namespace: "somens",
	}
}

func makeRCRef(name string) *kapi.ObjectReference {
	return &kapi.ObjectReference{
		Kind:      "ReplicationController",
		Name:      name,
		Namespace: "somens",
	}
}

func TestFindIdlablesForEndpoints(t *testing.T) {
	endpoints := &kapi.Endpoints{
		Subsets: []kapi.EndpointSubset{
			{
				Addresses: []kapi.EndpointAddress{
					{
						TargetRef: makePodRef("somepod1"),
					},
					{
						TargetRef: makePodRef("somepod2"),
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
						TargetRef: makePodRef("somepod3"),
					},
					{
						TargetRef: makePodRef("somepod4"),
					},
					{
						TargetRef: makePodRef("somepod5"),
					},
					{
						TargetRef: makePodRef("missingpod"),
					},
				},
			},
		},
	}

	pods := map[kapi.ObjectReference]kapi.Pod{
		*makePodRef("somepod1"): makePod("somepod1", "somerc1", t),
		*makePodRef("somepod2"): makePod("somepod2", "somerc2", t),
		*makePodRef("somepod3"): makePod("somepod3", "somerc1", t),
		*makePodRef("somepod4"): makePod("somepod4", "somerc3", t),
		*makePodRef("somepod5"): makePod("somepod5", "somerc4", t),
	}

	getPod := func(ref kapi.ObjectReference) (*kapi.Pod, error) {
		if pod, ok := pods[ref]; ok {
			return &pod, nil
		}
		return nil, kerrors.NewNotFound(kunversioned.GroupResource{Group: kapi.GroupName, Resource: "Pod"}, ref.Name)
	}

	controllers := map[kapi.ObjectReference]kruntime.Object{
		// prefer CreatedByAnnotation to DeploymentConfigAnnotation
		*makeRCRef("somerc1"): makeRC("somerc1", "nonsense-value", "somedc1", t),
		*makeRCRef("somerc2"): makeRC("somerc2", "", "", t),
		*makeRCRef("somerc3"): makeRC("somerc3", "somedc2", "", t),
		*makeRCRef("somerc4"): makeRC("somerc4", "", "somedc2", t),
	}

	getController := func(ref kapi.ObjectReference) (kruntime.Object, error) {
		if controller, ok := controllers[ref]; ok {
			return controller, nil
		}

		// NB: this GroupResource declaration plays fast and loose with various distinctions
		// but is good enough for being an error in a test
		return nil, kerrors.NewNotFound(kunversioned.GroupResource{Group: kapi.GroupName, Resource: ref.Kind}, ref.Name)

	}

	codec := kapi.Codecs.LegacyCodec(kunversioned.GroupVersion{Group: kapi.GroupName, Version: "v1"})
	refSet, err := findScalableResourcesForEndpoints(endpoints, codec, getPod, getController)

	if err != nil {
		t.Fatalf("Unexpected error while finding idlables: %v", err)
	}

	expectedRefs := []unidlingapi.CrossGroupObjectReference{
		{
			Kind: "DeploymentConfig",
			Name: "somedc1",
		},
		{
			Kind: "DeploymentConfig",
			Name: "somedc2",
		},
		{
			Kind: "ReplicationController",
			Name: "somerc2",
		},
	}

	if len(refSet) != len(expectedRefs) {
		t.Errorf("Expected to get somedc1, somedc2, somerc2, instead got %#v", refSet)
	}

	for _, ref := range expectedRefs {
		if _, ok := refSet[ref]; !ok {
			t.Errorf("expected ReplicationController %q to be present, but was not", ref.Name)
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

	scales := map[unidlingapi.CrossGroupObjectReference]int32{
		{
			Kind: "ReplicationController",
			Name: "somerc1",
		}: 2,
		{
			Kind: "ReplicationController",
			Name: "somerc2",
		}: 5,
		{
			Kind: "DeploymentConfig",
			Name: "somedc1",
		}: 0,
		{
			Kind: "DeploymentConfig",
			Name: "somedc2",
		}: 0,
	}

	newScaleRefs, err := pairScalesWithScaleRefs(ktypes.NamespacedName{Name: "somesvc"}, oldAnnotations, newRawRefs, scales)

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
