package validation

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	kapi "k8s.io/kubernetes/pkg/apis/core"

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
	wildcardAllowedCapAndRequiredDrop.AllowedCapabilities = []kapi.Capability{securityapi.AllowAllCapabilities}

	emptyFlexDriver := validSCC()
	emptyFlexDriver.Volumes = []securityapi.FSType{securityapi.FSTypeFlexVolume}
	emptyFlexDriver.AllowedFlexVolumes = []securityapi.AllowedFlexVolume{{}}

	nonEmptyFlexVolumes := validSCC()
	nonEmptyFlexVolumes.AllowedFlexVolumes = []securityapi.AllowedFlexVolume{{Driver: "example/driver"}}

	errorCases := map[string]struct {
		scc         *securityapi.SecurityContextConstraints
		errorType   field.ErrorType
		errorDetail string
	}{
		"no user options": {
			scc:         noUserOptions,
			errorType:   field.ErrorTypeInvalid,
			errorDetail: "invalid strategy type.  Valid values are MustRunAs, MustRunAsNonRoot, MustRunAsRange, RunAsAny",
		},
		"no selinux options": {
			scc:         noSELinuxOptions,
			errorType:   field.ErrorTypeInvalid,
			errorDetail: "invalid strategy type.  Valid values are MustRunAs, RunAsAny",
		},
		"no fsgroup options": {
			scc:         noFSGroupOptions,
			errorType:   field.ErrorTypeNotSupported,
			errorDetail: "supported values: \"MustRunAs\", \"RunAsAny\"",
		},
		"no sup group options": {
			scc:         noSupplementalGroupsOptions,
			errorType:   field.ErrorTypeNotSupported,
			errorDetail: "supported values: \"MustRunAs\", \"RunAsAny\"",
		},
		"invalid user strategy type": {
			scc:         invalidUserStratType,
			errorType:   field.ErrorTypeInvalid,
			errorDetail: "invalid strategy type.  Valid values are MustRunAs, MustRunAsNonRoot, MustRunAsRange, RunAsAny",
		},
		"invalid selinux strategy type": {
			scc:         invalidSELinuxStratType,
			errorType:   field.ErrorTypeInvalid,
			errorDetail: "invalid strategy type.  Valid values are MustRunAs, RunAsAny",
		},
		"invalid sup group strategy type": {
			scc:         invalidSupGroupStratType,
			errorType:   field.ErrorTypeNotSupported,
			errorDetail: "supported values: \"MustRunAs\", \"RunAsAny\"",
		},
		"invalid fs group strategy type": {
			scc:         invalidFSGroupStratType,
			errorType:   field.ErrorTypeNotSupported,
			errorDetail: "supported values: \"MustRunAs\", \"RunAsAny\"",
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
		"empty flex volume driver": {
			scc:         emptyFlexDriver,
			errorType:   field.ErrorTypeRequired,
			errorDetail: "must specify a driver",
		},
		"non-empty allowed flex volumes": {
			scc:         nonEmptyFlexVolumes,
			errorType:   field.ErrorTypeInvalid,
			errorDetail: "volumes does not include 'flexVolume' or '*', so no flex volumes are allowed",
		},
	}

	for k, v := range errorCases {
		t.Run(k, func(t *testing.T) {
			if errs := ValidateSecurityContextConstraints(v.scc); len(errs) == 0 || errs[0].Type != v.errorType || errs[0].Detail != v.errorDetail {
				t.Errorf("Expected %q got %q", v.errorType, errs[0].Type)
				t.Errorf("Expected %q got %q", v.errorDetail, errs[0].Detail)
				t.Errorf("got all these %v", errs)
			}
		})
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

	flexvolumeWhenFlexVolumesAllowed := validSCC()
	flexvolumeWhenFlexVolumesAllowed.Volumes = []securityapi.FSType{securityapi.FSTypeFlexVolume}
	flexvolumeWhenFlexVolumesAllowed.AllowedFlexVolumes = []securityapi.AllowedFlexVolume{
		{Driver: "example/driver1"},
	}

	flexvolumeWhenAllVolumesAllowed := validSCC()
	flexvolumeWhenAllVolumesAllowed.Volumes = []securityapi.FSType{securityapi.FSTypeAll}
	flexvolumeWhenAllVolumesAllowed.AllowedFlexVolumes = []securityapi.AllowedFlexVolume{
		{Driver: "example/driver2"},
	}

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
		"allow white-listed flexVolume when flex volumes are allowed": {
			scc: flexvolumeWhenFlexVolumesAllowed,
		},
		"allow white-listed flexVolume when all volumes are allowed": {
			scc: flexvolumeWhenAllVolumesAllowed,
		},
	}

	for k, v := range successCases {
		if errs := ValidateSecurityContextConstraints(v.scc); len(errs) != 0 {
			t.Errorf("Expected success for %q, got %v", k, errs)
		}
	}
}

