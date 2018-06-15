package keepalived

import (
	"strings"
	"testing"

	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/oc/experimental/ipfailover/ipfailover"
	"github.com/openshift/origin/pkg/oc/generate/app"
)

func makeIPFailoverConfigOptions(selector string, replicas int32, serviceAccount string) *ipfailover.IPFailoverConfigCmdOptions {
	return &ipfailover.IPFailoverConfigCmdOptions{
		ImageTemplate:    variable.NewDefaultImageTemplate(),
		Selector:         selector,
		VirtualIPs:       "",
		WatchPort:        80,
		NetworkInterface: "eth0",
		Replicas:         replicas,
		ServiceAccount:   serviceAccount,
	}

}

func makeSelector(options *ipfailover.IPFailoverConfigCmdOptions) map[string]string {
	labels, _, err := app.LabelsFromSpec(strings.Split(options.Selector, ","))
	if err == nil {
		return labels
	}

	return map[string]string{}
}

func TestGenerateDeploymentConfig(t *testing.T) {
	tests := []struct {
		Name              string
		Selector          string
		Replicas          int32
		PodSelectorLength int
		ServiceAccount    string
	}{
		{
			Name:              "config-test-no-selector",
			Selector:          "",
			Replicas:          1,
			PodSelectorLength: 0,
		},
		{
			Name:              "config-test-default-selector",
			Selector:          "ipfailover=config-test-default-selector",
			Replicas:          2,
			PodSelectorLength: 0,
		},
		{
			Name:              "config-test-non-default-selector",
			Selector:          "ipfailover=test-nodes",
			Replicas:          3,
			PodSelectorLength: 1,
		},
		{
			Name:              "config-test-selector",
			Selector:          "router=geo-us-west",
			Replicas:          3,
			PodSelectorLength: 1,
		},
		{
			Name:              "config-test-ha-router-selector",
			Selector:          "router=geo-us-west,az=us-west-1",
			Replicas:          4,
			PodSelectorLength: 2,
		},
		{
			Name:              "config-test-multi-selector",
			Selector:          "foo=bar,baz=none,open=sesame,ha=ha",
			Replicas:          42,
			PodSelectorLength: 4,
		},
		{
			Name:              "config-test-service-account",
			Selector:          "router=geo-us-west",
			Replicas:          3,
			PodSelectorLength: 1,
			ServiceAccount:    "foo",
		},
	}

	for _, tc := range tests {
		options := makeIPFailoverConfigOptions(tc.Selector, tc.Replicas, tc.ServiceAccount)
		selector := makeSelector(options)
		dc, err := GenerateDeploymentConfig(tc.Name, options, selector)
		if err != nil {
			t.Errorf("Test case for %s got an error %v where none was expected", tc.Name, err)
			continue
		}
		if tc.Name != dc.Name {
			t.Errorf("Test case for %s got DeploymentConfig name %v where %v was expected", tc.Name, dc.Name, tc.Name)
		}

		if dc.Spec.Replicas != tc.Replicas {
			t.Errorf("Test case for %s got controller replicas %v where %v was expected", tc.Name, dc.Spec.Replicas, tc.Replicas)
		}

		podSpec := dc.Spec.Template.Spec
		if !podSpec.SecurityContext.HostNetwork {
			t.Errorf("Test case for %s got HostNetwork disabled where HostNetwork was expected to be enabled", tc.Name)
		}

		if podSpec.ServiceAccountName != tc.ServiceAccount {
			t.Errorf("Test case for %s got service account %s when expecting %s", tc.Name, podSpec.ServiceAccountName, tc.ServiceAccount)
		}

		psLength := len(podSpec.NodeSelector)
		if tc.PodSelectorLength != psLength {
			t.Errorf("Test case for %s got pod spec NodeSelector length %v where %v was expected",
				tc.Name, psLength, tc.PodSelectorLength)
		}

		volumeCount := len(podSpec.Volumes)
		if volumeCount < 1 {
			t.Errorf("Test case for %s got pod spec Volumes count %v where at least 1 was expected", tc.Name, volumeCount)
		}
	}
}
