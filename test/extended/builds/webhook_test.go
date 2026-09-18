package builds

import (
	"context"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	watchapi "k8s.io/apimachinery/pkg/watch"

	buildv1 "github.com/openshift/api/build/v1"
)

func TestWaitForBuildSourceImage(t *testing.T) {
	const (
		buildName     = "pushbuild-1"
		expectedImage = "registry:3000/test/imagestream:success"
	)

	build := func(phase buildv1.BuildPhase, kind, image string) *buildv1.Build {
		return &buildv1.Build{
			ObjectMeta: metav1.ObjectMeta{
				Name:            buildName,
				UID:             types.UID("build-uid"),
				ResourceVersion: "42",
			},
			Spec: buildv1.BuildSpec{
				CommonSpec: buildv1.CommonSpec{
					Strategy: buildv1.BuildStrategy{
						SourceStrategy: &buildv1.SourceBuildStrategy{
							From: corev1.ObjectReference{Kind: kind, Name: image},
						},
					},
				},
			},
			Status: buildv1.BuildStatus{Phase: phase},
		}
	}

	t.Run("delayed resolution in a later phase succeeds", func(t *testing.T) {
		events := make(chan watchapi.Event)
		go func() {
			defer close(events)
			events <- watchapi.Event{Type: watchapi.Added, Object: build(buildv1.BuildPhaseNew, "ImageStreamTag", "image-stream:validtag")}
			time.Sleep(20 * time.Millisecond)
			events <- watchapi.Event{Type: watchapi.Modified, Object: build(buildv1.BuildPhaseRunning, "DockerImage", expectedImage)}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		resolved, err := waitForBuildSourceImage(ctx, events, buildName, expectedImage)
		if err != nil {
			t.Fatalf("expected delayed source-image resolution to succeed: %v", err)
		}
		if resolved.Status.Phase != buildv1.BuildPhaseRunning {
			t.Fatalf("expected Running build, got %q", resolved.Status.Phase)
		}
	})

	t.Run("wrong resolved image fails", func(t *testing.T) {
		events := make(chan watchapi.Event, 1)
		events <- watchapi.Event{Type: watchapi.Modified, Object: build(buildv1.BuildPhasePending, "DockerImage", "registry:3000/test/wrong:latest")}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_, err := waitForBuildSourceImage(ctx, events, buildName, expectedImage)
		if err == nil || !strings.Contains(err.Error(), "resolved source image") || !strings.Contains(err.Error(), "wrong:latest") {
			t.Fatalf("expected wrong-image error, got %v", err)
		}
	})

	t.Run("closed watch reports last state", func(t *testing.T) {
		events := make(chan watchapi.Event, 1)
		events <- watchapi.Event{Type: watchapi.Added, Object: build(buildv1.BuildPhaseNew, "ImageStreamTag", "image-stream:validtag")}
		close(events)

		_, err := waitForBuildSourceImage(context.Background(), events, buildName, expectedImage)
		assertErrorContains(t, err, "build watch closed", `phase="New"`, `uid="build-uid"`, `resourceVersion="42"`)
	})

	t.Run("watch error is actionable", func(t *testing.T) {
		events := make(chan watchapi.Event, 1)
		events <- watchapi.Event{Type: watchapi.Error, Object: &metav1.Status{
			Status:  metav1.StatusFailure,
			Message: "watch authorization failed",
			Reason:  metav1.StatusReasonForbidden,
			Code:    403,
		}}

		_, err := waitForBuildSourceImage(context.Background(), events, buildName, expectedImage)
		assertErrorContains(t, err, "build watch reported an error", "watch authorization failed")
	})

	t.Run("timeout reports last unresolved state", func(t *testing.T) {
		events := make(chan watchapi.Event, 1)
		events <- watchapi.Event{Type: watchapi.Added, Object: build(buildv1.BuildPhaseNew, "ImageStreamTag", "image-stream:validtag")}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		_, err := waitForBuildSourceImage(ctx, events, buildName, expectedImage)
		assertErrorContains(t, err, "timed out waiting", `phase="New"`, `uid="build-uid"`, `resourceVersion="42"`, `sourceImageKind="ImageStreamTag"`, `sourceImage="image-stream:validtag"`)
	})
}

func assertErrorContains(t *testing.T, err error, substrings ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, substring := range substrings {
		if !strings.Contains(err.Error(), substring) {
			t.Errorf("expected error %q to contain %q", err, substring)
		}
	}
}
