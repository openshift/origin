package v1_test

import (
	"reflect"
	"testing"

	sccutil "github.com/openshift/origin/pkg/security/securitycontextconstraints/util"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	versioned "github.com/openshift/api/security/v1"
	_ "github.com/openshift/origin/pkg/api/install"
	conversionv1 "github.com/openshift/origin/pkg/security/apis/security/v1"
)

func roundTrip(t *testing.T, obj runtime.Object) runtime.Object {
	codec := legacyscheme.Codecs.LegacyCodec(versioned.SchemeGroupVersion)
	data, err := runtime.Encode(codec, obj)
	if err != nil {
		t.Errorf("%v\n %#v", err, obj)
		return nil
	}
	obj2, err := runtime.Decode(codec, data)
	if err != nil {
		t.Errorf("%v\nData: %s\nSource: %#v", err, string(data), obj)
		return nil
	}
	obj3 := reflect.New(reflect.TypeOf(obj).Elem()).Interface().(runtime.Object)
	err = legacyscheme.Scheme.Convert(obj2, obj3, nil)
	if err != nil {
		t.Errorf("%v\nSource: %#v", err, obj2)
		return nil
	}
	return obj3
}

func TestDefaultSecurityContextConstraints(t *testing.T) {
	tests := map[string]struct {
		scc              *versioned.SecurityContextConstraints
		expectedFSGroup  versioned.FSGroupStrategyType
		expectedSupGroup versioned.SupplementalGroupsStrategyType
	}{
		"shouldn't default": {
			scc: &versioned.SecurityContextConstraints{
				FSGroup: versioned.FSGroupStrategyOptions{
					Type: versioned.FSGroupStrategyMustRunAs,
				},
				SupplementalGroups: versioned.SupplementalGroupsStrategyOptions{
					Type: versioned.SupplementalGroupsStrategyMustRunAs,
				},
			},
			expectedFSGroup:  versioned.FSGroupStrategyMustRunAs,
			expectedSupGroup: versioned.SupplementalGroupsStrategyMustRunAs,
		},
		"default fsgroup runAsAny": {
			scc: &versioned.SecurityContextConstraints{
				RunAsUser: versioned.RunAsUserStrategyOptions{
					Type: versioned.RunAsUserStrategyRunAsAny,
				},
				SupplementalGroups: versioned.SupplementalGroupsStrategyOptions{
					Type: versioned.SupplementalGroupsStrategyMustRunAs,
				},
			},
			expectedFSGroup:  versioned.FSGroupStrategyRunAsAny,
			expectedSupGroup: versioned.SupplementalGroupsStrategyMustRunAs,
		},
		"default sup group runAsAny": {
			scc: &versioned.SecurityContextConstraints{
				RunAsUser: versioned.RunAsUserStrategyOptions{
					Type: versioned.RunAsUserStrategyRunAsAny,
				},
				FSGroup: versioned.FSGroupStrategyOptions{
					Type: versioned.FSGroupStrategyMustRunAs,
				},
			},
			expectedFSGroup:  versioned.FSGroupStrategyMustRunAs,
			expectedSupGroup: versioned.SupplementalGroupsStrategyRunAsAny,
		},
		"default fsgroup runAsAny with mustRunAs UID strat": {
			scc: &versioned.SecurityContextConstraints{
				RunAsUser: versioned.RunAsUserStrategyOptions{
					Type: versioned.RunAsUserStrategyMustRunAsRange,
				},
				SupplementalGroups: versioned.SupplementalGroupsStrategyOptions{
					Type: versioned.SupplementalGroupsStrategyMustRunAs,
				},
			},
			expectedFSGroup:  versioned.FSGroupStrategyRunAsAny,
			expectedSupGroup: versioned.SupplementalGroupsStrategyMustRunAs,
		},
		"default sup group runAsAny with mustRunAs UID strat": {
			scc: &versioned.SecurityContextConstraints{
				RunAsUser: versioned.RunAsUserStrategyOptions{
					Type: versioned.RunAsUserStrategyMustRunAsRange,
				},
				FSGroup: versioned.FSGroupStrategyOptions{
					Type: versioned.FSGroupStrategyMustRunAs,
				},
			},
			expectedFSGroup:  versioned.FSGroupStrategyMustRunAs,
			expectedSupGroup: versioned.SupplementalGroupsStrategyRunAsAny,
		},
	}
	for k, v := range tests {
		output := roundTrip(t, runtime.Object(v.scc))
		scc := output.(*versioned.SecurityContextConstraints)

		if scc.FSGroup.Type != v.expectedFSGroup {
			t.Errorf("%s has invalid fsgroup.  Expected: %v got: %v", k, v.expectedFSGroup, scc.FSGroup.Type)
		}
		if scc.SupplementalGroups.Type != v.expectedSupGroup {
			t.Errorf("%s has invalid supplemental group.  Expected: %v got: %v", k, v.expectedSupGroup, scc.SupplementalGroups.Type)
		}
	}
}

