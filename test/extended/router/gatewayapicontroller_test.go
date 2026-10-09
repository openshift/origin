package router

import (
	"errors"
	"strings"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
)

func TestEvaluateGatewayAPITestEligibility(t *testing.T) {
	const (
		ipv6SkipReason    = "Skipping Gateway API tests on IPv6/dual-stack cluster"
		missingOLMReason  = "Skipping: OLM/Marketplace capabilities are not enabled and GatewayAPIWithoutOLM is not enabled"
		unsupportedReason = "Skipping on unsupported platform type \"OpenStack\""
		okdReason         = "Skipping on OKD cluster as OSSM is not available as a community operator"
	)

	tests := []struct {
		name                   string
		serviceNetworks        []string
		noOLM                  bool
		allowIPv6WithNoOLM     bool
		isOKD                  bool
		platformType           configv1.PlatformType
		olmCapabilitiesEnabled bool
		wantCapabilitiesCheck  bool
		wantSkip               bool
		wantReason             string
	}{
		{
			name:                   "management mode allows IPv4 without OLM",
			serviceNetworks:        []string{"172.30.0.0/16"},
			noOLM:                  true,
			allowIPv6WithNoOLM:     true,
			platformType:           configv1.AWSPlatformType,
			olmCapabilitiesEnabled: false,
		},
		{
			name:                   "management mode allows IPv6 without OLM",
			serviceNetworks:        []string{"fd02::/112"},
			noOLM:                  true,
			allowIPv6WithNoOLM:     true,
			platformType:           configv1.AWSPlatformType,
			olmCapabilitiesEnabled: false,
		},
		{
			name:                   "management mode allows dual-stack without OLM",
			serviceNetworks:        []string{"172.30.0.0/16", "fd02::/112"},
			noOLM:                  true,
			allowIPv6WithNoOLM:     true,
			platformType:           configv1.BareMetalPlatformType,
			olmCapabilitiesEnabled: false,
		},
		{
			name:                   "legacy OLM allows IPv4",
			serviceNetworks:        []string{"172.30.0.0/16"},
			allowIPv6WithNoOLM:     true,
			platformType:           configv1.AzurePlatformType,
			olmCapabilitiesEnabled: true,
			wantCapabilitiesCheck:  true,
		},
		{
			name:                   "legacy OLM skips IPv6",
			serviceNetworks:        []string{"fd02::/112"},
			allowIPv6WithNoOLM:     true,
			platformType:           configv1.GCPPlatformType,
			olmCapabilitiesEnabled: true,
			wantSkip:               true,
			wantReason:             ipv6SkipReason,
		},
		{
			name:                   "legacy OLM skips dual-stack",
			serviceNetworks:        []string{"172.30.0.0/16", "fd02::/112"},
			allowIPv6WithNoOLM:     true,
			platformType:           configv1.VSpherePlatformType,
			olmCapabilitiesEnabled: true,
			wantSkip:               true,
			wantReason:             ipv6SkipReason,
		},
		{
			name:                   "default route still skips IPv6 without OLM",
			serviceNetworks:        []string{"fd02::/112"},
			noOLM:                  true,
			platformType:           configv1.AWSPlatformType,
			olmCapabilitiesEnabled: true,
			wantSkip:               true,
			wantReason:             ipv6SkipReason,
		},
		{
			name:                   "default route still skips dual-stack without OLM",
			serviceNetworks:        []string{"172.30.0.0/16", "fd02::/112"},
			noOLM:                  true,
			platformType:           configv1.BareMetalPlatformType,
			olmCapabilitiesEnabled: true,
			wantSkip:               true,
			wantReason:             ipv6SkipReason,
		},
		{
			name:                   "legacy OLM requires capabilities",
			serviceNetworks:        []string{"172.30.0.0/16"},
			platformType:           configv1.IBMCloudPlatformType,
			olmCapabilitiesEnabled: false,
			wantCapabilitiesCheck:  true,
			wantSkip:               true,
			wantReason:             missingOLMReason,
		},
		{
			name:                   "unsupported platform remains skipped",
			serviceNetworks:        []string{"fd02::/112"},
			noOLM:                  true,
			allowIPv6WithNoOLM:     true,
			platformType:           configv1.OpenStackPlatformType,
			olmCapabilitiesEnabled: true,
			wantSkip:               true,
			wantReason:             unsupportedReason,
		},
		{
			name:                   "OKD remains skipped",
			serviceNetworks:        []string{"172.30.0.0/16"},
			noOLM:                  true,
			allowIPv6WithNoOLM:     true,
			isOKD:                  true,
			platformType:           configv1.AWSPlatformType,
			olmCapabilitiesEnabled: true,
			wantSkip:               true,
			wantReason:             okdReason,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			olmCapabilitiesChecked := false
			checks := gatewayAPITestEligibilityChecks{
				isOKD: func() (bool, error) {
					return tt.isOKD, nil
				},
				platformType: func() (configv1.PlatformType, error) {
					return tt.platformType, nil
				},
				isIPv6OrDualStack: func() (bool, error) {
					return hasIPv6ServiceNetwork(tt.serviceNetworks), nil
				},
				allOLMCapabilitiesEnabled: func() (bool, error) {
					olmCapabilitiesChecked = true
					return tt.olmCapabilitiesEnabled, nil
				},
			}

			skip, reason, err := evaluateGatewayAPITestEligibility(tt.noOLM, gatewayAPITestEligibilityOptions{
				allowIPv6WithNoOLM: tt.allowIPv6WithNoOLM,
			}, checks)
			if err != nil {
				t.Fatalf("evaluateGatewayAPITestEligibility() returned unexpected error: %v", err)
			}
			if skip != tt.wantSkip {
				t.Errorf("evaluateGatewayAPITestEligibility() skip = %t, want %t", skip, tt.wantSkip)
			}
			if reason != tt.wantReason {
				t.Errorf("evaluateGatewayAPITestEligibility() reason = %q, want %q", reason, tt.wantReason)
			}
			if olmCapabilitiesChecked != tt.wantCapabilitiesCheck {
				t.Errorf("OLM capabilities checked = %t, want %t", olmCapabilitiesChecked, tt.wantCapabilitiesCheck)
			}
		})
	}
}

