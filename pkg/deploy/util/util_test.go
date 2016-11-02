package util

import (
	"reflect"
	"sort"
	"strconv"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployv1 "github.com/openshift/origin/pkg/deploy/api/v1"

	_ "github.com/openshift/origin/pkg/api/install"
)

func podTemplateA() *kapi.PodTemplateSpec {
	t := deploytest.OkPodTemplate()
	t.Spec.Containers = append(t.Spec.Containers, kapi.Container{
		Name:  "container1",
		Image: "registry:8080/repo1:ref1",
	})
	return t
}

func podTemplateB() *kapi.PodTemplateSpec {
	t := podTemplateA()
	t.Labels = map[string]string{"c": "d"}
	return t
}

func podTemplateC() *kapi.PodTemplateSpec {
	t := podTemplateA()
	t.Spec.Containers[0] = kapi.Container{
		Name:  "container2",
		Image: "registry:8080/repo1:ref3",
	}

	return t
}

func podTemplateD() *kapi.PodTemplateSpec {
	t := podTemplateA()
	t.Spec.Containers = append(t.Spec.Containers, kapi.Container{
		Name:  "container2",
		Image: "registry:8080/repo1:ref4",
	})

	return t
}

func TestPodName(t *testing.T) {
	deployment := &kapi.ReplicationController{
		ObjectMeta: kapi.ObjectMeta{
			Name: "testName",
		},
	}
	expected := "testName-deploy"
	actual := DeployerPodNameForDeployment(deployment.Name)
	if expected != actual {
		t.Errorf("Unexpected pod name for deployment. Expected: %s Got: %s", expected, actual)
	}
}

func TestMakeDeploymentOk(t *testing.T) {
	config := deploytest.OkDeploymentConfig(1)
	deployment, err := MakeDeployment(config, kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion))

	if err != nil {
		t.Fatalf("unexpected error: %#v", err)
	}

	expectedAnnotations := map[string]string{
		deployapi.DeploymentConfigAnnotation:  config.Name,
		deployapi.DeploymentStatusAnnotation:  string(deployapi.DeploymentStatusNew),
		deployapi.DeploymentVersionAnnotation: strconv.FormatInt(config.Status.LatestVersion, 10),
	}

	for key, expected := range expectedAnnotations {
		if actual := deployment.Annotations[key]; actual != expected {
			t.Fatalf("expected deployment annotation %s=%s, got %s", key, expected, actual)
		}
	}

	expectedAnnotations = map[string]string{
		deployapi.DeploymentAnnotation:        deployment.Name,
		deployapi.DeploymentConfigAnnotation:  config.Name,
		deployapi.DeploymentVersionAnnotation: strconv.FormatInt(config.Status.LatestVersion, 10),
	}

	for key, expected := range expectedAnnotations {
		if actual := deployment.Spec.Template.Annotations[key]; actual != expected {
			t.Fatalf("expected pod template annotation %s=%s, got %s", key, expected, actual)
		}
	}

	if len(EncodedDeploymentConfigFor(deployment)) == 0 {
		t.Fatalf("expected deployment with DeploymentEncodedConfigAnnotation annotation")
	}

	if decodedConfig, err := DecodeDeploymentConfig(deployment, kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion)); err != nil {
		t.Fatalf("invalid encoded config on deployment: %v", err)
	} else {
		if e, a := config.Name, decodedConfig.Name; e != a {
			t.Fatalf("encoded config name doesn't match source config")
		}
		// TODO: more assertions
	}

	if deployment.Spec.Replicas != 0 {
		t.Fatalf("expected deployment replicas to be 0")
	}

	if l, e, a := deployapi.DeploymentConfigAnnotation, config.Name, deployment.Labels[deployapi.DeploymentConfigAnnotation]; e != a {
		t.Fatalf("expected label %s=%s, got %s", l, e, a)
	}

	if e, a := config.Name, deployment.Spec.Template.Labels[deployapi.DeploymentConfigLabel]; e != a {
		t.Fatalf("expected label DeploymentConfigLabel=%s, got %s", e, a)
	}

	if e, a := deployment.Name, deployment.Spec.Template.Labels[deployapi.DeploymentLabel]; e != a {
		t.Fatalf("expected label DeploymentLabel=%s, got %s", e, a)
	}

	if e, a := config.Name, deployment.Spec.Selector[deployapi.DeploymentConfigLabel]; e != a {
		t.Fatalf("expected selector DeploymentConfigLabel=%s, got %s", e, a)
	}

	if e, a := deployment.Name, deployment.Spec.Selector[deployapi.DeploymentLabel]; e != a {
		t.Fatalf("expected selector DeploymentLabel=%s, got %s", e, a)
	}
}

