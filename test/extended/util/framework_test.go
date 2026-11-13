package util

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	imagev1 "github.com/openshift/api/image/v1"
	imagev1typedclient "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
)

type importStatusImageStreams struct {
	imagev1typedclient.ImageStreamInterface
	stream      *imagev1.ImageStream
	watchStream *imagev1.ImageStream
	listError   error
	listDelay   time.Duration
	watchError  error
	watcher     *watch.RaceFreeFakeWatcher
}

func (client *importStatusImageStreams) List(context.Context, metav1.ListOptions) (*imagev1.ImageStreamList, error) {
	time.Sleep(client.listDelay)
	list := &imagev1.ImageStreamList{}
	if client.stream != nil {
		list.Items = append(list.Items, *client.stream)
	}
	return list, client.listError
}

func (client *importStatusImageStreams) Watch(context.Context, metav1.ListOptions) (watch.Interface, error) {
	if client.watcher.IsStopped() {
		return nil, errors.New("test watch stopped")
	}
	if client.watchStream != nil {
		client.watcher.Modify(client.watchStream)
	}
	return client.watcher, client.watchError
}

func TestImageStreamTagImportStatus(t *testing.T) {
	t.Parallel()
	generation := int64(7)
	stream := &imagev1.ImageStream{
		ObjectMeta: metav1.ObjectMeta{Generation: 8, Annotations: map[string]string{"private": "must-not-appear"}},
		Spec:       imagev1.ImageStreamSpec{Tags: []imagev1.TagReference{{Name: "latest", Generation: &generation, From: &corev1.ObjectReference{Name: "must-not-appear"}}}},
		Status: imagev1.ImageStreamStatus{Tags: []imagev1.NamedTagEventList{
			{Tag: "unrelated", Conditions: []imagev1.TagEventCondition{{Reason: "must-not-appear"}}},
			{Tag: "latest", Conditions: []imagev1.TagEventCondition{{Type: imagev1.ImportSuccess, Status: corev1.ConditionFalse, Reason: "Unauthorized", Message: "must-not-appear", Generation: 7}}},
		}},
	}
	status := imageStreamTagImportStatus(stream, "latest")
	for _, expected := range []string{"stream generation=8", "requested generation=7", "items=0; conditions=1", `type="ImportSuccess" status="False" reason="Unauthorized" generation=7`} {
		if !strings.Contains(status, expected) {
			t.Errorf("status %q missing %q", status, expected)
		}
	}
	if strings.Contains(status, "must-not-appear") {
		t.Fatalf("unrelated or free-form content leaked: %s", status)
	}
	stream.Status.Tags[1].Items = []imagev1.TagEvent{{Generation: 6, DockerImageReference: "must-not-appear"}}
	status = imageStreamTagImportStatus(stream, "latest")
	if !strings.Contains(status, "latest item generation=6") || strings.Contains(status, "must-not-appear") {
		t.Fatalf("unexpected item diagnostics: %s", status)
	}
	stream.Status.Tags[1].Conditions = make([]imagev1.TagEventCondition, 100)
	stream.Status.Tags[1].Conditions[0].Reason = strings.Repeat("a", 10000)
	stream.Status.Tags[1].Conditions[1].Reason = "https://example.invalid/?token=must-not-appear"
	status = imageStreamTagImportStatus(stream, "latest")
	if len(status) > 1024 || !strings.Contains(status, "remaining conditions omitted") || strings.Contains(status, "must-not-appear") || strings.Contains(status, strings.Repeat("a", 129)) {
		t.Fatalf("unbounded or unsafe status: %.1024s", status)
	}
}