func TestEvaluateGatewayAPITestEligibilityErrors(t *testing.T) {
	checkErr := errors.New("check failed")
	tests := []struct {
		name        string
		failedCheck string
		noOLM       bool
		wantMessage string
	}{
		{
			name:        "OKD detection error",
			failedCheck: "okd",
			noOLM:       true,
			wantMessage: "failed to determine if release is OKD",
		},
		{
			name:        "infrastructure error",
			failedCheck: "platform",
			noOLM:       true,
			wantMessage: "check failed",
		},
		{
			name:        "network error",
			failedCheck: "network",
			noOLM:       true,
			wantMessage: "failed to check IPv6/dual-stack",
		},
		{
			name:        "capability error",
			failedCheck: "capabilities",
			wantMessage: "failed to check OLM capabilities",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checks := gatewayAPITestEligibilityChecks{
				isOKD: func() (bool, error) {
					if tt.failedCheck == "okd" {
						return false, checkErr
					}
					return false, nil
				},
				platformType: func() (configv1.PlatformType, error) {
					if tt.failedCheck == "platform" {
						return "", checkErr
					}
					return configv1.AWSPlatformType, nil
				},
				isIPv6OrDualStack: func() (bool, error) {
					if tt.failedCheck == "network" {
						return false, checkErr
					}
					return false, nil
				},
				allOLMCapabilitiesEnabled: func() (bool, error) {
					if tt.failedCheck == "capabilities" {
						return false, checkErr
					}
					return true, nil
				},
			}

			skip, reason, err := evaluateGatewayAPITestEligibility(tt.noOLM, gatewayAPITestEligibilityOptions{
				allowIPv6WithNoOLM: true,
			}, checks)
			if err == nil {
				t.Fatal("evaluateGatewayAPITestEligibility() returned nil error")
			}
			if !errors.Is(err, checkErr) {
				t.Errorf("evaluateGatewayAPITestEligibility() error = %v, want wrapped check error", err)
			}
			if !strings.Contains(err.Error(), tt.wantMessage) {
				t.Errorf("evaluateGatewayAPITestEligibility() error = %q, want it to contain %q", err, tt.wantMessage)
			}
			if skip || reason != "" {
				t.Errorf("evaluateGatewayAPITestEligibility() = (%t, %q, %v), want (false, empty reason, error)", skip, reason, err)
			}
		})
	}
}
