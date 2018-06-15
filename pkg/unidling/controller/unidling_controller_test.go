package controller

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	unidlingapi "github.com/openshift/origin/pkg/unidling/api"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsfake "github.com/openshift/origin/pkg/apps/generated/internalclientset/fake"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kexternalfake "k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/apis/autoscaling"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kinternalfake "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	// install the APIs we need for the codecs to run correctly in order to build patches
	_ "github.com/openshift/origin/pkg/api/install"
)

type fakeResults struct {
	resMap       map[unidlingapi.CrossGroupObjectReference]autoscaling.Scale
	resEndpoints *kapi.Endpoints
}

func prepFakeClient(t *testing.T, nowTime time.Time, scales ...autoscaling.Scale) (*kinternalfake.Clientset, *appsfake.Clientset, *fakeResults) {
	fakeClient := &kinternalfake.Clientset{}
	fakeDeployClient := &appsfake.Clientset{}

	nowTimeStr := nowTime.Format(time.RFC3339)

	targets := make([]unidlingapi.RecordedScaleReference, len(scales))
	for i, scale := range scales {
		targets[i] = unidlingapi.RecordedScaleReference{
			CrossGroupObjectReference: unidlingapi.CrossGroupObjectReference{
				Name: scale.Name,
				Kind: scale.Kind,
			},
			Replicas: 2,
		}
	}
	targetsAnnotation, err := json.Marshal(targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	endpointsObj := kapi.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name: "somesvc",
			Annotations: map[string]string{
				unidlingapi.IdledAtAnnotation:      nowTimeStr,
				unidlingapi.UnidleTargetAnnotation: string(targetsAnnotation),
			},
		},
	}
	fakeClient.PrependReactor("get", "endpoints", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		if action.(clientgotesting.GetAction).GetName() == endpointsObj.Name {
			return true, &endpointsObj, nil
		}

		return false, nil, nil
	})

	fakeDeployClient.PrependReactor("get", "deploymentconfigs", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		objName := action.(clientgotesting.GetAction).GetName()
		for _, scale := range scales {
			if scale.Kind == "DeploymentConfig" && objName == scale.Name {
				return true, &appsapi.DeploymentConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: objName,
					},
					Spec: appsapi.DeploymentConfigSpec{
						Replicas: scale.Spec.Replicas,
					},
				}, nil
			}
		}

		return true, nil, errors.NewNotFound(action.GetResource().GroupResource(), objName)
	})

	fakeClient.PrependReactor("get", "replicationcontrollers", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		objName := action.(clientgotesting.GetAction).GetName()
		for _, scale := range scales {
			if scale.Kind == "ReplicationController" && objName == scale.Name {
				return true, &kapi.ReplicationController{
					ObjectMeta: metav1.ObjectMeta{
						Name: objName,
					},
					Spec: kapi.ReplicationControllerSpec{
						Replicas: scale.Spec.Replicas,
					},
				}, nil
			}
		}

		return true, nil, errors.NewNotFound(action.GetResource().GroupResource(), objName)
	})

	res := &fakeResults{
		resMap: make(map[unidlingapi.CrossGroupObjectReference]autoscaling.Scale),
	}

	fakeDeployClient.PrependReactor("update", "deploymentconfigs", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		obj := action.(clientgotesting.UpdateAction).GetObject().(*appsapi.DeploymentConfig)
		for _, scale := range scales {
			if scale.Kind == "DeploymentConfig" && obj.Name == scale.Name {
				newScale := scale
				newScale.Spec.Replicas = obj.Spec.Replicas
				res.resMap[unidlingapi.CrossGroupObjectReference{Name: obj.Name, Kind: "DeploymentConfig", Group: appsapi.GroupName}] = newScale
				return true, &appsapi.DeploymentConfig{}, nil
			}
		}

		return true, nil, errors.NewNotFound(action.GetResource().GroupResource(), obj.Name)
	})

	fakeClient.PrependReactor("update", "replicationcontrollers", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		obj := action.(clientgotesting.UpdateAction).GetObject().(*kapi.ReplicationController)
		for _, scale := range scales {
			if scale.Kind == "ReplicationController" && obj.Name == scale.Name {
				newScale := scale
				newScale.Spec.Replicas = obj.Spec.Replicas
				res.resMap[unidlingapi.CrossGroupObjectReference{Name: obj.Name, Kind: "ReplicationController"}] = newScale
				return true, &kapi.ReplicationController{}, nil
			}
		}

		return true, nil, errors.NewNotFound(action.GetResource().GroupResource(), obj.Name)
	})

	fakeDeployClient.PrependReactor("patch", "deploymentconfigs", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		patchAction := action.(clientgotesting.PatchActionImpl)
		var patch appsapi.DeploymentConfig
		json.Unmarshal(patchAction.GetPatch(), &patch)

		for _, scale := range scales {
			if scale.Kind == "DeploymentConfig" && patchAction.GetName() == scale.Name {
				newScale := scale
				newScale.Spec.Replicas = patch.Spec.Replicas
				res.resMap[unidlingapi.CrossGroupObjectReference{Name: patchAction.GetName(), Kind: "DeploymentConfig", Group: appsapi.GroupName}] = newScale
				return true, &appsapi.DeploymentConfig{}, nil
			}
		}

		return true, nil, errors.NewNotFound(action.GetResource().GroupResource(), patchAction.GetName())
	})

	fakeClient.PrependReactor("patch", "replicationcontrollers", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		patchAction := action.(clientgotesting.PatchActionImpl)
		var patch kapi.ReplicationController
		json.Unmarshal(patchAction.GetPatch(), &patch)

		for _, scale := range scales {
			if scale.Kind == "ReplicationController" && patchAction.GetName() == scale.Name {
				newScale := scale
				newScale.Spec.Replicas = patch.Spec.Replicas
				res.resMap[unidlingapi.CrossGroupObjectReference{Name: patchAction.GetName(), Kind: "ReplicationController"}] = newScale
				return true, &kapi.ReplicationController{}, nil
			}
		}

		return true, nil, errors.NewNotFound(action.GetResource().GroupResource(), patchAction.GetName())
	})

	fakeClient.AddReactor("*", "endpoints", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		obj := action.(clientgotesting.UpdateAction).GetObject().(*kapi.Endpoints)
		if obj.Name != endpointsObj.Name {
			return false, nil, nil
		}

		res.resEndpoints = obj

		return true, obj, nil
	})

	return fakeClient, fakeDeployClient, res
}