func TestWaitForAnImageStreamTag(t *testing.T) {
	t.Parallel()
	apiError := apierrors.NewForbidden(schema.GroupResource{Group: "image.openshift.io", Resource: "imagestreams"}, "busybox", errors.New("denied"))
	notFound := apierrors.NewNotFound(schema.GroupResource{Group: "image.openshift.io", Resource: "imagestreams"}, "busybox")
	for _, test := range []struct {
		name        string
		stream      *imagev1.ImageStream
		watchStream *imagev1.ImageStream
		listError   error
		listDelay   time.Duration
		watchError  error
		want        string
		success     bool
		timesOut    bool
	}{
		{name: "imported", stream: &imagev1.ImageStream{Status: imagev1.ImageStreamStatus{Tags: []imagev1.NamedTagEventList{{Tag: "latest", Items: []imagev1.TagEvent{{Generation: 2}}}}}}, success: true},
		{name: "watch imported", watchStream: &imagev1.ImageStream{Status: imagev1.ImageStreamStatus{Tags: []imagev1.NamedTagEventList{{Tag: "latest", Items: []imagev1.TagEvent{{Generation: 2}}}}}}, success: true},
		{name: "delayed imported", listDelay: 250 * time.Millisecond, stream: &imagev1.ImageStream{Status: imagev1.ImageStreamStatus{Tags: []imagev1.NamedTagEventList{{Tag: "latest", Items: []imagev1.TagEvent{{Generation: 2}}}}}}, success: true},
		{name: "delayed list API error", listDelay: 250 * time.Millisecond, listError: apiError, want: "no image stream observed"},
		{name: "delayed watch API error", listDelay: 250 * time.Millisecond, watchError: apiError, want: "no image stream observed"},
		{name: "watch timeout retains conditions", watchStream: &imagev1.ImageStream{Status: imagev1.ImageStreamStatus{Tags: []imagev1.NamedTagEventList{{Tag: "latest", Conditions: []imagev1.TagEventCondition{{Type: imagev1.ImportSuccess, Status: corev1.ConditionFalse, Reason: "Unauthorized", Generation: 4}}}}}}, want: `reason="Unauthorized" generation=4`, timesOut: true},
		{name: "list API error", listError: apiError, want: "no image stream observed"},
		{name: "not found", listError: notFound, want: "no image stream observed"},
		{name: "empty list timeout", want: "no image stream observed", timesOut: true},
		{name: "empty status timeout", stream: &imagev1.ImageStream{}, want: "tag status absent", timesOut: true},
		{name: "empty tag timeout", stream: &imagev1.ImageStream{Status: imagev1.ImageStreamStatus{Tags: []imagev1.NamedTagEventList{{Tag: "latest"}}}}, want: "items=0; conditions=0", timesOut: true},
		{name: "watch API error retains conditions", stream: &imagev1.ImageStream{Status: imagev1.ImageStreamStatus{Tags: []imagev1.NamedTagEventList{{Tag: "latest", Conditions: []imagev1.TagEventCondition{{Type: imagev1.ImportSuccess, Status: corev1.ConditionFalse, Reason: "Unauthorized", Generation: 3}}}}}}, watchError: apiError, want: `reason="Unauthorized" generation=3`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			watcher := watch.NewRaceFreeFake()
			defer watcher.Stop()
			client := &importStatusImageStreams{stream: test.stream, watchStream: test.watchStream, listError: test.listError, listDelay: test.listDelay, watchError: test.watchError, watcher: watcher}
			waitTimeout := 5 * time.Second
			if test.timesOut {
				waitTimeout = 100 * time.Millisecond
			}
			err := waitForAnImageStreamTag(client, "test", "busybox", "latest", waitTimeout)
			if test.success {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q, got %v", test.want, err)
			}
			for _, original := range []error{test.listError, test.watchError} {
				if original != nil && !errors.Is(err, original) {
					t.Errorf("original error not preserved: %v", err)
				}
			}
			if test.timesOut && !strings.HasPrefix(err.Error(), "timed out while waiting of an image stream tag test/busybox:latest") {
				t.Errorf("timeout changed: %v", err)
			}
			if test.listError == apiError || test.watchError == apiError {
				if !apierrors.IsForbidden(err) {
					t.Errorf("API failure classification lost: %v", err)
				}
			}
			if test.listError == notFound && !apierrors.IsNotFound(err) {
				t.Errorf("not-found classification lost: %v", err)
			}
			t.Log(err)
		})
	}
}

func TestImageStreamTagImportStatusKnownIdentifiers(t *testing.T) {
	t.Parallel()
	for _, conditionStatus := range []corev1.ConditionStatus{corev1.ConditionTrue, corev1.ConditionFalse, corev1.ConditionUnknown} {
		for _, reason := range []string{"Unauthorized", "Forbidden", "NotFound", "Invalid", "InternalError", "ImportFailed"} {
			t.Run(string(conditionStatus)+"/"+reason, func(t *testing.T) {
				t.Parallel()
				stream := &imagev1.ImageStream{Status: imagev1.ImageStreamStatus{Tags: []imagev1.NamedTagEventList{{Tag: "latest", Conditions: []imagev1.TagEventCondition{{Type: imagev1.ImportSuccess, Status: conditionStatus, Reason: reason}}}}}}
				status := imageStreamTagImportStatus(stream, "latest")
				for _, expected := range []string{`type="ImportSuccess"`, `status="` + string(conditionStatus) + `"`, `reason="` + reason + `"`} {
					if !strings.Contains(status, expected) {
						t.Errorf("status %q missing %q", status, expected)
					}
				}
			})
		}
	}
}

func TestImageStreamTagImportStatusUnrecognizedIdentifiers(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		payload string
	}{
		{name: "synthetic digits", payload: "000000000"},
		{name: "synthetic SSN shape", payload: "000-00-0000"},
		{name: "synthetic alphanumeric token", payload: "SyntheticTokenabc123"},
		{name: "empty"},
		{name: "oversized", payload: strings.Repeat("a", 10000)},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			payload := test.payload
			stream := &imagev1.ImageStream{Status: imagev1.ImageStreamStatus{Tags: []imagev1.NamedTagEventList{{Tag: "latest", Conditions: []imagev1.TagEventCondition{{Type: imagev1.TagEventConditionType(payload), Status: corev1.ConditionStatus(payload), Reason: payload, Message: payload}}}}}}
			watcher := watch.NewRaceFreeFake()
			defer watcher.Stop()
			apiError := apierrors.NewForbidden(schema.GroupResource{Group: "image.openshift.io", Resource: "imagestreams"}, "busybox", errors.New("denied"))
			client := &importStatusImageStreams{stream: stream, watchError: apiError, watcher: watcher}
			err := waitForAnImageStreamTag(client, "test", "busybox", "latest", 5*time.Second)
			if !errors.Is(err, apiError) || !apierrors.IsForbidden(err) {
				t.Fatalf("API error not preserved: %v", err)
			}
			for _, output := range []string{imageStreamTagImportStatus(stream, "latest"), err.Error()} {
				if !strings.Contains(output, `type="omitted" status="omitted" reason="omitted"`) {
					t.Errorf("identifiers not omitted: %s", output)
				}
				if payload != "" && strings.Contains(output, payload) {
					t.Errorf("unrecognized identifier leaked in diagnostics")
				}
			}
		})
	}
}
