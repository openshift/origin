package util

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"
	api "k8s.io/kubernetes/pkg/apis/core"

	securityapi "github.com/openshift/origin/pkg/security/apis/security"
)

func GetAllFSTypesExcept(exceptions ...string) sets.String {
	fstypes := GetAllFSTypesAsSet()
	for _, e := range exceptions {
		fstypes.Delete(e)
	}
	return fstypes
}

func GetAllFSTypesAsSet() sets.String {
	fstypes := sets.NewString()
	fstypes.Insert(
		string(securityapi.FSTypeHostPath),
		string(securityapi.FSTypeAzureFile),
		string(securityapi.FSTypeFlocker),
		string(securityapi.FSTypeFlexVolume),
		string(securityapi.FSTypeEmptyDir),
		string(securityapi.FSTypeGCEPersistentDisk),
		string(securityapi.FSTypeAWSElasticBlockStore),
		string(securityapi.FSTypeGitRepo),
		string(securityapi.FSTypeSecret),
		string(securityapi.FSTypeNFS),
		string(securityapi.FSTypeISCSI),
		string(securityapi.FSTypeGlusterfs),
		string(securityapi.FSTypePersistentVolumeClaim),
		string(securityapi.FSTypeRBD),
		string(securityapi.FSTypeCinder),
		string(securityapi.FSTypeCephFS),
		string(securityapi.FSTypeDownwardAPI),
		string(securityapi.FSTypeFC),
		string(securityapi.FSTypeConfigMap),
		string(securityapi.FSTypeVsphereVolume),
		string(securityapi.FSTypeQuobyte),
		string(securityapi.FSTypeAzureDisk),
		string(securityapi.FSTypePhotonPersistentDisk),
		string(securityapi.FSProjected),
		string(securityapi.FSPortworxVolume),
		string(securityapi.FSScaleIO),
		string(securityapi.FSStorageOS),
	)
	return fstypes
}

// getVolumeFSType gets the FSType for a volume.
func GetVolumeFSType(v api.Volume) (securityapi.FSType, error) {
	switch {
	case v.HostPath != nil:
		return securityapi.FSTypeHostPath, nil
	case v.EmptyDir != nil:
		return securityapi.FSTypeEmptyDir, nil
	case v.GCEPersistentDisk != nil:
		return securityapi.FSTypeGCEPersistentDisk, nil
	case v.AWSElasticBlockStore != nil:
		return securityapi.FSTypeAWSElasticBlockStore, nil
	case v.GitRepo != nil:
		return securityapi.FSTypeGitRepo, nil
	case v.Secret != nil:
		return securityapi.FSTypeSecret, nil
	case v.NFS != nil:
		return securityapi.FSTypeNFS, nil
	case v.ISCSI != nil:
		return securityapi.FSTypeISCSI, nil
	case v.Glusterfs != nil:
		return securityapi.FSTypeGlusterfs, nil
	case v.PersistentVolumeClaim != nil:
		return securityapi.FSTypePersistentVolumeClaim, nil
	case v.RBD != nil:
		return securityapi.FSTypeRBD, nil
	case v.FlexVolume != nil:
		return securityapi.FSTypeFlexVolume, nil
	case v.Cinder != nil:
		return securityapi.FSTypeCinder, nil
	case v.CephFS != nil:
		return securityapi.FSTypeCephFS, nil
	case v.Flocker != nil:
		return securityapi.FSTypeFlocker, nil
	case v.DownwardAPI != nil:
		return securityapi.FSTypeDownwardAPI, nil
	case v.FC != nil:
		return securityapi.FSTypeFC, nil
	case v.AzureFile != nil:
		return securityapi.FSTypeAzureFile, nil
	case v.ConfigMap != nil:
		return securityapi.FSTypeConfigMap, nil
	case v.VsphereVolume != nil:
		return securityapi.FSTypeVsphereVolume, nil
	case v.Quobyte != nil:
		return securityapi.FSTypeQuobyte, nil
	case v.AzureDisk != nil:
		return securityapi.FSTypeAzureDisk, nil
	case v.PhotonPersistentDisk != nil:
		return securityapi.FSTypePhotonPersistentDisk, nil
	case v.Projected != nil:
		return securityapi.FSProjected, nil
	case v.PortworxVolume != nil:
		return securityapi.FSPortworxVolume, nil
	case v.ScaleIO != nil:
		return securityapi.FSScaleIO, nil
	case v.StorageOS != nil:
		return securityapi.FSStorageOS, nil
	}

	return "", fmt.Errorf("unknown volume type for volume: %#v", v)
}

// fsTypeToStringSet converts an FSType slice to a string set.
func FSTypeToStringSet(fsTypes []securityapi.FSType) sets.String {
	set := sets.NewString()
	for _, v := range fsTypes {
		set.Insert(string(v))
	}
	return set
}

// SCCAllowsAllVolumes checks for FSTypeAll in the scc's allowed volumes.
func SCCAllowsAllVolumes(scc *securityapi.SecurityContextConstraints) bool {
	return SCCAllowsFSType(scc, securityapi.FSTypeAll)
}

// SCCAllowsFSType is a utility for checking if an SCC allows a particular FSType.
// If all volumes are allowed then this will return true for any FSType passed.
func SCCAllowsFSType(scc *securityapi.SecurityContextConstraints, fsType securityapi.FSType) bool {
	if scc == nil {
		return false
	}

	for _, v := range scc.Volumes {
		if v == fsType || v == securityapi.FSTypeAll {
			return true
		}
	}
	return false
}

// EqualStringSlices compares string slices for equality. Slices are equal when
// their sizes and elements on similar positions are equal.
func EqualStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