func TestControllerHandlesStaleEvents(t *testing.T) {
	nowTime := time.Now().Truncate(time.Second)
	fakeClient, fakeDeployClient, res := prepFakeClient(t, nowTime)
	fakeExternalClient := kexternalfake.NewSimpleClientset()
	controller := &UnidlingController{
		scaleNamespacer:     fakeExternalClient.Extensions(),
		endpointsNamespacer: fakeClient.Core(),
		rcNamespacer:        fakeClient.Core(),
		dcNamespacer:        fakeDeployClient.Apps(),
	}

	retry, err := controller.handleRequest(types.NamespacedName{
		Namespace: "somens",
		Name:      "somesvc",
	}, nowTime.Add(-10*time.Second))

	if err != nil {
		t.Fatalf("Unable to unidle: unexpected error (retry: %v): %v", retry, err)
	}

	if len(res.resMap) != 0 {
		t.Errorf("Did not expect to have anything scaled, but got %v", res.resMap)
	}

	if res.resEndpoints != nil {
		t.Errorf("Did not expect to have endpoints object updated, but got %v", res.resEndpoints)
	}
}

func TestControllerIgnoresAlreadyScaledObjects(t *testing.T) {
	// truncate to avoid conversion comparison issues
	nowTime := time.Now().Truncate(time.Second)
	baseScales := []autoscaling.Scale{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "somerc",
			},
			TypeMeta: metav1.TypeMeta{
				Kind: "ReplicationController",
			},
			Spec: autoscaling.ScaleSpec{
				Replicas: 0,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "somedc",
			},
			TypeMeta: metav1.TypeMeta{
				Kind: "DeploymentConfig",
			},
			Spec: autoscaling.ScaleSpec{
				Replicas: 5,
			},
		},
	}

	idledTime := nowTime.Add(-10 * time.Second)
	fakeClient, fakeDeployClient, res := prepFakeClient(t, idledTime, baseScales...)
	fakeExternalClient := kexternalfake.NewSimpleClientset()

	controller := &UnidlingController{
		scaleNamespacer:     fakeExternalClient.Extensions(),
		endpointsNamespacer: fakeClient.Core(),
		rcNamespacer:        fakeClient.Core(),
		dcNamespacer:        fakeDeployClient.Apps(),
	}

	retry, err := controller.handleRequest(types.NamespacedName{
		Namespace: "somens",
		Name:      "somesvc",
	}, nowTime)

	if err != nil {
		t.Fatalf("Unable to unidle: unexpected error (retry: %v): %v", retry, err)
	}

	if len(res.resMap) != 1 {
		t.Errorf("Incorrect unidling results: got %v, expected to end up with 1 objects scaled to 1", res.resMap)
	}

	stillPresent := make(map[unidlingapi.CrossGroupObjectReference]struct{})

	for _, scale := range baseScales {
		scaleRef := unidlingapi.CrossGroupObjectReference{Kind: scale.Kind, Name: scale.Name}
		if scale.Kind == "DeploymentConfig" {
			scaleRef.Group = appsapi.GroupName
		}
		resScale, ok := res.resMap[scaleRef]
		if scale.Spec.Replicas != 0 {
			stillPresent[scaleRef] = struct{}{}
			if ok {
				t.Errorf("Expected to %s %q to not have been scaled, but it was scaled to %v", scale.Kind, scale.Name, resScale.Spec.Replicas)
			}
			continue
		} else if !ok {
			t.Errorf("Expected to %s %q to have been scaled, but it was not", scale.Kind, scale.Name)
			continue
		}

		if resScale.Spec.Replicas != 2 {
			t.Errorf("Expected %s %q to have been scaled to 2, but it was scaled to %v", scale.Kind, scale.Name, resScale.Spec.Replicas)
		}
	}

	if res.resEndpoints == nil {
		t.Fatalf("Expected endpoints object to be updated, but it was not")
	}

	resTargetsRaw, hadTargets := res.resEndpoints.Annotations[unidlingapi.UnidleTargetAnnotation]
	resIdledTimeRaw, hadIdledTime := res.resEndpoints.Annotations[unidlingapi.IdledAtAnnotation]

	if !hadTargets {
		t.Errorf("Expected targets annotation to still be present, but it was not")
	}
	var resTargets []unidlingapi.RecordedScaleReference
	if err = json.Unmarshal([]byte(resTargetsRaw), &resTargets); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(resTargets) != len(stillPresent) {
		t.Errorf("Expected the new target list to contain the unscaled scalables only, but it was %v", resTargets)
	}
	for _, target := range resTargets {
		if target.Kind == "DeploymentConfig" {
			target.CrossGroupObjectReference.Group = appsapi.GroupName
		}
		if _, ok := stillPresent[target.CrossGroupObjectReference]; !ok {
			t.Errorf("Expected new target list to contain the unscaled scalables only, but it was %v", resTargets)
		}
	}

	if !hadIdledTime {
		t.Errorf("Expected idled-at annotation to still be present, but it was not")
	}
	resIdledTime, err := time.Parse(time.RFC3339, resIdledTimeRaw)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !resIdledTime.Equal(idledTime) {
		t.Errorf("Expected output idled time annotation to be %s, but was changed to %s", idledTime, resIdledTime)
	}
}

