package scc

import (
	"testing"

	securityapi "github.com/openshift/origin/pkg/security/apis/security"
)

func TestPointValue(t *testing.T) {
	newSCC := func(priv bool, seLinuxStrategy securityapi.SELinuxContextStrategyType, userStrategy securityapi.RunAsUserStrategyType) *securityapi.SecurityContextConstraints {
		return &securityapi.SecurityContextConstraints{
			AllowPrivilegedContainer: priv,
			SELinuxContext: securityapi.SELinuxContextStrategyOptions{
				Type: seLinuxStrategy,
			},
			RunAsUser: securityapi.RunAsUserStrategyOptions{
				Type: userStrategy,
			},
		}
	}

	seLinuxStrategies := map[securityapi.SELinuxContextStrategyType]points{
		securityapi.SELinuxStrategyRunAsAny:  runAsAnyUserPoints,
		securityapi.SELinuxStrategyMustRunAs: runAsUserPoints,
	}
	userStrategies := map[securityapi.RunAsUserStrategyType]points{
		securityapi.RunAsUserStrategyRunAsAny:         runAsAnyUserPoints,
		securityapi.RunAsUserStrategyMustRunAsNonRoot: runAsNonRootPoints,
		securityapi.RunAsUserStrategyMustRunAsRange:   runAsRangePoints,
		securityapi.RunAsUserStrategyMustRunAs:        runAsUserPoints,
	}

	// run through all combos of user strategy + seLinux strategy + priv
	for userStrategy, userStrategyPoints := range userStrategies {
		for seLinuxStrategy, seLinuxStrategyPoints := range seLinuxStrategies {
			expectedPoints := privilegedPoints + userStrategyPoints + seLinuxStrategyPoints
			scc := newSCC(true, seLinuxStrategy, userStrategy)
			actualPoints := pointValue(scc)

			if actualPoints != expectedPoints {
				t.Errorf("privileged, user: %v, seLinux %v expected %d score but got %d", userStrategy, seLinuxStrategy, expectedPoints, actualPoints)
			}

			expectedPoints = userStrategyPoints + seLinuxStrategyPoints
			scc = newSCC(false, seLinuxStrategy, userStrategy)
			actualPoints = pointValue(scc)

			if actualPoints != expectedPoints {
				t.Errorf("non privileged, user: %v, seLinux %v expected %d score but got %d", userStrategy, seLinuxStrategy, expectedPoints, actualPoints)
			}
		}
	}

	// sanity check to ensure volume score is added (specific volumes scores are tested below
	scc := newSCC(false, securityapi.SELinuxStrategyMustRunAs, securityapi.RunAsUserStrategyMustRunAs)
	scc.Volumes = []securityapi.FSType{securityapi.FSTypeHostPath}
	actualPoints := pointValue(scc)
	// SELinux + User + host path volume
	expectedPoints := runAsUserPoints + runAsUserPoints + hostVolumePoints
	if actualPoints != expectedPoints {
		t.Errorf("volume score was not added to the scc point value correctly!")
	}
}

func TestVolumePointValue(t *testing.T) {
	newSCC := func(host, nonTrivial, trivial bool) *securityapi.SecurityContextConstraints {
		volumes := []securityapi.FSType{}
		if host {
			volumes = append(volumes, securityapi.FSTypeHostPath)
		}
		if nonTrivial {
			volumes = append(volumes, securityapi.FSTypeAWSElasticBlockStore)
		}
		if trivial {
			volumes = append(volumes, securityapi.FSTypeSecret)
		}
		return &securityapi.SecurityContextConstraints{
			Volumes: volumes,
		}
	}

	allowAllSCC := &securityapi.SecurityContextConstraints{
		Volumes: []securityapi.FSType{securityapi.FSTypeAll},
	}
	nilVolumeSCC := &securityapi.SecurityContextConstraints{}

	tests := map[string]struct {
		scc            *securityapi.SecurityContextConstraints
		expectedPoints points
	}{
		"all volumes": {
			scc:            allowAllSCC,
			expectedPoints: hostVolumePoints,
		},
		"host volume": {
			scc:            newSCC(true, false, false),
			expectedPoints: hostVolumePoints,
		},
		"host volume and non trivial volumes": {
			scc:            newSCC(true, true, false),
			expectedPoints: hostVolumePoints,
		},
		"host volume, non trivial, and trivial": {
			scc:            newSCC(true, true, true),
			expectedPoints: hostVolumePoints,
		},
		"non trivial": {
			scc:            newSCC(false, true, false),
			expectedPoints: nonTrivialVolumePoints,
		},
		"non trivial and trivial": {
			scc:            newSCC(false, true, true),
			expectedPoints: nonTrivialVolumePoints,
		},
		"trivial": {
			scc:            newSCC(false, false, true),
			expectedPoints: noPoints,
		},
		"trivial - secret": {
			scc: &securityapi.SecurityContextConstraints{
				Volumes: []securityapi.FSType{securityapi.FSTypeSecret},
			},
			expectedPoints: noPoints,
		},
		"trivial - configMap": {
			scc: &securityapi.SecurityContextConstraints{
				Volumes: []securityapi.FSType{securityapi.FSTypeConfigMap},
			},
			expectedPoints: noPoints,
		},
		"trivial - emptyDir": {
			scc: &securityapi.SecurityContextConstraints{
				Volumes: []securityapi.FSType{securityapi.FSTypeEmptyDir},
			},
			expectedPoints: noPoints,
		},
		"trivial - downwardAPI": {
			scc: &securityapi.SecurityContextConstraints{
				Volumes: []securityapi.FSType{securityapi.FSTypeDownwardAPI},
			},
			expectedPoints: noPoints,
		},
		"trivial - projected": {
			scc: &securityapi.SecurityContextConstraints{
				Volumes: []securityapi.FSType{securityapi.FSProjected},
			},
			expectedPoints: noPoints,
		},
		"trivial - none": {
			scc: &securityapi.SecurityContextConstraints{
				Volumes: []securityapi.FSType{securityapi.FSTypeNone},
			},
			expectedPoints: noPoints,
		},
		"no volumes allowed": {
			scc:            newSCC(false, false, false),
			expectedPoints: noPoints,
		},
		"nil volumes": {
			scc:            nilVolumeSCC,
			expectedPoints: noPoints,
		},
	}
	for k, v := range tests {
		actualPoints := volumePointValue(v.scc)
		if actualPoints != v.expectedPoints {
			t.Errorf("%s expected %d volume score but got %d", k, v.expectedPoints, actualPoints)
		}
	}
}
