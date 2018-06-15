package build

import (
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/build/controller/common"
)

// buildUpdate holds a set of updates to be made to a build object.
// Only the fields defined in this struct will be updated/patched by this controller.
// The reason this exists is that there isn't separation at the API
// level between build spec and build status. Once that happens, the
// controller should only be able to update the status, while end users
// should be able to update the spec.
type buildUpdate struct {
	podNameAnnotation *string
	phase             *buildapi.BuildPhase
	reason            *buildapi.StatusReason
	message           *string
	startTime         *metav1.Time
	completionTime    *metav1.Time
	duration          *time.Duration
	outputRef         *string
	logSnippet        *string
	pushSecret        *kapi.LocalObjectReference
}

func (u *buildUpdate) setPhase(phase buildapi.BuildPhase) {
	u.phase = &phase
}

func (u *buildUpdate) setReason(reason buildapi.StatusReason) {
	u.reason = &reason
}

func (u *buildUpdate) setMessage(message string) {
	u.message = &message
}

func (u *buildUpdate) setStartTime(startTime metav1.Time) {
	u.startTime = &startTime
}

func (u *buildUpdate) setCompletionTime(completionTime metav1.Time) {
	u.completionTime = &completionTime
}

func (u *buildUpdate) setDuration(duration time.Duration) {
	u.duration = &duration
}

func (u *buildUpdate) setOutputRef(ref string) {
	u.outputRef = &ref
}

func (u *buildUpdate) setPodNameAnnotation(podName string) {
	u.podNameAnnotation = &podName
}

func (u *buildUpdate) setLogSnippet(message string) {
	u.logSnippet = &message
}

func (u *buildUpdate) setPushSecret(pushSecret kapi.LocalObjectReference) {
	u.pushSecret = &pushSecret
}

func (u *buildUpdate) reset() {
	u.podNameAnnotation = nil
	u.phase = nil
	u.reason = nil
	u.message = nil
	u.startTime = nil
	u.completionTime = nil
	u.duration = nil
	u.outputRef = nil
	u.logSnippet = nil
	u.pushSecret = nil
}

func (u *buildUpdate) isEmpty() bool {
	return u.podNameAnnotation == nil &&
		u.phase == nil &&
		u.reason == nil &&
		u.message == nil &&
		u.startTime == nil &&
		u.completionTime == nil &&
		u.duration == nil &&
		u.outputRef == nil &&
		u.logSnippet == nil &&
		u.pushSecret == nil
}

func (u *buildUpdate) apply(build *buildapi.Build) {
	if u.phase != nil {
		build.Status.Phase = *u.phase
	}
	if u.reason != nil {
		build.Status.Reason = *u.reason
	}
	if u.message != nil {
		build.Status.Message = *u.message
	}
	if u.startTime != nil {
		build.Status.StartTimestamp = u.startTime
	}
	if u.completionTime != nil {
		build.Status.CompletionTimestamp = u.completionTime
	}
	if u.duration != nil {
		build.Status.Duration = *u.duration
	}
	if u.podNameAnnotation != nil {
		common.SetBuildPodNameAnnotation(build, *u.podNameAnnotation)
	}
	if u.outputRef != nil {
		build.Status.OutputDockerImageReference = *u.outputRef
	}
	if u.logSnippet != nil {
		build.Status.LogSnippet = *u.logSnippet
	}
	if u.pushSecret != nil {
		build.Spec.Output.PushSecret = u.pushSecret
	}
}

// String returns a string representation of this update
// Used with %v in string formatting
func (u *buildUpdate) String() string {
	updates := []string{}
	if u.phase != nil {
		updates = append(updates, fmt.Sprintf("phase: %q", *u.phase))
	}
	if u.reason != nil {
		updates = append(updates, fmt.Sprintf("reason: %q", *u.reason))
	}
	if u.message != nil {
		updates = append(updates, fmt.Sprintf("message: %q", *u.message))
	}
	if u.startTime != nil {
		updates = append(updates, fmt.Sprintf("startTime: %q", u.startTime.String()))
	}
	if u.completionTime != nil {
		updates = append(updates, fmt.Sprintf("completionTime: %q", u.completionTime.String()))
	}
	if u.duration != nil {
		updates = append(updates, fmt.Sprintf("duration: %q", u.duration.String()))
	}
	if u.outputRef != nil {
		updates = append(updates, fmt.Sprintf("outputRef: %q", *u.outputRef))
	}
	if u.podNameAnnotation != nil {
		updates = append(updates, fmt.Sprintf("podName: %q", *u.podNameAnnotation))
	}
	if u.logSnippet != nil {
		updates = append(updates, fmt.Sprintf("logSnippet: %q", *u.logSnippet))
	}
	if u.pushSecret != nil {
		updates = append(updates, fmt.Sprintf("pushSecret: %v", *u.pushSecret))
	}
	return fmt.Sprintf("buildUpdate(%s)", strings.Join(updates, ", "))
}