func TestControllerUnidlesProperly(t *testing.T) {
	nowTime := time.Now().Truncate(time.Second)
	baseScales := []autoscaling.Scale{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "somerc",
			},
			TypeMeta: metav1.TypeMeta{
				Kind: "ReplicationController",
			},
			Spec: autoscaling.ScaleSpec{
				Replicas: 0,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "somedc",
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "DeploymentConfig",
				APIVersion: "apps.openshift.io/v1",
			},
			Spec: autoscaling.ScaleSpec{
				Replicas: 0,
			},
		},
	}

	fakeClient, fakeDeployClient, res := prepFakeClient(t, nowTime.Add(-10*time.Second), baseScales...)
	fakeExternalClient := kexternalfake.NewSimpleClientset()

	controller := &UnidlingController{
		scaleNamespacer:     fakeExternalClient.Extensions(),
		endpointsNamespacer: fakeClient.Core(),
		rcNamespacer:        fakeClient.Core(),
		dcNamespacer:        fakeDeployClient.Apps(),
	}

	retry, err := controller.handleRequest(types.NamespacedName{
		Namespace: "somens",
		Name:      "somesvc",
	}, nowTime)

	if err != nil {
		t.Fatalf("Unable to unidle: unexpected error (retry: %v): %v", retry, err)
	}

	if len(res.resMap) != len(baseScales) {
		t.Errorf("Incorrect unidling results: got %v, expected to end up with %v objects scaled to 1", res.resMap, len(baseScales))
	}

	for _, scale := range baseScales {
		ref := unidlingapi.CrossGroupObjectReference{Kind: scale.Kind, Name: scale.Name}
		if scale.Kind == "DeploymentConfig" {
			ref.Group = appsapi.GroupName
		}
		resScale, ok := res.resMap[ref]
		if !ok {
			t.Errorf("Expected to %s %q to have been scaled, but it was not", scale.Kind, scale.Name)
			continue
		}

		if resScale.Spec.Replicas != 2 {
			t.Errorf("Expected %s %q to have been scaled to 2, but it was scaled to %v", scale.Kind, scale.Name, resScale.Spec.Replicas)
		}
	}

	if res.resEndpoints == nil {
		t.Fatalf("Expected endpoints object to be updated, but it was not")
	}

	resTargets, hadTargets := res.resEndpoints.Annotations[unidlingapi.UnidleTargetAnnotation]
	resIdledTime, hadIdledTime := res.resEndpoints.Annotations[unidlingapi.IdledAtAnnotation]

	if hadTargets {
		t.Errorf("Expected targets annotation to be removed, but it was %q", resTargets)
	}

	if hadIdledTime {
		t.Errorf("Expected idled-at annotation to be removed, but it was %q", resIdledTime)
	}
}