func TestDefaultSCCVolumes(t *testing.T) {
	tests := map[string]struct {
		scc             *versioned.SecurityContextConstraints
		expectedVolumes []versioned.FSType
		expectedHostDir bool
	}{
		// this expects the volumes to default to all for an empty volume slice
		// but since the host dir setting is false it should be all - host dir
		"old client - default allow* fields, no volumes slice": {
			scc:             &versioned.SecurityContextConstraints{},
			expectedVolumes: conversionv1.StringSetToFSType(sccutil.GetAllFSTypesExcept(string(versioned.FSTypeHostPath))),
			expectedHostDir: false,
		},
		// this expects the volumes to default to all for an empty volume slice
		"old client - set allowHostDir true fields, no volumes slice": {
			scc: &versioned.SecurityContextConstraints{
				AllowHostDirVolumePlugin: true,
			},
			expectedVolumes: []versioned.FSType{versioned.FSTypeAll},
			expectedHostDir: true,
		},
		"new client - allow* fields set with matching volume slice": {
			scc: &versioned.SecurityContextConstraints{
				Volumes:                  []versioned.FSType{versioned.FSTypeEmptyDir, versioned.FSTypeHostPath},
				AllowHostDirVolumePlugin: true,
			},
			expectedVolumes: []versioned.FSType{versioned.FSTypeEmptyDir, versioned.FSTypeHostPath},
			expectedHostDir: true,
		},
		"new client - allow* fields set with mismatch host dir volume slice": {
			scc: &versioned.SecurityContextConstraints{
				Volumes:                  []versioned.FSType{versioned.FSTypeEmptyDir, versioned.FSTypeHostPath},
				AllowHostDirVolumePlugin: false,
			},
			expectedVolumes: []versioned.FSType{versioned.FSTypeEmptyDir},
			expectedHostDir: false,
		},
		"new client - allow* fields set with mismatch FSTypeAll volume slice": {
			scc: &versioned.SecurityContextConstraints{
				Volumes:                  []versioned.FSType{versioned.FSTypeAll},
				AllowHostDirVolumePlugin: false,
			},
			expectedVolumes: conversionv1.StringSetToFSType(sccutil.GetAllFSTypesExcept(string(versioned.FSTypeHostPath))),
			expectedHostDir: false,
		},
		"new client - allow* fields unset with volume slice": {
			scc: &versioned.SecurityContextConstraints{
				Volumes: []versioned.FSType{versioned.FSTypeEmptyDir, versioned.FSTypeHostPath},
			},
			expectedVolumes: []versioned.FSType{versioned.FSTypeEmptyDir},
			expectedHostDir: false,
		},
		"new client - extra volume params retained": {
			scc: &versioned.SecurityContextConstraints{
				Volumes: []versioned.FSType{versioned.FSTypeEmptyDir, versioned.FSTypeHostPath, versioned.FSTypeGitRepo},
			},
			expectedVolumes: []versioned.FSType{versioned.FSTypeEmptyDir, versioned.FSTypeGitRepo},
			expectedHostDir: false,
		},
		"new client - empty volume slice, host dir true": {
			scc: &versioned.SecurityContextConstraints{
				Volumes:                  []versioned.FSType{},
				AllowHostDirVolumePlugin: true,
			},
			expectedVolumes: []versioned.FSType{versioned.FSTypeHostPath},
			expectedHostDir: true,
		},
		"new client - empty volume slice, host dir false": {
			scc: &versioned.SecurityContextConstraints{
				Volumes:                  []versioned.FSType{},
				AllowHostDirVolumePlugin: false,
			},
			expectedVolumes: []versioned.FSType{versioned.FSTypeNone},
			expectedHostDir: false,
		},
	}
	for k, v := range tests {
		output := roundTrip(t, runtime.Object(v.scc))
		scc := output.(*versioned.SecurityContextConstraints)

		if !reflect.DeepEqual(scc.Volumes, v.expectedVolumes) {
			t.Errorf("%s has invalid volumes.  Expected: %v got: %v", k, v.expectedVolumes, scc.Volumes)
		}

		if scc.AllowHostDirVolumePlugin != v.expectedHostDir {
			t.Errorf("%s has invalid host dir.  Expected: %v got: %v", k, v.expectedHostDir, scc.AllowHostDirVolumePlugin)
		}
	}
}
