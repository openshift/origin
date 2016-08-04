package scc

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
)

func TestPointValue(t *testing.T) {
	newSCC := func(priv bool, seLinuxStrategy kapi.SELinuxContextStrategyType, userStrategy kapi.RunAsUserStrategyType) *kapi.SecurityContextConstraints {
		scc := &kapi.SecurityContextConstraints{
			SELinuxContext: kapi.SELinuxContextStrategyOptions{
				Type: seLinuxStrategy,
			},
			RunAsUser: kapi.RunAsUserStrategyOptions{
				Type: userStrategy,
			},
		}
		if priv {
			scc.AllowPrivilegedContainer = true
		}

		return scc
	}

	seLinuxStrategies := map[kapi.SELinuxContextStrategyType]int{
		kapi.SELinuxStrategyRunAsAny:  4,
		kapi.SELinuxStrategyMustRunAs: 1,
	}
	userStrategies := map[kapi.RunAsUserStrategyType]int{
		kapi.RunAsUserStrategyRunAsAny:         4,
		kapi.RunAsUserStrategyMustRunAsNonRoot: 3,
		kapi.RunAsUserStrategyMustRunAsRange:   2,
		kapi.RunAsUserStrategyMustRunAs:        1,
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
	scc := newSCC(false, kapi.SELinuxStrategyMustRunAs, kapi.RunAsUserStrategyMustRunAs)
	scc.Volumes = []kapi.FSType{kapi.FSTypeHostPath}
	actualPoints := pointValue(scc)
	if actualPoints != 12 { //1 (SELinux) + 1 (User) + 10 (host path volume)
		t.Errorf("volume score was not added to the scc point value correctly!")
	}
}

func TestVolumePointValue(t *testing.T) {
	newSCC := func(host, nonTrivial, trivial bool) *kapi.SecurityContextConstraints {
		volumes := []kapi.FSType{}
		if host {
			volumes = append(volumes, kapi.FSTypeHostPath)
		}
		if nonTrivial {
			volumes = append(volumes, kapi.FSTypeAWSElasticBlockStore)
		}
		if trivial {
			volumes = append(volumes, kapi.FSTypeSecret)
		}
		return &kapi.SecurityContextConstraints{
			Volumes: volumes,
		}
	}

	allowAllSCC := &kapi.SecurityContextConstraints{
		Volumes: []kapi.FSType{kapi.FSTypeAll},
	}
	nilVolumeSCC := &kapi.SecurityContextConstraints{}

	tests := map[string]struct {
		scc            *kapi.SecurityContextConstraints
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
			scc: &kapi.SecurityContextConstraints{
				Volumes: []kapi.FSType{kapi.FSTypeSecret},
			},
			expectedPoints: 0,
		},
		"trivial - configMap": {
			scc: &kapi.SecurityContextConstraints{
				Volumes: []kapi.FSType{kapi.FSTypeConfigMap},
			},
			expectedPoints: 0,
		},
		"trivial - emptyDir": {
			scc: &kapi.SecurityContextConstraints{
				Volumes: []kapi.FSType{kapi.FSTypeEmptyDir},
			},
			expectedPoints: 0,
		},
		"trivial - downwardAPI": {
			scc: &kapi.SecurityContextConstraints{
				Volumes: []kapi.FSType{kapi.FSTypeDownwardAPI},
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