type failureTestInfo struct {
	name                   string
	endpointsGet           *kapi.Endpoints
	scaleGets              []autoscaling.Scale
	scaleUpdatesNotFound   []bool
	preventEndpointsUpdate bool

	errorExpected       bool
	retryExpected       bool
	annotationsExpected map[string]string
}

func prepareFakeClientForFailureTest(test failureTestInfo) (*kinternalfake.Clientset, *appsfake.Clientset) {
	fakeClient := &kinternalfake.Clientset{}
	fakeDeployClient := &appsfake.Clientset{}

	fakeClient.PrependReactor("get", "endpoints", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		objName := action.(clientgotesting.GetAction).GetName()
		if test.endpointsGet != nil && objName == test.endpointsGet.Name {
			return true, test.endpointsGet, nil
		}

		return true, nil, errors.NewNotFound(action.GetResource().GroupResource(), objName)
	})

	fakeDeployClient.PrependReactor("get", "deploymentconfigs", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		objName := action.(clientgotesting.GetAction).GetName()
		for _, scale := range test.scaleGets {
			if scale.Kind == "DeploymentConfig" && objName == scale.Name {
				return true, &appsapi.DeploymentConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: objName,
					},
					Spec: appsapi.DeploymentConfigSpec{
						Replicas: scale.Spec.Replicas,
					},
				}, nil
			}
		}

		return true, nil, errors.NewNotFound(action.GetResource().GroupResource(), objName)
	})

	fakeClient.PrependReactor("get", "replicationcontrollers", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		objName := action.(clientgotesting.GetAction).GetName()
		for _, scale := range test.scaleGets {
			if scale.Kind == "ReplicationController" && objName == scale.Name {
				return true, &kapi.ReplicationController{
					ObjectMeta: metav1.ObjectMeta{
						Name: objName,
					},
					Spec: kapi.ReplicationControllerSpec{
						Replicas: scale.Spec.Replicas,
					},
				}, nil
			}
		}

		return true, nil, errors.NewNotFound(action.GetResource().GroupResource(), objName)
	})

	fakeDeployClient.PrependReactor("update", "deploymentconfigs", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		obj := action.(clientgotesting.UpdateAction).GetObject().(*appsapi.DeploymentConfig)
		for i, scale := range test.scaleGets {
			if scale.Kind == "DeploymentConfig" && obj.Name == scale.Name {
				if test.scaleUpdatesNotFound != nil && test.scaleUpdatesNotFound[i] {
					return false, nil, nil
				}

				return true, &appsapi.DeploymentConfig{}, nil
			}
		}

		return true, nil, errors.NewNotFound(action.GetResource().GroupResource(), obj.Name)
	})

	fakeClient.PrependReactor("update", "replicationcontrollers", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		obj := action.(clientgotesting.UpdateAction).GetObject().(*kapi.ReplicationController)
		for i, scale := range test.scaleGets {
			if scale.Kind == "ReplicationController" && obj.Name == scale.Name {
				if test.scaleUpdatesNotFound != nil && test.scaleUpdatesNotFound[i] {
					return false, nil, nil
				}
				return true, &kapi.ReplicationController{}, nil
			}
		}

		return true, nil, errors.NewNotFound(action.GetResource().GroupResource(), obj.Name)
	})

	fakeClient.PrependReactor("update", "endpoints", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		obj := action.(clientgotesting.UpdateAction).GetObject().(*kapi.Endpoints)
		if obj.Name != test.endpointsGet.Name {
			return false, nil, nil
		}

		if test.preventEndpointsUpdate {
			return true, nil, fmt.Errorf("some problem updating the endpoints")
		}

		return true, obj, nil
	})

	return fakeClient, fakeDeployClient
}