func validPodSpec() kapi.PodSpec {
	activeDeadlineSeconds := int64(1)
	return kapi.PodSpec{
		Volumes: []kapi.Volume{
			{Name: "vol", VolumeSource: kapi.VolumeSource{EmptyDir: &kapi.EmptyDirVolumeSource{}}},
		},
		Containers: []kapi.Container{
			{
				Name:                     "ctr",
				Image:                    "image",
				ImagePullPolicy:          "IfNotPresent",
				TerminationMessagePolicy: kapi.TerminationMessageReadFile,
			},
		},
		RestartPolicy: kapi.RestartPolicyAlways,
		NodeSelector: map[string]string{
			"key": "value",
		},
		NodeName:              "foobar",
		DNSPolicy:             kapi.DNSClusterFirst,
		ActiveDeadlineSeconds: &activeDeadlineSeconds,
		ServiceAccountName:    "acct",
		SchedulerName:         kapi.DefaultSchedulerName,
	}
}

func invalidPodSpec() kapi.PodSpec {
	return kapi.PodSpec{
		Containers:    []kapi.Container{{TerminationMessagePolicy: kapi.TerminationMessageReadFile}},
		RestartPolicy: kapi.RestartPolicyAlways,
		DNSPolicy:     kapi.DNSClusterFirst,
	}
}

func TestValidatePodSecurityPolicySelfSubjectReview(t *testing.T) {
	okCases := map[string]securityapi.PodSecurityPolicySelfSubjectReview{
		"good case": {
			Spec: securityapi.PodSecurityPolicySelfSubjectReviewSpec{
				Template: kapi.PodTemplateSpec{
					Spec: validPodSpec(),
				},
			},
		},
	}
	for k, v := range okCases {
		errs := ValidatePodSecurityPolicySelfSubjectReview(&v)
		if len(errs) > 0 {
			t.Errorf("%s unexpected error %v", k, errs)
		}
	}

	koCases := map[string]securityapi.PodSecurityPolicySelfSubjectReview{
		"[spec.template.spec.containers[0].name: Required value, spec.template.spec.containers[0].image: Required value, spec.template.spec.containers[0].imagePullPolicy: Required value]": {
			Spec: securityapi.PodSecurityPolicySelfSubjectReviewSpec{
				Template: kapi.PodTemplateSpec{
					Spec: invalidPodSpec(),
				},
			},
		},
	}
	for k, v := range koCases {
		errs := ValidatePodSecurityPolicySelfSubjectReview(&v)
		if len(errs) == 0 {
			t.Errorf("%s expected error %v", k, errs)
			continue
		}
		if errs.ToAggregate().Error() != k {
			t.Errorf("Expected '%s' got '%s'", k, errs.ToAggregate().Error())
		}
	}
}

