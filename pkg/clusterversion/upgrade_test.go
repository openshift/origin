package clusterversion

import (
	"testing"

	configv1 "github.com/openshift/api/config/v1"
)

func TestIsUpgradedFromMinorVersion(t *testing.T) {
	cases := []struct {
		Name               string
		UpgradeFromVersion string
		VersionHistory     []string
		Expected           bool
	}{
		{
			Name:               "no history",
			UpgradeFromVersion: "4.15",
			Expected:           false,
		},
		{
			Name:               "upgraded to 4.16 from 4.15",
			UpgradeFromVersion: "4.15",
			VersionHistory:     []string{"4.16.0", "4.15.9", "4.15.7", "4.15.2"},
			Expected:           true,
		},
		{
			Name:               "upgraded to 4.16 from 4.14",
			UpgradeFromVersion: "4.15",
			VersionHistory:     []string{"4.16.0", "4.15.9", "4.15.7", "4.15.2", "4.14.9"},
			Expected:           true,
		},
		{
			Name:               "skip odd minor version",
			UpgradeFromVersion: "4.15",
			VersionHistory:     []string{"4.16.0", "4.14.9", "4.14.2", "4.12.14", "4.12.8"},
			Expected:           true,
		},
		{
			Name:               "not reached upgrade",
			UpgradeFromVersion: "4.15",
			VersionHistory:     []string{"4.14.0", "4.13.9", "4.13.2", "4.12.14", "4.12.8"},
			Expected:           false,
		},
		{
			Name:               "invalid version",
			UpgradeFromVersion: "bad-data",
			VersionHistory:     []string{"4.16.0", "4.15.9", "4.15.7"},
			Expected:           false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			history := []configv1.UpdateHistory{}
			for _, version := range tc.VersionHistory {
				history = append(history, configv1.UpdateHistory{
					Version: version,
				})
			}
			cv := &configv1.ClusterVersion{
				Status: configv1.ClusterVersionStatus{
					History: history,
				},
			}
			upgraded := IsUpgradedFromMinorVersion(tc.UpgradeFromVersion, cv)
			if upgraded != tc.Expected {
				t.Errorf("expected %v, got %v", tc.Expected, upgraded)
			}
		})
	}
}