func TestDeploymentsByLatestVersion_sorting(t *testing.T) {
	mkdeployment := func(version int64) kapi.ReplicationController {
		deployment, _ := MakeDeployment(deploytest.OkDeploymentConfig(version), kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion))
		return *deployment
	}
	deployments := []kapi.ReplicationController{
		mkdeployment(4),
		mkdeployment(1),
		mkdeployment(2),
		mkdeployment(3),
	}
	sort.Sort(ByLatestVersionAsc(deployments))
	for i := int64(0); i < 4; i++ {
		if e, a := i+1, DeploymentVersionFor(&deployments[i]); e != a {
			t.Errorf("expected deployment[%d]=%d, got %d", i, e, a)
		}
	}
	sort.Sort(ByLatestVersionDesc(deployments))
	for i := int64(0); i < 4; i++ {
		if e, a := 4-i, DeploymentVersionFor(&deployments[i]); e != a {
			t.Errorf("expected deployment[%d]=%d, got %d", i, e, a)
		}
	}
}

// TestSort verifies that builds are sorted by most recently created
func TestSort(t *testing.T) {
	present := unversioned.Now()
	past := unversioned.NewTime(present.Time.Add(-1 * time.Minute))
	controllers := []*kapi.ReplicationController{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:              "past",
				CreationTimestamp: past,
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:              "present",
				CreationTimestamp: present,
			},
		},
	}
	sort.Sort(ByMostRecent(controllers))
	if controllers[0].Name != "present" {
		t.Errorf("Unexpected sort order")
	}
	if controllers[1].Name != "past" {
		t.Errorf("Unexpected sort order")
	}
}

func TestCanTransitionPhase(t *testing.T) {
	tests := []struct {
		name          string
		current, next deployapi.DeploymentStatus
		expected      bool
	}{
		{
			name:     "New->New",
			current:  deployapi.DeploymentStatusNew,
			next:     deployapi.DeploymentStatusNew,
			expected: false,
		},
		{
			name:     "New->Pending",
			current:  deployapi.DeploymentStatusNew,
			next:     deployapi.DeploymentStatusPending,
			expected: true,
		},
		{
			name:     "New->Running",
			current:  deployapi.DeploymentStatusNew,
			next:     deployapi.DeploymentStatusRunning,
			expected: true,
		},
		{
			name:     "New->Complete",
			current:  deployapi.DeploymentStatusNew,
			next:     deployapi.DeploymentStatusComplete,
			expected: true,
		},
		{
			name:     "New->Failed",
			current:  deployapi.DeploymentStatusNew,
			next:     deployapi.DeploymentStatusFailed,
			expected: true,
		},
		{
			name:     "Pending->New",
			current:  deployapi.DeploymentStatusPending,
			next:     deployapi.DeploymentStatusNew,
			expected: false,
		},
		{
			name:     "Pending->Pending",
			current:  deployapi.DeploymentStatusPending,
			next:     deployapi.DeploymentStatusPending,
			expected: false,
		},
		{
			name:     "Pending->Running",
			current:  deployapi.DeploymentStatusPending,
			next:     deployapi.DeploymentStatusRunning,
			expected: true,
		},
		{
			name:     "Pending->Failed",
			current:  deployapi.DeploymentStatusPending,
			next:     deployapi.DeploymentStatusFailed,
			expected: true,
		},
		{
			name:     "Pending->Complete",
			current:  deployapi.DeploymentStatusPending,
			next:     deployapi.DeploymentStatusComplete,
			expected: true,
		},
		{
			name:     "Running->New",
			current:  deployapi.DeploymentStatusRunning,
			next:     deployapi.DeploymentStatusNew,
			expected: false,
		},
		{
			name:     "Running->Pending",
			current:  deployapi.DeploymentStatusRunning,
			next:     deployapi.DeploymentStatusPending,
			expected: false,
		},
		{
			name:     "Running->Running",
			current:  deployapi.DeploymentStatusRunning,
			next:     deployapi.DeploymentStatusRunning,
			expected: false,
		},
		{
			name:     "Running->Failed",
			current:  deployapi.DeploymentStatusRunning,
			next:     deployapi.DeploymentStatusFailed,
			expected: true,
		},
		{
			name:     "Running->Complete",
			current:  deployapi.DeploymentStatusRunning,
			next:     deployapi.DeploymentStatusComplete,
			expected: true,
		},
		{
			name:     "Complete->New",
			current:  deployapi.DeploymentStatusComplete,
			next:     deployapi.DeploymentStatusNew,
			expected: false,
		},
		{
			name:     "Complete->Pending",
			current:  deployapi.DeploymentStatusComplete,
			next:     deployapi.DeploymentStatusPending,
			expected: false,
		},
		{
			name:     "Complete->Running",
			current:  deployapi.DeploymentStatusComplete,
			next:     deployapi.DeploymentStatusRunning,
			expected: false,
		},
		{
			name:     "Complete->Failed",
			current:  deployapi.DeploymentStatusComplete,
			next:     deployapi.DeploymentStatusFailed,
			expected: false,
		},
		{
			name:     "Complete->Complete",
			current:  deployapi.DeploymentStatusComplete,
			next:     deployapi.DeploymentStatusComplete,
			expected: false,
		},
		{
			name:     "Failed->New",
			current:  deployapi.DeploymentStatusFailed,
			next:     deployapi.DeploymentStatusNew,
			expected: false,
		},
		{
			name:     "Failed->Pending",
			current:  deployapi.DeploymentStatusFailed,
			next:     deployapi.DeploymentStatusPending,
			expected: false,
		},
		{
			name:     "Failed->Running",
			current:  deployapi.DeploymentStatusFailed,
			next:     deployapi.DeploymentStatusRunning,
			expected: false,
		},
		{
			name:     "Failed->Complete",
			current:  deployapi.DeploymentStatusFailed,
			next:     deployapi.DeploymentStatusComplete,
			expected: false,
		},
		{
			name:     "Failed->Failed",
			current:  deployapi.DeploymentStatusFailed,
			next:     deployapi.DeploymentStatusFailed,
			expected: false,
		},
	}

	for _, test := range tests {
		got := CanTransitionPhase(test.current, test.next)
		if got != test.expected {
			t.Errorf("%s: expected %t, got %t", test.name, test.expected, got)
		}
	}
}

