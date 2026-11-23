package podaccess

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

type capturedLines []LogLineContent

func (c *capturedLines) HandleLogLine(line LogLineContent) { *c = append(*c, line) }

// ReadLog tags only a failed pod lookup. The message names the pod that was
// looked up, while the wrapped error and API status reason stay what existing
// callers already match on, such as apierrors.IsNotFound.
func TestReadLogTagsOnlyPodLookupFailures(t *testing.T) {
	const namespace, podName = "openshift-test-operator", "test-operator-abcde"
	notFound := apierrors.NewNotFound(corev1.Resource("pods"), podName)
	forbidden := apierrors.NewForbidden(corev1.Resource("pods"), podName, errors.New("synthetic denial"))

	tests := []struct {
		name         string
		lookupErr    error
		logErr       error
		wantTagged   bool
		wantMessage  string
		wantReason   metav1.StatusReason
		wantLines    []string
		wantRequests []string
	}{
		{
			name:         "pod lookup NotFound",
			lookupErr:    notFound,
			wantTagged:   true,
			wantMessage:  "error getting pod to read logs for pods/test-operator-abcde -n openshift-test-operator: " + notFound.Error(),
			wantReason:   metav1.StatusReasonNotFound,
			wantRequests: []string{"get pods"},
		},
		{
			name:         "pod lookup forbidden",
			lookupErr:    forbidden,
			wantTagged:   true,
			wantMessage:  "error getting pod to read logs for pods/test-operator-abcde -n openshift-test-operator: " + forbidden.Error(),
			wantReason:   metav1.StatusReasonForbidden,
			wantRequests: []string{"get pods"},
		},
		{
			name:         "pod log NotFound naming the same pod",
			logErr:       notFound,
			wantReason:   metav1.StatusReasonNotFound,
			wantRequests: []string{"get pods", "get pods/log"},
		},
		{
			name:         "pod log read",
			wantLines:    []string{"operator started"},
			wantRequests: []string{"get pods", "get pods/log"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleClientset(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: podName}})
			client.PrependReactor("get", "pods", func(action clienttesting.Action) (bool, runtime.Object, error) {
				switch {
				case action.GetSubresource() == "" && tt.lookupErr != nil:
					return true, nil, tt.lookupErr
				case action.GetSubresource() == "log" && tt.logErr != nil:
					return true, nil, tt.logErr
				case action.GetSubresource() == "log":
					return true, &runtime.Unknown{Raw: []byte("2026-01-01T00:00:00Z operator started\n")}, nil
				}
				return false, nil, nil
			})
			var lines capturedLines

			err := NewOneTimePodStreamer(client, namespace, podName, "operator", &lines).ReadLog(context.Background())
			if wantErr := tt.wantReason != ""; (err != nil) != wantErr {
				t.Fatalf("ReadLog() error = %v, want error: %v", err, wantErr)
			}

			var lookupErr *PodLookupError
			if got := errors.As(err, &lookupErr); got != tt.wantTagged {
				t.Errorf("ReadLog() error %v tagged as a pod lookup failure = %v, want %v", err, got, tt.wantTagged)
			}
			if tt.wantTagged {
				if err.Error() != tt.wantMessage {
					t.Errorf("ReadLog() error = %q, want %q", err, tt.wantMessage)
				}
				if errors.Unwrap(err) != tt.lookupErr {
					t.Errorf("ReadLog() error unwraps to %v, want %v", errors.Unwrap(err), tt.lookupErr)
				}
			}
			if got := apierrors.ReasonForError(err); got != tt.wantReason {
				t.Errorf("apierrors.ReasonForError(ReadLog()) = %q, want %q", got, tt.wantReason)
			}

			var gotLines []string
			for _, line := range lines {
				if line.Pod.Name != podName || !line.Instant.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
					t.Errorf("handled log line %+v, want one from %s timestamped 2026-01-01T00:00:00Z", line, podName)
				}
				gotLines = append(gotLines, line.Line)
			}
			if !slices.Equal(gotLines, tt.wantLines) {
				t.Errorf("handled log lines = %q, want %q", gotLines, tt.wantLines)
			}

			var requests []string
			for _, action := range client.Actions() {
				request := action.GetVerb() + " " + action.GetResource().Resource
				if action.GetSubresource() != "" {
					request += "/" + action.GetSubresource()
				}
				requests = append(requests, request)
			}
			if !slices.Equal(requests, tt.wantRequests) {
				t.Errorf("API requests = %v, want %v", requests, tt.wantRequests)
			}
		})
	}
}