func TestControllerPerformsCorrectlyOnFailures(t *testing.T) {
	nowTime := time.Now().Truncate(time.Second)

	baseScalables := []unidlingapi.RecordedScaleReference{
		{
			CrossGroupObjectReference: unidlingapi.CrossGroupObjectReference{
				Kind: "ReplicationController",
				Name: "somerc",
			},
			Replicas: 2,
		},
		{
			CrossGroupObjectReference: unidlingapi.CrossGroupObjectReference{
				Kind:  "DeploymentConfig",
				Group: appsapi.GroupName,
				Name:  "somedc",
			},
			Replicas: 2,
		},
	}
	baseScalablesBytes, err := json.Marshal(baseScalables)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	outScalables := []unidlingapi.RecordedScaleReference{
		{
			CrossGroupObjectReference: unidlingapi.CrossGroupObjectReference{
				Kind:  "DeploymentConfig",
				Group: appsapi.GroupName,
				Name:  "somedc",
			},
			Replicas: 2,
		},
	}
	outScalablesBytes, err := json.Marshal(outScalables)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	tests := []failureTestInfo{
		{
			name:          "retry on failed endpoints get",
			endpointsGet:  nil,
			errorExpected: true,
			retryExpected: true,
		},
		{
			name: "not retry on failure to parse time",
			endpointsGet: &kapi.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name: "somesvc",
					Annotations: map[string]string{
						unidlingapi.IdledAtAnnotation: "cheddar",
					},
				},
			},
			errorExpected: true,
			retryExpected: false,
		},
		{
			name: "not retry on failure to unmarshal target scalables",
			endpointsGet: &kapi.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name: "somesvc",
					Annotations: map[string]string{
						unidlingapi.IdledAtAnnotation:      nowTime.Format(time.RFC3339),
						unidlingapi.UnidleTargetAnnotation: "pecorino romano",
					},
				},
			},
			errorExpected: true,
			retryExpected: false,
		},
		{
			name: "remove a scalable from the list if it cannot be found (while getting)",
			endpointsGet: &kapi.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name: "somesvc",
					Annotations: map[string]string{
						unidlingapi.IdledAtAnnotation:      nowTime.Format(time.RFC3339),
						unidlingapi.UnidleTargetAnnotation: string(baseScalablesBytes),
					},
				},
			},
			scaleGets: []autoscaling.Scale{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "DeploymentConfig",
						APIVersion: "apps.openshift.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "somedc",
					},
					Spec: autoscaling.ScaleSpec{Replicas: 0},
				},
			},
			errorExpected: false,
			annotationsExpected: map[string]string{
				unidlingapi.IdledAtAnnotation:      nowTime.Format(time.RFC3339),
				unidlingapi.UnidleTargetAnnotation: string(outScalablesBytes),
			},
		},
		{
			name: "should remove a scalable from the list if it cannot be found (while updating)",
			endpointsGet: &kapi.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name: "somesvc",
					Annotations: map[string]string{
						unidlingapi.IdledAtAnnotation:      nowTime.Format(time.RFC3339),
						unidlingapi.UnidleTargetAnnotation: string(baseScalablesBytes),
					},
				},
			},
			scaleGets: []autoscaling.Scale{
				{
					TypeMeta: metav1.TypeMeta{
						Kind: "ReplicationController",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "somerc",
					},
					Spec: autoscaling.ScaleSpec{Replicas: 0},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "DeploymentConfig",
						APIVersion: "apps.openshift.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "somedc",
					},
					Spec: autoscaling.ScaleSpec{Replicas: 0},
				},
			},
			scaleUpdatesNotFound: []bool{false, true},
			errorExpected:        false,
			annotationsExpected: map[string]string{
				unidlingapi.IdledAtAnnotation:      nowTime.Format(time.RFC3339),
				unidlingapi.UnidleTargetAnnotation: string(outScalablesBytes),
			},
		},
		{
			name: "retry on failed endpoints update",
			endpointsGet: &kapi.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name: "somesvc",
					Annotations: map[string]string{
						unidlingapi.IdledAtAnnotation:      nowTime.Format(time.RFC3339),
						unidlingapi.UnidleTargetAnnotation: string(baseScalablesBytes),
					},
				},
			},
			scaleGets: []autoscaling.Scale{
				{
					TypeMeta: metav1.TypeMeta{
						Kind: "ReplicationController",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "somerc",
					},
					Spec: autoscaling.ScaleSpec{Replicas: 0},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "DeploymentConfig",
						APIVersion: "apps.openshift.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "somedc",
					},
					Spec: autoscaling.ScaleSpec{Replicas: 0},
				},
			},
			preventEndpointsUpdate: true,
			errorExpected:          true,
			retryExpected:          true,
		},
	}

	for _, test := range tests {
		fakeClient, fakeDeployClient := prepareFakeClientForFailureTest(test)
		fakeExternalClient := kexternalfake.NewSimpleClientset()
		controller := &UnidlingController{
			scaleNamespacer:     fakeExternalClient.Extensions(),
			endpointsNamespacer: fakeClient.Core(),
			rcNamespacer:        fakeClient.Core(),
			dcNamespacer:        fakeDeployClient.Apps(),
		}

		var retry bool
		retry, err = controller.handleRequest(types.NamespacedName{
			Namespace: "somens",
			Name:      "somesvc",
		}, nowTime.Add(10*time.Second))

		if err != nil && !test.errorExpected {
			t.Errorf("for test 'it should %s': unexpected error while idling: %v", test.name, err)
			continue
		}

		if err == nil && test.errorExpected {
			t.Errorf("for test 'it should %s': expected error, but did not get one", test.name)
			continue
		}

		if test.errorExpected && (test.retryExpected != retry) {
			t.Errorf("for test 'it should %s': expected retry to be %v, but it was %v with error %v", test.name, test.retryExpected, retry, err)
			return
		}
	}
}
