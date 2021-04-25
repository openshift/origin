package main

import (
	"os"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	exutilcluster "github.com/openshift/origin/test/extended/util/cluster"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

var gcePlatform = &configv1.PlatformStatus{
	Type: configv1.GCPPlatformType,
	GCP: &configv1.GCPPlatformStatus{
		ProjectID: "openshift-gce-devel-ci",
		Region:    "us-east1",
	},
}

var awsPlatform = &configv1.PlatformStatus{
	Type: configv1.AWSPlatformType,
	AWS: &configv1.AWSPlatformStatus{
		Region: "us-east-2",
	},
}

var vspherePlatform = &configv1.PlatformStatus{
	Type: configv1.VSpherePlatformType,
}

var noPlatform = &configv1.PlatformStatus{
	Type: configv1.NonePlatformType,
}

var gceMasters = &corev1.NodeList{
	Items: []corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "master-1",
				Labels: map[string]string{
					"failure-domain.beta.kubernetes.io/zone": "us-east1-a",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "master-2",
				Labels: map[string]string{
					"failure-domain.beta.kubernetes.io/zone": "us-east1-b",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "master-3",
				Labels: map[string]string{
					"failure-domain.beta.kubernetes.io/zone": "us-east1-c",
				},
			},
		},
	},
}

var simpleMasters = &corev1.NodeList{
	Items: []corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "master-1",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "master-2",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "master-3",
			},
		},
	},
}

var nonMasters = &corev1.NodeList{
	Items: []corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "worker-1",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "worker-2",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "worker-3",
			},
		},
	},
}

var sdnConfig = &operatorv1.NetworkSpec{
	DefaultNetwork: operatorv1.DefaultNetworkDefinition{
		Type:               operatorv1.NetworkTypeOpenShiftSDN,
		OpenShiftSDNConfig: &operatorv1.OpenShiftSDNConfig{},
	},
	ServiceNetwork: []string{"172.30.0.0/16"},
}

var multitenantConfig = &operatorv1.NetworkSpec{
	DefaultNetwork: operatorv1.DefaultNetworkDefinition{
		Type: operatorv1.NetworkTypeOpenShiftSDN,
		OpenShiftSDNConfig: &operatorv1.OpenShiftSDNConfig{
			Mode: operatorv1.SDNModeMultitenant,
		},
	},
	ServiceNetwork: []string{"172.30.0.0/16"},
}

var ovnKubernetesConfig = &operatorv1.NetworkSpec{
	DefaultNetwork: operatorv1.DefaultNetworkDefinition{
		Type: operatorv1.NetworkTypeOVNKubernetes,
	},
	ServiceNetwork: []string{
		"172.30.0.0/16",
		"fd02::/112",
	},
}

var e2eTests = map[string]string{
	"everyone":        "[Skipped:Wednesday]",
	"not-gce":         "[Skipped:gce]",
	"not-aws":         "[Skipped:aws]",
	"not-sdn":         "[Skipped:Network/OpenShiftSDN]",
	"not-multitenant": "[Skipped:Network/OpenShiftSDN/Multitenant]",
	"online":          "[Skipped:Disconnected]",
	"ipv4":            "[Feature:Networking-IPv4]",
	"ipv6":            "[Feature:Networking-IPv6]",
	"dual-stack":      "[Feature:IPv6DualStackAlpha]",
	"sctp":            "[Feature:SCTPConnectivity]",
}

