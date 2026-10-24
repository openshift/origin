package images

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	imagev1 "github.com/openshift/api/image/v1"
	imagev1client "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type diagnosticImageStreams struct {
	imagev1client.ImageStreamInterface
	get func(context.Context, string, metav1.GetOptions) (*imagev1.ImageStream, error)
}

func (streams diagnosticImageStreams) Get(ctx context.Context, name string, options metav1.GetOptions) (*imagev1.ImageStream, error) {
	return streams.get(ctx, name, options)
}

func TestImageStreamTagImportDiagnostics(t *testing.T) {
	t.Parallel()
	generation := int64(7)
	transition := metav1.NewTime(time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC))
	for _, testCase := range []struct {
		name     string
		stream   *imagev1.ImageStream
		err      error
		expected []string
	}{
		{
			name: "import failure exposes only safe metadata",
			stream: &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Generation: 9, Annotations: map[string]string{"private": "sensitive-value"}},
				Spec:       imagev1.ImageStreamSpec{Tags: []imagev1.TagReference{{Name: "latest", Generation: &generation, From: &corev1.ObjectReference{Name: "sensitive-value"}}}},
				Status: imagev1.ImageStreamStatus{Tags: []imagev1.NamedTagEventList{
					{Tag: "other", Conditions: []imagev1.TagEventCondition{{Type: imagev1.ImportSuccess, Generation: 12345}}},
					{Tag: "latest", Items: []imagev1.TagEvent{{Generation: 6, DockerImageReference: "sensitive-value"}}, Conditions: []imagev1.TagEventCondition{
						{Type: imagev1.ImportSuccess, Status: corev1.ConditionFalse, Generation: 7, LastTransitionTime: transition, Reason: "sensitive-value", Message: "sensitive-value"},
						{Type: "sensitive-value", Status: "sensitive-value"},
						{Type: imagev1.ImportSuccess, Status: "sensitive-value"},
					}},
				}},
			},
			expected: []string{"generation=9", "requested tag generation=7", "events=1 conditions=3", "latest event generation=6", "ImportSuccess=False generation=7 lastTransitionTime=2026-01-02T03:04:05Z", "ImportSuccess=Unknown"},
		},
		{
			name:     "missing tag status",
			stream:   &imagev1.ImageStream{},
			expected: []string{"tag status absent"},
		},
		{
			name: "empty tag status and unset generation",
			stream: &imagev1.ImageStream{
				Spec:   imagev1.ImageStreamSpec{Tags: []imagev1.TagReference{{Name: "latest"}}},
				Status: imagev1.ImageStreamStatus{Tags: []imagev1.NamedTagEventList{{Tag: "latest"}}},
			},
			expected: []string{"events=0 conditions=0"},
		},
		{
			name:     "fetch error is best effort and does not expose details",
			err:      errors.New("sensitive-value"),
			expected: []string{"diagnostics unavailable"},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			streams := diagnosticImageStreams{get: func(ctx context.Context, name string, options metav1.GetOptions) (*imagev1.ImageStream, error) {
				calls++
				if name != "busybox" || options != (metav1.GetOptions{}) {
					t.Fatalf("unexpected GET: name=%q options=%v", name, options)
				}
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > 10*time.Second || ctx.Err() != nil {
					t.Fatal("diagnostic GET must have a live context bounded to ten seconds")
				}
				return testCase.stream, testCase.err
			}}
			output := imageStreamTagImportDiagnostics(context.Background(), streams, "busybox", "latest")
			if calls != 1 {
				t.Fatalf("expected one GET, got %d", calls)
			}
			for _, expected := range testCase.expected {
				if !strings.Contains(output, expected) {
					t.Errorf("missing %q in diagnostics: %s", expected, output)
				}
			}
			if strings.Contains(output, "sensitive-value") || strings.Contains(output, "12345") {
				t.Errorf("diagnostics exposed unselected data: %s", output)
			}
		})
	}
}

func TestImageStreamTagImportDiagnosticsBoundsConditions(t *testing.T) {
	t.Parallel()
	conditions := make([]imagev1.TagEventCondition, 100)
	for conditionIndex := range conditions {
		conditions[conditionIndex] = imagev1.TagEventCondition{Type: imagev1.ImportSuccess, Status: corev1.ConditionTrue}
	}
	streams := diagnosticImageStreams{get: func(context.Context, string, metav1.GetOptions) (*imagev1.ImageStream, error) {
		return &imagev1.ImageStream{Status: imagev1.ImageStreamStatus{Tags: []imagev1.NamedTagEventList{{Tag: "latest", Conditions: conditions}}}}, nil
	}}
	output := imageStreamTagImportDiagnostics(context.Background(), streams, "busybox", "latest")
	if strings.Count(output, "ImportSuccess=True") != 10 || !strings.Contains(output, "remaining conditions omitted") {
		t.Fatalf("conditions were not bounded: %s", output)
	}
}

func TestImageStreamTagImportDiagnosticsHonorsCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	streams := diagnosticImageStreams{get: func(ctx context.Context, _ string, _ metav1.GetOptions) (*imagev1.ImageStream, error) {
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatal("diagnostic GET must honor parent cancellation")
		}
		return nil, ctx.Err()
	}}
	if output := imageStreamTagImportDiagnostics(ctx, streams, "busybox", "latest"); !strings.Contains(output, "diagnostics unavailable") {
		t.Fatalf("unexpected cancellation diagnostics: %s", output)
	}
}
