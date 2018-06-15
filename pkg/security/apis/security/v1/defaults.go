package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openshift/api/security/v1"
	sccutil "github.com/openshift/origin/pkg/security/securitycontextconstraints/util"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	RegisterDefaults(scheme)
	scheme.AddTypeDefaultingFunc(&v1.SecurityContextConstraints{}, func(obj interface{}) { SetDefaults_SCC(obj.(*v1.SecurityContextConstraints)) })
	return nil
}

// Default SCCs for new fields.  FSGroup and SupplementalGroups are
// set to the RunAsAny strategy if they are unset on the scc.
func SetDefaults_SCC(scc *v1.SecurityContextConstraints) {
	if len(scc.FSGroup.Type) == 0 {
		scc.FSGroup.Type = v1.FSGroupStrategyRunAsAny
	}
	if len(scc.SupplementalGroups.Type) == 0 {
		scc.SupplementalGroups.Type = v1.SupplementalGroupsStrategyRunAsAny
	}

	if scc.Users == nil {
		scc.Users = []string{}
	}
	if scc.Groups == nil {
		scc.Groups = []string{}
	}

	var defaultAllowedVolumes sets.String
	switch {
	case scc.Volumes == nil:
		// assume a nil volume slice is allowing everything for backwards compatibility
		defaultAllowedVolumes = sets.NewString(string(v1.FSTypeAll))

	case len(scc.Volumes) == 0 && scc.AllowHostDirVolumePlugin:
		// an empty volume slice means "allow no volumes", but the boolean fields will always take precedence.
		defaultAllowedVolumes = sets.NewString(string(v1.FSTypeHostPath))

	case len(scc.Volumes) == 0 && !scc.AllowHostDirVolumePlugin:
		// an empty volume slice means "allow no volumes", but cannot be persisted in protobuf.
		// convert this to volumes:["none"]
		defaultAllowedVolumes = sets.NewString(string(v1.FSTypeNone))

	default:
		// defaults the volume slice of the SCC.
		// In order to support old clients the boolean fields will always take precedence.
		defaultAllowedVolumes = fsTypeToStringSet(scc.Volumes)
	}

	if scc.AllowHostDirVolumePlugin {
		// if already allowing all then there is no reason to add
		if !defaultAllowedVolumes.Has(string(v1.FSTypeAll)) {
			defaultAllowedVolumes.Insert(string(v1.FSTypeHostPath))
		}
	} else {
		// we should only default all volumes if the SCC came in with FSTypeAll or we defaulted it
		// otherwise we should only change the volumes slice to ensure that it does not conflict with
		// the AllowHostDirVolumePlugin setting
		shouldDefaultAllVolumes := defaultAllowedVolumes.Has(string(v1.FSTypeAll))

		// remove anything from volumes that conflicts with AllowHostDirVolumePlugin = false
		defaultAllowedVolumes.Delete(string(v1.FSTypeAll))
		defaultAllowedVolumes.Delete(string(v1.FSTypeHostPath))

		if shouldDefaultAllVolumes {
			allVolumes := sccutil.GetAllFSTypesExcept(string(v1.FSTypeHostPath))
			defaultAllowedVolumes.Insert(allVolumes.List()...)
		}
	}

	scc.Volumes = StringSetToFSType(defaultAllowedVolumes)
}

func StringSetToFSType(set sets.String) []v1.FSType {
	if set == nil {
		return nil
	}
	volumes := []v1.FSType{}
	for _, v := range set.List() {
		volumes = append(volumes, v1.FSType(v))
	}
	return volumes
}

func fsTypeToStringSet(volumes []v1.FSType) sets.String {
	if volumes == nil {
		return nil
	}
	set := sets.NewString()
	for _, v := range volumes {
		set.Insert(string(v))
	}
	return set
}
