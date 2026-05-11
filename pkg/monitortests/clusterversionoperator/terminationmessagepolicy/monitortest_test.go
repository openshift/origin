package terminationmessagepolicy

import (
	"testing"

	configv1 "github.com/openshift/api/config/v1"
)

func TestHasOldVersion(t *testing.T) {
	tests := []struct {
		name     string
		versions []string
		want     bool
	}{
		{
			name:     "4.15.3 is old",
			versions: []string{"4.15.3"},
			want:     true,
		},
		{
			name:     "4.15.0-rc.1 is old",
			versions: []string{"4.15.0-rc.1"},
			want:     true,
		},
		{
			name:     "4.15 is not semver, don't recognize",
			versions: []string{"4.15"},
			want:     false,
		},
		{
			name:     "4.16.0-okd is not old",
			versions: []string{"4.16.0-okd"},
			want:     false,
		},
		{
			name:     "5.0 is not old",
			versions: []string{"5.0"},
			want:     false,
		},
		{
			name:     "empty history",
			versions: []string{},
			want:     false,
		},
		{
			name:     "new version with old version in history",
			versions: []string{"4.16.2", "4.15.1"},
			want:     true,
		},
		{
			name:     "4.100 is not old",
			versions: []string{"4.100"},
			want:     false,
		},
		{
			name:     "14.11 is not old (major ending in 4)",
			versions: []string{"14.11"},
			want:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cv := &configv1.ClusterVersion{}
			for _, v := range tt.versions {
				cv.Status.History = append(cv.Status.History, configv1.UpdateHistory{Version: v})
			}
			if got := hasOldVersion(cv); got != tt.want {
				t.Errorf("hasOldVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}
