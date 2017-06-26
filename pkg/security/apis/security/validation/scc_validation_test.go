package validation

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	kapi "k8s.io/kubernetes/pkg/api"

	securityapi "github.com/openshift/origin/pkg/security/apis/security"
)

func TestValidateSecurityContextConstraints(t *testing.T) {
	var invalidUID int64 = -1
	var invalidPriority int32 = -1
	var validPriority int32 = 1

	validSCC := func() *securityapi.SecurityContextConstraints {
		return &securityapi.SecurityContextConstraints{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			SELinuxContext: securityapi.SELinuxContextStrategyOptions{
				Type: securityapi.SELinuxStrategyRunAsAny,
			},
			RunAsUser: securityapi.RunAsUserStrategyOptions{
				Type: securityapi.RunAsUserStrategyRunAsAny,
			},
			FSGroup: securityapi.FSGroupStrategyOptions{
				Type: securityapi.FSGroupStrategyRunAsAny,
			},
			SupplementalGroups: securityapi.SupplementalGroupsStrategyOptions{
				Type: securityapi.SupplementalGroupsStrategyRunAsAny,
			},
			Priority: &validPriority,
		}
	}

	noUserOptions := validSCC()
	noUserOptions.RunAsUser.Type = ""

	noSELinuxOptions := validSCC()
	noSELinuxOptions.SELinuxContext.Type = ""

	invalidUserStratType := validSCC()
	invalidUserStratType.RunAsUser.Type = "invalid"

	invalidSELinuxStratType := validSCC()
	invalidSELinuxStratType.SELinuxContext.Type = "invalid"

	invalidUIDSCC := validSCC()
	invalidUIDSCC.RunAsUser.Type = securityapi.RunAsUserStrategyMustRunAs
	invalidUIDSCC.RunAsUser.UID = &invalidUID

	missingObjectMetaName := validSCC()
	missingObjectMetaName.ObjectMeta.Name = ""

	noFSGroupOptions := validSCC()
	noFSGroupOptions.FSGroup.Type = ""

	invalidFSGroupStratType := validSCC()
	invalidFSGroupStratType.FSGroup.Type = "invalid"

	noSupplementalGroupsOptions := validSCC()
	noSupplementalGroupsOptions.SupplementalGroups.Type = ""

	invalidSupGroupStratType := validSCC()
	invalidSupGroupStratType.SupplementalGroups.Type = "invalid"

	invalidRangeMinGreaterThanMax := validSCC()
	invalidRangeMinGreaterThanMax.FSGroup.Ranges = []securityapi.IDRange{
		{Min: 2, Max: 1},
	}

	invalidRangeNegativeMin := validSCC()
	invalidRangeNegativeMin.FSGroup.Ranges = []securityapi.IDRange{
		{Min: -1, Max: 10},
	}

	invalidRangeNegativeMax := validSCC()
	invalidRangeNegativeMax.FSGroup.Ranges = []securityapi.IDRange{
		{Min: 1, Max: -10},
	}

	negativePriority := validSCC()
	negativePriority.Priority = &invalidPriority

	requiredCapAddAndDrop := validSCC()
	requiredCapAddAndDrop.DefaultAddCapabilities = []kapi.Capability{"foo"}
	requiredCapAddAndDrop.RequiredDropCapabilities = []kapi.Capability{"foo"}

	allowedCapListedInRequiredDrop := validSCC()
	allowedCapListedInRequiredDrop.RequiredDropCapabilities = []kapi.Capability{"foo"}
	allowedCapListedInRequiredDrop.AllowedCapabilities = []kapi.Capability{"foo"}

	wildcardAllowedCapAndRequiredDrop := validSCC()
	wildcardAllowedCapAndRequiredDrop.RequiredDropCapabilities = []kapi.Capability{"foo"}
	wildcardAllowedCapAndRequiredDrop.AllowedCapabilities = []kapi.Capability{kapi.CapabilityAll}

	errorCases := map[string]struct {
		scc         *securityapi.SecurityContextConstraints
		errorType   field.ErrorType
		errorDetail string
	}{
		"no user options": {
			scc:         noUserOptions,
			errorType:   field.ErrorTypeInvalid,
			errorDetail: "invalid strategy type.  Valid values are MustRunAs, MustRunAsNonRoot, RunAsAny",
		},
		"no selinux options": {
			scc:         noSELinuxOptions,
			errorType:   field.ErrorTypeInvalid,
			errorDetail: "invalid strategy type.  Valid values are MustRunAs, RunAsAny",
		},
		"no fsgroup options": {
			scc:         noFSGroupOptions,
			errorType:   field.ErrorTypeNotSupported,
			errorDetail: "supported values: MustRunAs, RunAsAny",
		},
		"no sup group options": {
			scc:         noSupplementalGroupsOptions,
			errorType:   field.ErrorTypeNotSupported,
			errorDetail: "supported values: MustRunAs, RunAsAny",
		},
		"invalid user strategy type": {
			scc:         invalidUserStratType,
			errorType:   field.ErrorTypeInvalid,
			errorDetail: "invalid strategy type.  Valid values are MustRunAs, MustRunAsNonRoot, RunAsAny",
		},
		"invalid selinux strategy type": {
			scc:         invalidSELinuxStratType,
			errorType:   field.ErrorTypeInvalid,
			errorDetail: "invalid strategy type.  Valid values are MustRunAs, RunAsAny",
		},
		"invalid sup group strategy type": {
			scc:         invalidSupGroupStratType,
			errorType:   field.ErrorTypeNotSupported,
			errorDetail: "supported values: MustRunAs, RunAsAny",
		},
		"invalid fs group strategy type": {
			scc:         invalidFSGroupStratType,
			errorType:   field.ErrorTypeNotSupported,
			errorDetail: "supported values: MustRunAs, RunAsAny",
		},
		"invalid uid": {
			scc:         invalidUIDSCC,
			errorType:   field.ErrorTypeInvalid,
			errorDetail: "uid cannot be negative",
		},
		"missing object meta name": {
			scc:         missingObjectMetaName,
			errorType:   field.ErrorTypeRequired,
			errorDetail: "name or generateName is required",
		},
		"invalid range min greater than max": {
			scc:         invalidRangeMinGreaterThanMax,
			errorType:   field.ErrorTypeInvalid,
			errorDetail: "min cannot be greater than max",
		},
		"invalid range negative min": {
			scc:         invalidRangeNegativeMin,
			errorType:   field.ErrorTypeInvalid,
			errorDetail: "min cannot be negative",
		},
		"invalid range negative max": {
			scc:         invalidRangeNegativeMax,
			errorType:   field.ErrorTypeInvalid,
			errorDetail: "max cannot be negative",
		},
		"negative priority": {
			scc:         negativePriority,
			errorType:   field.ErrorTypeInvalid,
			errorDetail: "priority cannot be negative",
		},
		"invalid required caps": {
			scc:         requiredCapAddAndDrop,
			errorType:   field.ErrorTypeInvalid,
			errorDetail: "capability is listed in defaultAddCapabilities and requiredDropCapabilities",
		},
		"allowed cap listed in required drops": {
			scc:         allowedCapListedInRequiredDrop,
			errorType:   field.ErrorTypeInvalid,
			errorDetail: "capability is listed in allowedCapabilities and requiredDropCapabilities",
		},
		"all caps allowed by a wildcard and required drops is not empty": {
			scc:         wildcardAllowedCapAndRequiredDrop,
			errorType:   field.ErrorTypeInvalid,
			errorDetail: "required capabilities must be empty when all capabilities are allowed by a wildcard",
		},
	}

	for k, v := range errorCases {
		if errs := ValidateSecurityContextConstraints(v.scc); len(errs) == 0 || errs[0].Type != v.errorType || errs[0].Detail != v.errorDetail {
			t.Errorf("Expected error type %s with detail %s for %s, got %v", v.errorType, v.errorDetail, k, errs)
		}
	}

	var validUID int64 = 1

	mustRunAs := validSCC()
	mustRunAs.FSGroup.Type = securityapi.FSGroupStrategyMustRunAs
	mustRunAs.SupplementalGroups.Type = securityapi.SupplementalGroupsStrategyMustRunAs
	mustRunAs.RunAsUser.Type = securityapi.RunAsUserStrategyMustRunAs
	mustRunAs.RunAsUser.UID = &validUID
	mustRunAs.SELinuxContext.Type = securityapi.SELinuxStrategyMustRunAs

	runAsNonRoot := validSCC()
	runAsNonRoot.RunAsUser.Type = securityapi.RunAsUserStrategyMustRunAsNonRoot

	caseInsensitiveAddDrop := validSCC()
	caseInsensitiveAddDrop.DefaultAddCapabilities = []kapi.Capability{"foo"}
	caseInsensitiveAddDrop.RequiredDropCapabilities = []kapi.Capability{"FOO"}

	caseInsensitiveAllowedDrop := validSCC()
	caseInsensitiveAllowedDrop.RequiredDropCapabilities = []kapi.Capability{"FOO"}
	caseInsensitiveAllowedDrop.AllowedCapabilities = []kapi.Capability{"foo"}

	successCases := map[string]struct {
		scc *securityapi.SecurityContextConstraints
	}{
		"must run as": {
			scc: mustRunAs,
		},
		"run as any": {
			scc: validSCC(),
		},
		"run as non-root (user only)": {
			scc: runAsNonRoot,
		},
		"comparison for add -> drop is case sensitive": {
			scc: caseInsensitiveAddDrop,
		},
		"comparison for allowed -> drop is case sensitive": {
			scc: caseInsensitiveAllowedDrop,
		},
	}

	for k, v := range successCases {
		if errs := ValidateSecurityContextConstraints(v.scc); len(errs) != 0 {
			t.Errorf("Expected success for %s, got %v", k, errs)
		}
	}
}
