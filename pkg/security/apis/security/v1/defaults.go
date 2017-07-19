package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"

	sccutil "github.com/openshift/origin/pkg/security/securitycontextconstraints/util"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	RegisterDefaults(scheme)
	scheme.AddTypeDefaultingFunc(&SecurityContextConstraints{}, func(obj interface{}) { SetDefaults_SCC(obj.(*SecurityContextConstraints)) })
	return nil
}

// Default SCCs for new fields.  FSGroup and SupplementalGroups are
// set to the RunAsAny strategy if they are unset on the scc.
func SetDefaults_SCC(scc *SecurityContextConstraints) {
	if len(scc.FSGroup.Type) == 0 {
		scc.FSGroup.Type = FSGroupStrategyRunAsAny
	}
	if len(scc.SupplementalGroups.Type) == 0 {
		scc.SupplementalGroups.Type = SupplementalGroupsStrategyRunAsAny
	}

	var defaultAllowedVolumes sets.String
	switch {
	case scc.Volumes == nil:
		// assume a nil volume slice is allowing everything for backwards compatibility
		defaultAllowedVolumes = sets.NewString(string(FSTypeAll))

	case len(scc.Volumes) == 0 && scc.AllowHostDirVolumePlugin:
		// an empty volume slice means "allow no volumes", but the boolean fields will always take precedence.
		defaultAllowedVolumes = sets.NewString(string(FSTypeHostPath))

	case len(scc.Volumes) == 0 && !scc.AllowHostDirVolumePlugin:
		// an empty volume slice means "allow no volumes", but cannot be persisted in protobuf.
		// convert this to volumes:["none"]
		defaultAllowedVolumes = sets.NewString(string(FSTypeNone))

	default:
		// defaults the volume slice of the SCC.
		// In order to support old clients the boolean fields will always take precedence.
		defaultAllowedVolumes = fsTypeToStringSet(scc.Volumes)
	}

	if scc.AllowHostDirVolumePlugin {
		// if already allowing all then there is no reason to add
		if !defaultAllowedVolumes.Has(string(FSTypeAll)) {
			defaultAllowedVolumes.Insert(string(FSTypeHostPath))
		}
	} else {
		// we should only default all volumes if the SCC came in with FSTypeAll or we defaulted it
		// otherwise we should only change the volumes slice to ensure that it does not conflict with
		// the AllowHostDirVolumePlugin setting
		shouldDefaultAllVolumes := defaultAllowedVolumes.Has(string(FSTypeAll))

		// remove anything from volumes that conflicts with AllowHostDirVolumePlugin = false
		defaultAllowedVolumes.Delete(string(FSTypeAll))
		defaultAllowedVolumes.Delete(string(FSTypeHostPath))

		if shouldDefaultAllVolumes {
			allVolumes := sccutil.GetAllFSTypesExcept(string(FSTypeHostPath))
			defaultAllowedVolumes.Insert(allVolumes.List()...)
		}
	}

	scc.Volumes = StringSetToFSType(defaultAllowedVolumes)
}

func StringSetToFSType(set sets.String) []FSType {
	if set == nil {
		return nil
	}
	volumes := []FSType{}
	for _, v := range set.List() {
		volumes = append(volumes, FSType(v))
	}
	return volumes
}

func fsTypeToStringSet(volumes []FSType) sets.String {
	if volumes == nil {
		return nil
	}
	set := sets.NewString()
	for _, v := range volumes {
		set.Insert(string(v))
	}
	return set
}
