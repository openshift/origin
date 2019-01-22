package certrotation

import (
	"testing"
	"time"
)

func TestIsOverlapSufficient(t *testing.T) {
	tests := []struct {
		name string

		saValidity              time.Duration
		saRefreshPercentage     float32
		targetValidity          time.Duration
		targetRefreshPercentage float32

		expected bool
	}{
		{
			name:                    "plenty",
			saValidity:              4,
			saRefreshPercentage:     0.5,
			targetValidity:          2,
			targetRefreshPercentage: 0.5,
			expected:                true,
		},
		{
			name:                    "tight",
			saValidity:              10,
			saRefreshPercentage:     0.5,
			targetValidity:          5,
			targetRefreshPercentage: 0.5,
			expected:                true,
		},
		{
			name:                    "not enough",
			saValidity:              10,
			saRefreshPercentage:     0.3,
			targetValidity:          100,
			targetRefreshPercentage: 0.8,
			expected:                false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := isOverlapSufficient(
				SigningRotation{Validity: test.saValidity, RefreshPercentage: test.saRefreshPercentage},
				TargetRotation{Validity: test.targetValidity, RefreshPercentage: test.targetRefreshPercentage},
			)
			if test.expected != actual {
				t.Errorf("expected %v, got %v", test.expected, actual)
			}
		})
	}

}
