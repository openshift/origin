package signature

import (
	"context"
	"testing"
	"time"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

func TestDownloadImageSignatures(t *testing.T) {
	tests := []struct {
		reference            string
		expectSignatureCount int
		expectError          bool
	}{
		{
			reference:            "registry.access.redhat.com/rhel7:latest",
			expectSignatureCount: 0,
			expectError:          false,
		},
		{
			reference:            "docker-registry:5000/foo/bar",
			expectSignatureCount: 0,
			expectError:          false,
		},
		{
			reference:            "test",
			expectSignatureCount: 0,
			expectError:          false,
		},
	}

	for _, c := range tests {
		d := NewContainerImageSignatureDownloader(context.Background(), 1*time.Second)
		image := &imageapi.Image{DockerImageReference: c.reference}
		signatures, err := d.DownloadImageSignatures(image)
		if len(signatures) != c.expectSignatureCount {
			t.Errorf("[%s] expected %d signatures, got %d", c.reference, c.expectSignatureCount, len(signatures))
		}
		if err != nil && !c.expectError {
			t.Errorf("[%s] unexpected error: %v", c.reference, err)
		}
		if err == nil && c.expectError {
			t.Errorf("[%s] expected error, got no error", c.reference)
		}
	}

}