func TestDecodeProvider(t *testing.T) {
	var testCases = []struct {
		name     string
		provider string

		discoveredPlatform *configv1.PlatformStatus
		discoveredMasters  *corev1.NodeList
		discoveredNetwork  *operatorv1.NetworkSpec

		expectedConfig string
		runTests       sets.String
	}{
		{
			name:               "simple GCE",
			provider:           "",
			discoveredPlatform: gcePlatform,
			discoveredMasters:  gceMasters,
			discoveredNetwork:  sdnConfig,
			expectedConfig:     `{"type":"gce","ProjectID":"openshift-gce-devel-ci","Region":"us-east1","Zone":"us-east1-a","NumNodes":3,"MultiMaster":true,"MultiZone":true,"Zones":["us-east1-a","us-east1-b","us-east1-c"],"ConfigFile":"","Disconnected":false,"NetworkPlugin":"OpenShiftSDN","HasIPv4":true,"HasIPv6":false,"HasSCTP":false}`,
			runTests:           sets.NewString("everyone", "not-aws", "not-multitenant", "online", "ipv4"),
		},
		{
			name:               "GCE multitenant",
			provider:           "",
			discoveredPlatform: gcePlatform,
			discoveredMasters:  gceMasters,
			discoveredNetwork:  multitenantConfig,
			expectedConfig:     `{"type":"gce","ProjectID":"openshift-gce-devel-ci","Region":"us-east1","Zone":"us-east1-a","NumNodes":3,"MultiMaster":true,"MultiZone":true,"Zones":["us-east1-a","us-east1-b","us-east1-c"],"ConfigFile":"","Disconnected":false,"NetworkPlugin":"OpenShiftSDN","NetworkPluginMode":"Multitenant","HasIPv4":true,"HasIPv6":false,"HasSCTP":false}`,
			runTests:           sets.NewString("everyone", "not-aws", "online", "ipv4"),
		},
		{
			name:               "simple non-cloud",
			provider:           "",
			discoveredPlatform: noPlatform,
			discoveredMasters:  simpleMasters,
			discoveredNetwork:  sdnConfig,
			expectedConfig:     `{"type":"skeleton","ProjectID":"","Region":"","Zone":"","NumNodes":3,"MultiMaster":true,"MultiZone":false,"Zones":[],"ConfigFile":"","Disconnected":false,"NetworkPlugin":"OpenShiftSDN","HasIPv4":true,"HasIPv6":false,"HasSCTP":false}`,
			runTests:           sets.NewString("everyone", "not-gce", "not-aws", "not-multitenant", "online", "ipv4"),
		},
		{
			name:               "simple override",
			provider:           "vsphere",
			discoveredPlatform: vspherePlatform,
			discoveredMasters:  simpleMasters,
			discoveredNetwork:  sdnConfig,
			// NB: It does not actually use the passed-in Provider value
			expectedConfig: `{"type":"skeleton","ProjectID":"","Region":"","Zone":"","NumNodes":3,"MultiMaster":true,"MultiZone":false,"Zones":[],"ConfigFile":"","Disconnected":false,"NetworkPlugin":"OpenShiftSDN","HasIPv4":true,"HasIPv6":false,"HasSCTP":false}`,
			runTests:       sets.NewString("everyone", "not-gce", "not-aws", "not-multitenant", "online", "ipv4"),
		},
		{
			name:               "json simple override",
			provider:           `{"type": "openstack"}`,
			discoveredPlatform: noPlatform,
			discoveredMasters:  simpleMasters,
			discoveredNetwork:  sdnConfig,
			expectedConfig:     `{"type":"openstack","ProjectID":"","Region":"","Zone":"","NumNodes":3,"MultiMaster":true,"MultiZone":false,"Zones":[],"ConfigFile":"","Disconnected":false,"NetworkPlugin":"OpenShiftSDN","HasIPv4":true,"HasIPv6":false,"HasSCTP":false}`,
			runTests:           sets.NewString("everyone", "not-gce", "not-aws", "not-multitenant", "online", "ipv4"),
		},
		{
			name:               "complex override, dual-stack",
			provider:           `{"type":"aws","region":"us-east-2","zone":"us-east-2a","multimaster":false,"multizone":true}`,
			discoveredPlatform: awsPlatform,
			discoveredMasters:  simpleMasters,
			discoveredNetwork:  ovnKubernetesConfig,
			expectedConfig:     `{"type":"aws","ProjectID":"","Region":"us-east-2","Zone":"us-east-2a","NumNodes":3,"MultiMaster":false,"MultiZone":true,"Zones":[],"ConfigFile":"","Disconnected":false,"NetworkPlugin":"OVNKubernetes","HasIPv4":true,"HasIPv6":true,"HasSCTP":false}`,
			runTests:           sets.NewString("everyone", "not-gce", "not-sdn", "not-multitenant", "online", "ipv4", "ipv6", "dual-stack"),
		},
		{
			name:               "complex override without discovery",
			provider:           `{"type":"aws","region":"us-east-2","zone":"us-east-2a","multimaster":false,"multizone":true}`,
			discoveredPlatform: nil,
			expectedConfig:     `{"type":"aws","ProjectID":"","Region":"us-east-2","Zone":"us-east-2a","NumNodes":0,"MultiMaster":false,"MultiZone":true,"Zones":null,"ConfigFile":"","Disconnected":false,"NetworkPlugin":"","HasIPv4":false,"HasIPv6":false,"HasSCTP":false}`,
			runTests:           sets.NewString("everyone", "not-gce", "not-sdn", "not-multitenant", "online"),
		},
		{
			name:               "disconnected",
			provider:           `{"type":"none","disconnected":true}`,
			discoveredPlatform: noPlatform,
			discoveredMasters:  simpleMasters,
			discoveredNetwork:  ovnKubernetesConfig,
			expectedConfig:     `{"type":"none","ProjectID":"","Region":"","Zone":"","NumNodes":3,"MultiMaster":true,"MultiZone":false,"Zones":[],"ConfigFile":"","Disconnected":true,"NetworkPlugin":"OVNKubernetes","HasIPv4":true,"HasIPv6":true,"HasSCTP":false}`,
			runTests:           sets.NewString("everyone", "not-gce", "not-aws", "not-sdn", "not-multitenant", "ipv4", "ipv6", "dual-stack"),
		},
		{
			name:               "override network plugin",
			provider:           `{"type":"aws","networkPlugin":"Calico","hasIPv4":false,"hasIPv6":true,"hasSCTP":true}`,
			discoveredPlatform: awsPlatform,
			discoveredMasters:  simpleMasters,
			discoveredNetwork:  ovnKubernetesConfig,
			expectedConfig:     `{"type":"aws","ProjectID":"","Region":"us-east-2","Zone":"","NumNodes":3,"MultiMaster":true,"MultiZone":false,"Zones":[],"ConfigFile":"","Disconnected":false,"NetworkPlugin":"Calico","HasIPv4":false,"HasIPv6":true,"HasSCTP":true}`,
			runTests:           sets.NewString("everyone", "not-gce", "not-sdn", "not-multitenant", "online", "ipv6", "sctp"),
		},
	}

	// Unset these to keep decodeProvider from returning "local"
	os.Unsetenv("KUBE_SSH_USER")
	os.Unsetenv("LOCAL_SSH_KEY")

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			discover := tc.discoveredPlatform != nil
			var testState *exutilcluster.ClusterState
			if discover {
				testState = &exutilcluster.ClusterState{
					PlatformStatus: tc.discoveredPlatform,
					Masters:        tc.discoveredMasters,
					NonMasters:     nonMasters,
					NetworkSpec:    tc.discoveredNetwork,
				}
			}
			config, err := decodeProvider(tc.provider, false, discover, testState)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			configJSON := config.ToJSONString()
			if configJSON != tc.expectedConfig {
				t.Fatalf("Generated config:\n%s\ndoes not match expected:\n%s\n", configJSON, tc.expectedConfig)
			}
			matchFn := config.MatchFn()

			runTests := sets.NewString()
			for name, tags := range e2eTests {
				if matchFn(name + " " + tags) {
					runTests.Insert(name)
				}
			}
			if !runTests.Equal(tc.runTests) {
				t.Fatalf("Matched tests:\n%v\ndid not match expected:\n%v\n", runTests.List(), tc.runTests.List())
			}
		})
	}
}