var (
	now   = unversioned.Now()
	later = unversioned.Time{Time: now.Add(time.Minute)}

	condProgressing = func() deployapi.DeploymentCondition {
		return deployapi.DeploymentCondition{
			Type:               deployapi.DeploymentProgressing,
			Status:             kapi.ConditionTrue,
			LastTransitionTime: now,
			Reason:             "ForSomeReason",
		}
	}

	condProgressingDifferentTime = func() deployapi.DeploymentCondition {
		return deployapi.DeploymentCondition{
			Type:               deployapi.DeploymentProgressing,
			Status:             kapi.ConditionTrue,
			LastTransitionTime: later,
			Reason:             "ForSomeReason",
		}
	}

	condProgressingDifferentReason = func() deployapi.DeploymentCondition {
		return deployapi.DeploymentCondition{
			Type:               deployapi.DeploymentProgressing,
			Status:             kapi.ConditionTrue,
			LastTransitionTime: later,
			Reason:             "BecauseItIs",
		}
	}

	condNotProgressing = func() deployapi.DeploymentCondition {
		return deployapi.DeploymentCondition{
			Type:   deployapi.DeploymentProgressing,
			Status: kapi.ConditionFalse,
			Reason: "NotYet",
		}
	}

	condAvailable = func() deployapi.DeploymentCondition {
		return deployapi.DeploymentCondition{
			Type:   deployapi.DeploymentAvailable,
			Status: kapi.ConditionTrue,
			Reason: "AwesomeController",
		}
	}
)

func TestGetCondition(t *testing.T) {
	exampleStatus := func() deployapi.DeploymentConfigStatus {
		return deployapi.DeploymentConfigStatus{
			Conditions: []deployapi.DeploymentCondition{condProgressing(), condAvailable()},
		}
	}

	tests := []struct {
		name string

		status     deployapi.DeploymentConfigStatus
		condType   deployapi.DeploymentConditionType
		condStatus kapi.ConditionStatus
		condReason string

		expected bool
	}{
		{
			name: "condition exists",

			status:   exampleStatus(),
			condType: deployapi.DeploymentAvailable,

			expected: true,
		},
		{
			name: "condition does not exist",

			status:   exampleStatus(),
			condType: deployapi.DeploymentReplicaFailure,

			expected: false,
		},
	}

	for _, test := range tests {
		cond := GetDeploymentCondition(test.status, test.condType)
		exists := cond != nil
		if exists != test.expected {
			t.Errorf("%s: expected condition to exist: %t, got: %t", test.name, test.expected, exists)
		}
	}
}

