package util

import (
	"reflect"
	"testing"

	api "k8s.io/kubernetes/pkg/apis/core"

	securityapi "github.com/openshift/origin/pkg/security/apis/security"
)

// TestVolumeSourceFSTypeDrift ensures that for every known type of volume source (by the fields on
// a VolumeSource object that GetVolumeFSType is returning a good value.  This ensures both that we're
// returning an FSType for the VolumeSource field (protect the GetVolumeFSType method) and that we
// haven't drifted (ensure new fields in VolumeSource are covered).
func TestVolumeSourceFSTypeDrift(t *testing.T) {
	allFSTypes := GetAllFSTypesAsSet()
	val := reflect.ValueOf(api.VolumeSource{})

	for i := 0; i < val.NumField(); i++ {
		fieldVal := val.Type().Field(i)

		volumeSource := api.VolumeSource{}
		volumeSourceVolume := reflect.New(fieldVal.Type.Elem())

		reflect.ValueOf(&volumeSource).Elem().FieldByName(fieldVal.Name).Set(volumeSourceVolume)

		fsType, err := GetVolumeFSType(api.Volume{VolumeSource: volumeSource})
		if err != nil {
			t.Errorf("error getting fstype for field %s.  This likely means that drift has occured between FSType and VolumeSource.  Please update the api and getVolumeFSType", fieldVal.Name)
		}

		if !allFSTypes.Has(string(fsType)) {
			t.Errorf("%s was missing from GetFSTypesAsSet", fsType)
		}
	}
}

func TestSCCAllowsVolumeType(t *testing.T) {
	tests := map[string]struct {
		scc    *securityapi.SecurityContextConstraints
		fsType securityapi.FSType
		allows bool
	}{
		"nil scc": {
			scc:    nil,
			fsType: securityapi.FSTypeHostPath,
			allows: false,
		},
		"empty volumes": {
			scc:    &securityapi.SecurityContextConstraints{},
			fsType: securityapi.FSTypeHostPath,
			allows: false,
		},
		"non-matching": {
			scc: &securityapi.SecurityContextConstraints{
				Volumes: []securityapi.FSType{securityapi.FSTypeAWSElasticBlockStore},
			},
			fsType: securityapi.FSTypeHostPath,
			allows: false,
		},
		"match on FSTypeAll": {
			scc: &securityapi.SecurityContextConstraints{
				Volumes: []securityapi.FSType{securityapi.FSTypeAll},
			},
			fsType: securityapi.FSTypeHostPath,
			allows: true,
		},
		"match on direct match": {
			scc: &securityapi.SecurityContextConstraints{
				Volumes: []securityapi.FSType{securityapi.FSTypeHostPath},
			},
			fsType: securityapi.FSTypeHostPath,
			allows: true,
		},
	}

	for k, v := range tests {
		allows := SCCAllowsFSType(v.scc, v.fsType)
		if v.allows != allows {
			t.Errorf("%s expected SCCAllowsFSType to return %t but got %t", k, v.allows, allows)
		}
	}
}

func TestEqualStringSlices(t *testing.T) {
	tests := map[string]struct {
		arg1           []string
		arg2           []string
		expectedResult bool
	}{
		"nil equals to nil": {
			arg1:           nil,
			arg2:           nil,
			expectedResult: true,
		},
		"equal by size": {
			arg1:           []string{"1", "1"},
			arg2:           []string{"1", "1"},
			expectedResult: true,
		},
		"not equal by size": {
			arg1:           []string{"1"},
			arg2:           []string{"1", "1"},
			expectedResult: false,
		},
		"not equal by elements": {
			arg1:           []string{"1", "1"},
			arg2:           []string{"1", "2"},
			expectedResult: false,
		},
	}

	for k, v := range tests {
		if result := EqualStringSlices(v.arg1, v.arg2); result != v.expectedResult {
			t.Errorf("%s expected to return %t but got %t", k, v.expectedResult, result)
		}
	}
}
