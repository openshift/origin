package legacynodemonitortests

import (
	"fmt"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/stretchr/testify/assert"
)

func TestInvalidImagesREMatchExpectedPatterns(t *testing.T) {
	// Event strings from actual conformance test runs that should be skipped
	// because they come from tests using intentionally invalid images.
	expectedMatches := []struct {
		name  string
		event string
	}{
		{
			name:  "invalid.com registry",
			event: `namespace/e2e-container-runtime-123 pod/test: cause/ErrImagePull reason/ContainerWait  unable to pull image docker://invalid.com/test:latest`,
		},
		{
			name:  "authenticated alpine image",
			event: `namespace/e2e-container-runtime-801 pod/image-pull-test: cause/ErrImagePull reason/ContainerWait  unable to pull image docker://gcr.io/authenticated-image-pulling/alpine:3.7`,
		},
		{
			name:  "webserver:404 nonexistent tag",
			event: `namespace/e2e-deployment-123 pod/test: cause/ErrImagePull reason/ContainerWait  unable to pull image docker://docker.io/library/webserver:404`,
		},
		{
			name:  "gb-redisslave:nonexistent deployment test",
			event: `namespace/e2e-deployment-6285 pod/test-rollover: cause/ErrImagePull reason/ContainerWait  unable to pull image docker://gcr.io/google_samples/gb-redisslave:nonexistent`,
		},
		{
			name:  "some-image-that-doesnt-exist pod test",
			event: `namespace/e2e-node-123 pod/test: cause/ErrImagePull reason/ContainerWait  unable to pull image docker://localhost/some-image-that-doesnt-exist:latest`,
		},
		{
			name:  "bare nonexistent image",
			event: `namespace/e2e-image-volume-6286 pod/image-volume-test: cause/ErrImagePull reason/ContainerWait  unable to pull image docker://nonexistent:latest`,
		},
	}

	for _, tc := range expectedMatches {
		t.Run(tc.name, func(t *testing.T) {
			matched := false
			for _, re := range invalidImagesRE {
				if re.MatchString(tc.event) {
					matched = true
					break
				}
			}
			assert.True(t, matched, "invalidImagesRE should match event: %s", tc.event)
		})
	}
}

func TestInvalidImagesREDoNotMatchRealFailures(t *testing.T) {
	// Events that represent real ErrImagePull failures that should NOT be skipped.
	unexpectedMatches := []struct {
		name  string
		event string
	}{
		{
			name:  "real image pull failure in test namespace",
			event: `namespace/e2e-test-123 pod/app: cause/ErrImagePull reason/ContainerWait  unable to pull image docker://quay.io/openshift/my-real-app:v1.0`,
		},
		{
			name:  "real image pull failure in default namespace",
			event: `namespace/default pod/app: cause/ErrImagePull reason/ContainerWait  unable to pull image docker://registry.example.com/app:latest`,
		},
		{
			name:  "nonexistent prefix should not match",
			event: `namespace/e2e-test-123 pod/app: cause/ErrImagePull reason/ContainerWait  unable to pull image docker://nonexistent-app:v1`,
		},
		{
			name:  "look-alike gcr registry should not match",
			event: `namespace/e2e-test-123 pod/app: cause/ErrImagePull reason/ContainerWait  unable to pull image docker://gcrXio/authenticated-image-pulling/alpine:3.7`,
		},
		{
			name:  "look-alike docker registry should not match",
			event: `namespace/e2e-test-123 pod/app: cause/ErrImagePull reason/ContainerWait  unable to pull image docker://dockerXio/library/webserver:404`,
		},
		{
			name:  "different authenticated alpine tag should not match",
			event: `namespace/e2e-test-123 pod/app: cause/ErrImagePull reason/ContainerWait  unable to pull image docker://gcr.io/authenticated-image-pulling/alpine:latest`,
		},
	}

	for _, tc := range unexpectedMatches {
		t.Run(tc.name, func(t *testing.T) {
			matched := false
			for _, re := range invalidImagesRE {
				if re.MatchString(tc.event) {
					matched = true
					break
				}
			}
			assert.False(t, matched, "invalidImagesRE should NOT match real failure: %s", tc.event)
		})
	}
}

func TestErrImagePullManifestUnknownAllowlistEvaluation(t *testing.T) {
	tests := []struct {
		name        string
		image       string
		wantFailure bool
	}{
		{
			name:        "expected invalid image pull is skipped",
			image:       "docker://gcr.io/authenticated-image-pulling/alpine:3.7",
			wantFailure: false,
		},
		{
			name:        "genuine image pull failure is reported",
			image:       "docker://nonexistent-app:v1",
			wantFailure: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			event := monitorapi.NewInterval(monitorapi.SourceKubeEvent, monitorapi.Warning).
				Locator(monitorapi.NewLocator().PodFromNames("e2e-test", "test-pod", "")).
				Message(monitorapi.NewMessage().
					Reason("ErrImagePull").
					HumanMessage(fmt.Sprintf("unable to pull image %s: manifest unknown", tc.image))).
				Build(time.Unix(0, 0), time.Unix(1, 0))

			junitTests := testErrImagePullManifestUnknown(monitorapi.Intervals{event})
			hasFailure := false
			for _, junitTest := range junitTests {
				if junitTest.FailureOutput != nil {
					hasFailure = true
					break
				}
			}

			assert.Equal(t, tc.wantFailure, hasFailure)
		})
	}
}
