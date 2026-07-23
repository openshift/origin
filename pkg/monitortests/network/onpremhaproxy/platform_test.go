package onpremhaproxy

import (
	"testing"

	"github.com/stretchr/testify/assert"

	configv1 "github.com/openshift/api/config/v1"
)

func TestNotSupportedPlatformReason(t *testing.T) {
	tests := []struct {
		name            string
		platformStatus  *configv1.PlatformStatus
		expectSupported bool
	}{
		{
			name:            "no platform status",
			platformStatus:  nil,
			expectSupported: false,
		},
		{
			name: "aws",
			platformStatus: &configv1.PlatformStatus{
				Type: configv1.AWSPlatformType,
				AWS:  &configv1.AWSPlatformStatus{Region: "us-east-1"},
			},
			expectSupported: false,
		},
		{
			name: "platform none",
			platformStatus: &configv1.PlatformStatus{
				Type: configv1.NonePlatformType,
			},
			expectSupported: false,
		},
		{
			name: "nutanix with API VIP is not scanned by this monitor test",
			platformStatus: &configv1.PlatformStatus{
				Type: configv1.NutanixPlatformType,
				Nutanix: &configv1.NutanixPlatformStatus{
					APIServerInternalIPs: []string{"192.168.111.5"},
				},
			},
			expectSupported: false,
		},
		{
			name: "baremetal with API VIPs",
			platformStatus: &configv1.PlatformStatus{
				Type: configv1.BareMetalPlatformType,
				BareMetal: &configv1.BareMetalPlatformStatus{
					APIServerInternalIPs: []string{"192.168.111.5"},
				},
			},
			expectSupported: true,
		},
		{
			name: "baremetal with only the deprecated API VIP field",
			platformStatus: &configv1.PlatformStatus{
				Type: configv1.BareMetalPlatformType,
				BareMetal: &configv1.BareMetalPlatformStatus{
					APIServerInternalIP: "192.168.111.5",
				},
			},
			expectSupported: true,
		},
		{
			name: "baremetal with the default loadbalancer set explicitly",
			platformStatus: &configv1.PlatformStatus{
				Type: configv1.BareMetalPlatformType,
				BareMetal: &configv1.BareMetalPlatformStatus{
					APIServerInternalIPs: []string{"192.168.111.5"},
					LoadBalancer: &configv1.BareMetalPlatformLoadBalancer{
						Type: configv1.LoadBalancerTypeOpenShiftManagedDefault,
					},
				},
			},
			expectSupported: true,
		},
		{
			name: "baremetal with a user-managed loadbalancer",
			platformStatus: &configv1.PlatformStatus{
				Type: configv1.BareMetalPlatformType,
				BareMetal: &configv1.BareMetalPlatformStatus{
					APIServerInternalIPs: []string{"192.168.111.5"},
					LoadBalancer: &configv1.BareMetalPlatformLoadBalancer{
						Type: configv1.LoadBalancerTypeUserManaged,
					},
				},
			},
			expectSupported: false,
		},
		{
			name: "baremetal without platform-specific status",
			platformStatus: &configv1.PlatformStatus{
				Type: configv1.BareMetalPlatformType,
			},
			expectSupported: false,
		},
		{
			name: "openstack with API VIPs",
			platformStatus: &configv1.PlatformStatus{
				Type: configv1.OpenStackPlatformType,
				OpenStack: &configv1.OpenStackPlatformStatus{
					APIServerInternalIPs: []string{"10.0.0.5"},
				},
			},
			expectSupported: true,
		},
		{
			name: "openstack without API VIP",
			platformStatus: &configv1.PlatformStatus{
				Type:      configv1.OpenStackPlatformType,
				OpenStack: &configv1.OpenStackPlatformStatus{},
			},
			expectSupported: false,
		},
		{
			name: "vsphere IPI with API VIPs",
			platformStatus: &configv1.PlatformStatus{
				Type: configv1.VSpherePlatformType,
				VSphere: &configv1.VSpherePlatformStatus{
					APIServerInternalIPs: []string{"10.0.0.5"},
				},
			},
			expectSupported: true,
		},
		{
			name: "vsphere UPI without API VIP",
			platformStatus: &configv1.PlatformStatus{
				Type:    configv1.VSpherePlatformType,
				VSphere: &configv1.VSpherePlatformStatus{},
			},
			expectSupported: false,
		},
		{
			name: "vsphere without platform-specific status",
			platformStatus: &configv1.PlatformStatus{
				Type: configv1.VSpherePlatformType,
			},
			expectSupported: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			infra := &configv1.Infrastructure{
				Status: configv1.InfrastructureStatus{
					PlatformStatus: tt.platformStatus,
				},
			}

			reason := notSupportedPlatformReason(infra)
			if tt.expectSupported {
				assert.Empty(t, reason, "expected the platform to be supported")
			} else {
				assert.NotEmpty(t, reason, "expected the platform not to be supported")
			}
		})
	}
}
