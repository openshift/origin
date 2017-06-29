package scc

import (
	"testing"

	securityapi "github.com/openshift/origin/pkg/security/apis/security"
)

func TestPointValue(t *testing.T) {
	newSCC := func(priv bool, seLinuxStrategy securityapi.SELinuxContextStrategyType, userStrategy securityapi.RunAsUserStrategyType) *securityapi.SecurityContextConstraints {
		scc := &securityapi.SecurityContextConstraints{
			SELinuxContext: securityapi.SELinuxContextStrategyOptions{
				Type: seLinuxStrategy,
			},
			RunAsUser: securityapi.RunAsUserStrategyOptions{
				Type: userStrategy,
			},
		}
		if priv {
			scc.AllowPrivilegedContainer = true
		}

		return scc
	}

	seLinuxStrategies := map[securityapi.SELinuxContextStrategyType]int{
		securityapi.SELinuxStrategyRunAsAny:  4,
		securityapi.SELinuxStrategyMustRunAs: 1,
	}
	userStrategies := map[securityapi.RunAsUserStrategyType]int{
		securityapi.RunAsUserStrategyRunAsAny:         4,
		securityapi.RunAsUserStrategyMustRunAsNonRoot: 3,
		securityapi.RunAsUserStrategyMustRunAsRange:   2,
		securityapi.RunAsUserStrategyMustRunAs:        1,
	}

	privilegedPoints := 20

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
	if actualPoints != 12 { //1 (SELinux) + 1 (User) + 10 (host path volume)
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
		expectedPoints int
	}{
		"all volumes": {
			scc:            allowAllSCC,
			expectedPoints: 10,
		},
		"host volume": {
			scc:            newSCC(true, false, false),
			expectedPoints: 10,
		},
		"host volume and non trivial volumes": {
			scc:            newSCC(true, true, false),
			expectedPoints: 10,
		},
		"host volume, non trivial, and trivial": {
			scc:            newSCC(true, true, true),
			expectedPoints: 10,
		},
		"non trivial": {
			scc:            newSCC(false, true, false),
			expectedPoints: 5,
		},
		"non trivial and trivial": {
			scc:            newSCC(false, true, true),
			expectedPoints: 5,
		},
		"trivial": {
			scc:            newSCC(false, false, true),
			expectedPoints: 0,
		},
		"trivial - secret": {
			scc: &securityapi.SecurityContextConstraints{
				Volumes: []securityapi.FSType{securityapi.FSTypeSecret},
			},
			expectedPoints: 0,
		},
		"trivial - configMap": {
			scc: &securityapi.SecurityContextConstraints{
				Volumes: []securityapi.FSType{securityapi.FSTypeConfigMap},
			},
			expectedPoints: 0,
		},
		"trivial - emptyDir": {
			scc: &securityapi.SecurityContextConstraints{
				Volumes: []securityapi.FSType{securityapi.FSTypeEmptyDir},
			},
			expectedPoints: 0,
		},
		"trivial - downwardAPI": {
			scc: &securityapi.SecurityContextConstraints{
				Volumes: []securityapi.FSType{securityapi.FSTypeDownwardAPI},
			},
			expectedPoints: 0,
		},
		"trivial - projected": {
			scc: &securityapi.SecurityContextConstraints{
				Volumes: []securityapi.FSType{securityapi.FSProjected},
			},
			expectedPoints: 0,
		},
		"trivial - none": {
			scc: &securityapi.SecurityContextConstraints{
				Volumes: []securityapi.FSType{securityapi.FSTypeNone},
			},
			expectedPoints: 0,
		},
		"no volumes allowed": {
			scc:            newSCC(false, false, false),
			expectedPoints: 0,
		},
		"nil volumes": {
			scc:            nilVolumeSCC,
			expectedPoints: 0,
		},
	}
	for k, v := range tests {
		actualPoints := volumePointValue(v.scc)
		if actualPoints != v.expectedPoints {
			t.Errorf("%s expected %d volume score but got %d", k, v.expectedPoints, actualPoints)
		}
	}
}
