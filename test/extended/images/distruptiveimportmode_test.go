package images

import (
	"testing"

	configv1 "github.com/openshift/api/config/v1"
)

func TestExpectedImageStreamImportMode(t *testing.T) {
	const (
		legacy    = string(configv1.ImportModeLegacy)
		preserve  = string(configv1.ImportModePreserveOriginal)
		multiArch = configv1.ClusterVersionArchitectureMulti
	)

	tests := []struct {
		name         string
		specMode     string
		currentMode  string
		architecture configv1.ClusterVersionArchitecture
		want         string
	}{
		{
			name:         "explicit spec wins over architecture",
			specMode:     legacy,
			currentMode:  preserve,
			architecture: multiArch,
			want:         legacy,
		},
		{
			name:         "explicit PreserveOriginal spec wins",
			specMode:     preserve,
			currentMode:  legacy,
			architecture: "",
			want:         preserve,
		},
		{
			name:         "empty spec with multi architecture maps to PreserveOriginal",
			specMode:     "",
			currentMode:  legacy,
			architecture: multiArch,
			want:         preserve,
		},
		{
			name:         "empty spec with non-empty non-multi architecture maps to Legacy",
			specMode:     "",
			currentMode:  preserve,
			architecture: "Amd64",
			want:         legacy,
		},
		{
			name:         "empty spec with empty architecture preserves current Legacy",
			specMode:     "",
			currentMode:  legacy,
			architecture: "",
			want:         legacy,
		},
		{
			name:         "empty spec with empty architecture preserves current PreserveOriginal",
			specMode:     "",
			currentMode:  preserve,
			architecture: "",
			want:         preserve,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expectedImageStreamImportMode(tt.specMode, tt.currentMode, tt.architecture)
			if got != tt.want {
				t.Errorf("expectedImageStreamImportMode(%q, %q, %q) = %q, want %q",
					tt.specMode, tt.currentMode, tt.architecture, got, tt.want)
			}
		})
	}
}