func TestSetCondition(t *testing.T) {
	tests := []struct {
		name string

		status *deployapi.DeploymentConfigStatus
		cond   deployapi.DeploymentCondition

		expectedStatus *deployapi.DeploymentConfigStatus
	}{
		{
			name: "set for the first time",

			status: &deployapi.DeploymentConfigStatus{},
			cond:   condAvailable(),

			expectedStatus: &deployapi.DeploymentConfigStatus{
				Conditions: []deployapi.DeploymentCondition{
					condAvailable(),
				},
			},
		},
		{
			name: "simple set",

			status: &deployapi.DeploymentConfigStatus{
				Conditions: []deployapi.DeploymentCondition{
					condProgressing(),
				},
			},
			cond: condAvailable(),

			expectedStatus: &deployapi.DeploymentConfigStatus{
				Conditions: []deployapi.DeploymentCondition{
					condProgressing(), condAvailable(),
				},
			},
		},
		{
			name: "replace if status changes",

			status: &deployapi.DeploymentConfigStatus{
				Conditions: []deployapi.DeploymentCondition{
					condNotProgressing(),
				},
			},
			cond: condProgressing(),

			expectedStatus: &deployapi.DeploymentConfigStatus{Conditions: []deployapi.DeploymentCondition{condProgressing()}},
		},
		{
			name: "replace if reason changes",

			status: &deployapi.DeploymentConfigStatus{
				Conditions: []deployapi.DeploymentCondition{
					condProgressing(),
				},
			},
			cond: condProgressingDifferentReason(),

			expectedStatus: &deployapi.DeploymentConfigStatus{
				Conditions: []deployapi.DeploymentCondition{
					{
						Type:   deployapi.DeploymentProgressing,
						Status: kapi.ConditionTrue,
						// Note that LastTransitionTime stays the same.
						LastTransitionTime: now,
						// Only the reason changes.
						Reason: "BecauseItIs",
					},
				},
			},
		},
		{
			name: "don't replace if status and reason don't change",

			status: &deployapi.DeploymentConfigStatus{
				Conditions: []deployapi.DeploymentCondition{
					condProgressing(),
				},
			},
			cond: condProgressingDifferentTime(),

			expectedStatus: &deployapi.DeploymentConfigStatus{Conditions: []deployapi.DeploymentCondition{condProgressing()}},
		},
	}

	for _, test := range tests {
		SetDeploymentCondition(test.status, test.cond)
		if !reflect.DeepEqual(test.status, test.expectedStatus) {
			t.Errorf("%s: expected status: %v, got: %v", test.name, test.expectedStatus, test.status)
		}
	}
}

func TestRemoveCondition(t *testing.T) {
	exampleStatus := func() *deployapi.DeploymentConfigStatus {
		return &deployapi.DeploymentConfigStatus{
			Conditions: []deployapi.DeploymentCondition{condProgressing(), condAvailable()},
		}
	}

	tests := []struct {
		name string

		status   *deployapi.DeploymentConfigStatus
		condType deployapi.DeploymentConditionType

		expectedStatus *deployapi.DeploymentConfigStatus
	}{
		{
			name: "remove from empty status",

			status:   &deployapi.DeploymentConfigStatus{},
			condType: deployapi.DeploymentProgressing,

			expectedStatus: &deployapi.DeploymentConfigStatus{},
		},
		{
			name: "simple remove",

			status:   &deployapi.DeploymentConfigStatus{Conditions: []deployapi.DeploymentCondition{condProgressing()}},
			condType: deployapi.DeploymentProgressing,

			expectedStatus: &deployapi.DeploymentConfigStatus{},
		},
		{
			name: "doesn't remove anything",

			status:   exampleStatus(),
			condType: deployapi.DeploymentReplicaFailure,

			expectedStatus: exampleStatus(),
		},
	}

	for _, test := range tests {
		RemoveDeploymentCondition(test.status, test.condType)
		if !reflect.DeepEqual(test.status, test.expectedStatus) {
			t.Errorf("%s: expected status: %v, got: %v", test.name, test.expectedStatus, test.status)
		}
	}
}