func TestValidatePodSecurityPolicySubjectReview(t *testing.T) {
	okCases := map[string]securityapi.PodSecurityPolicySubjectReview{
		"good case": {
			Spec: securityapi.PodSecurityPolicySubjectReviewSpec{
				Template: kapi.PodTemplateSpec{
					Spec: validPodSpec(),
				},
			},
		},
	}
	for k, v := range okCases {
		errs := ValidatePodSecurityPolicySubjectReview(&v)
		if len(errs) > 0 {
			t.Errorf("%s unexpected error %v", k, errs)
		}
	}

	koCases := map[string]securityapi.PodSecurityPolicySubjectReview{
		"[spec.template.spec.containers[0].name: Required value, spec.template.spec.containers[0].image: Required value, spec.template.spec.containers[0].imagePullPolicy: Required value]": {
			Spec: securityapi.PodSecurityPolicySubjectReviewSpec{
				Template: kapi.PodTemplateSpec{
					Spec: invalidPodSpec(),
				},
			},
		},
	}
	for k, v := range koCases {
		errs := ValidatePodSecurityPolicySubjectReview(&v)
		if len(errs) == 0 {
			t.Errorf("%s expected error %v", k, errs)
			continue
		}
		if errs.ToAggregate().Error() != k {
			t.Errorf("Expected '%s' got '%s'", k, errs.ToAggregate().Error())
		}
	}
}

func TestValidatePodSecurityPolicyReview(t *testing.T) {
	okCases := map[string]securityapi.PodSecurityPolicyReview{
		"good case 1": {
			Spec: securityapi.PodSecurityPolicyReviewSpec{
				Template: kapi.PodTemplateSpec{
					Spec: validPodSpec(),
				},
			},
		},
		"good case 2": {
			Spec: securityapi.PodSecurityPolicyReviewSpec{
				Template: kapi.PodTemplateSpec{
					Spec: validPodSpec(),
				},
				ServiceAccountNames: []string{"good-service.account"},
			},
		},
	}
	for k, v := range okCases {
		errs := ValidatePodSecurityPolicyReview(&v)
		if len(errs) > 0 {
			t.Errorf("%s unexpected error %v", k, errs)
		}
	}

	koCases := map[string]securityapi.PodSecurityPolicyReview{
		"[spec.template.spec.containers[0].name: Required value, spec.template.spec.containers[0].image: Required value, spec.template.spec.containers[0].imagePullPolicy: Required value]": {
			Spec: securityapi.PodSecurityPolicyReviewSpec{
				Template: kapi.PodTemplateSpec{
					Spec: invalidPodSpec(),
				},
			},
		},
		`spec.serviceAccountNames[0]: Invalid value: "my bad sa": a DNS-1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`: {
			Spec: securityapi.PodSecurityPolicyReviewSpec{
				Template: kapi.PodTemplateSpec{
					Spec: validPodSpec(),
				},
				ServiceAccountNames: []string{"my bad sa"},
			},
		},
		`spec.serviceAccountNames[1]: Invalid value: "my bad sa": a DNS-1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`: {
			Spec: securityapi.PodSecurityPolicyReviewSpec{
				Template: kapi.PodTemplateSpec{
					Spec: validPodSpec(),
				},
				ServiceAccountNames: []string{"good-service.account", "my bad sa"},
			},
		},
		`spec.serviceAccountNames[2]: Invalid value: ""`: {
			Spec: securityapi.PodSecurityPolicyReviewSpec{
				Template: kapi.PodTemplateSpec{
					Spec: validPodSpec(),
				},
				ServiceAccountNames: []string{"good-service.account1", "good-service.account2", ""},
			},
		},
	}
	for k, v := range koCases {
		errs := ValidatePodSecurityPolicyReview(&v)
		if len(errs) == 0 {
			t.Errorf("%s expected error %v", k, errs)
			continue
		}
		if errs.ToAggregate().Error() != k {
			t.Errorf("Expected '%s' got '%s'", k, errs.ToAggregate().Error())
		}
	}

}
