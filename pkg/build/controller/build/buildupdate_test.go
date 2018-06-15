package build

import (
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
)

func TestBuildUpdateSetters(t *testing.T) {
	now := metav1.Now()
	tests := []struct {
		f             func(*buildUpdate)
		validateApply func(*buildapi.Build) bool
		expected      string
	}{
		{
			f: func(u *buildUpdate) {
				u.setPhase(buildapi.BuildPhaseCancelled)
			},
			validateApply: func(b *buildapi.Build) bool {
				return b.Status.Phase == buildapi.BuildPhaseCancelled
			},
			expected: "buildUpdate(phase: \"Cancelled\")",
		},
		{
			f: func(u *buildUpdate) {
				u.setReason(buildapi.StatusReasonCannotCreateBuildPodSpec)
			},
			validateApply: func(b *buildapi.Build) bool {
				return b.Status.Reason == buildapi.StatusReasonCannotCreateBuildPodSpec
			},
			expected: "buildUpdate(reason: \"CannotCreateBuildPodSpec\")",
		},
		{
			f: func(u *buildUpdate) {
				u.setMessage("hello")
			},
			validateApply: func(b *buildapi.Build) bool {
				return b.Status.Message == "hello"
			},
			expected: "buildUpdate(message: \"hello\")",
		},
		{
			f: func(u *buildUpdate) {
				u.setStartTime(now)
			},
			validateApply: func(b *buildapi.Build) bool {
				return (*b.Status.StartTimestamp) == now
			},
			expected: fmt.Sprintf("buildUpdate(startTime: \"%v\")", now),
		},
		{
			f: func(u *buildUpdate) {
				u.setCompletionTime(now)
			},
			validateApply: func(b *buildapi.Build) bool {
				return (*b.Status.CompletionTimestamp) == now
			},
			expected: fmt.Sprintf("buildUpdate(completionTime: \"%v\")", now),
		},
		{
			f: func(u *buildUpdate) {
				d := time.Duration(2 * time.Hour)
				u.setDuration(d)
			},
			validateApply: func(b *buildapi.Build) bool {
				return b.Status.Duration == time.Duration(2*time.Hour)
			},
			expected: fmt.Sprintf("buildUpdate(duration: \"%v\")", time.Duration(2*time.Hour)),
		},
		{
			f: func(u *buildUpdate) {
				u.setOutputRef("1234567890")
			},
			validateApply: func(b *buildapi.Build) bool {
				return b.Status.OutputDockerImageReference == "1234567890"
			},
			expected: fmt.Sprintf("buildUpdate(outputRef: %q)", "1234567890"),
		},
		{
			f: func(u *buildUpdate) {
				u.setPodNameAnnotation("test-pod-name")
			},
			validateApply: func(b *buildapi.Build) bool {
				return b.Annotations != nil && b.Annotations[buildapi.BuildPodNameAnnotation] == "test-pod-name"
			},
			expected: "buildUpdate(podName: \"test-pod-name\")",
		},
	}

	for _, test := range tests {
		buildUpdate := &buildUpdate{}
		test.f(buildUpdate)
		if actual := buildUpdate.String(); actual != test.expected {
			t.Errorf("Unexpected string: %s, expected: %s", actual, test.expected)
		}
		b := &buildapi.Build{}
		buildUpdate.apply(b)
		if !test.validateApply(b) {
			t.Errorf("Failed to apply update %v to build %#v", buildUpdate, b)
		}
	}
}

func TestBuildUpdateIsEmpty(t *testing.T) {
	update := &buildUpdate{}
	if !update.isEmpty() {
		t.Errorf("isEmpty returned false, expecting true")
	}
	update.setOutputRef("123456789")
	if update.isEmpty() {
		t.Errorf("isEmpty returned true, expecting false")
	}
	update.reset()
	if !update.isEmpty() {
		t.Errorf("isEmpty returned false, expecting true")
	}
}
