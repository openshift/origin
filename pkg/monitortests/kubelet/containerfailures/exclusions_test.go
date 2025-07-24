package containerfailures

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
)

func Test_test_exclusions(t *testing.T) {
	tests := []struct {
		name                   string
		mockJobData            platformidentification.JobType
		clusterVersionUpgrades []string
		event                  string
		expected               bool
	}{
		{
			name:        "restart regression that should fail test",
			mockJobData: platformidentification.JobType{},
			event:       "namespace/openshift-container ... container/dummy restarted 4 times at",
			expected:    false,
		},
		{
			name:        "multus; exclude for all platforms",
			mockJobData: platformidentification.JobType{},
			event:       "namespace/openshift-multus ... container/kube-multus restarted 4 times at:",
			expected:    true,
		},
		{
			name:        "container/ovn-acl-loggiing; exclude for all platforms",
			mockJobData: platformidentification.JobType{},
			event:       "namespace/openshift-ovn-kubernetes ... container/ovn-acl-logging restarted 4 times at",
			expected:    true,
		},

		{
			name:                   "upgrades from changing minor version are excluded",
			mockJobData:            platformidentification.JobType{},
			clusterVersionUpgrades: []string{"4.17, 4.18"},
			event:                  "dummy event",
			expected:               true,
		},
		{
			name:                   "upgrades 4.18 and up are not excluded",
			mockJobData:            platformidentification.JobType{},
			clusterVersionUpgrades: []string{"4.18, 4.19"},
			event:                  "dummy event",
			expected:               false,
		},
		{
			name:                   "upgrades 4.18 and up are not excluded",
			mockJobData:            platformidentification.JobType{},
			clusterVersionUpgrades: []string{"4.18, 4.19"},
			event:                  "dummy event",
			expected:               false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExclusion := Exclusion{
				clusterData: platformidentification.ClusterData{
					JobType:               tt.mockJobData,
					ClusterVersionHistory: tt.clusterVersionUpgrades,
				},
			}
			actual := isThisContainerRestartExcluded(tt.event, mockExclusion)
			assert.Equal(t, actual, tt.expected)
		})
	}
}
