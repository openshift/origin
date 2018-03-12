package scc

import (
	"fmt"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	policy "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	internalextensions "k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/security/podsecuritypolicy/seccomp"

	securityv1 "github.com/openshift/api/security/v1"
)

func TestExtractPrivileged(t *testing.T) {
	expected := true

	scc := &securityv1.SecurityContextConstraints{
		AllowPrivilegedContainer: expected,
	}

	if actual := extractPrivileged(scc); actual != expected {
		t.Errorf("extractPrivileged() expected to return %t but got %t", expected, actual)
	}
}

func TestExtractDefaultAddCapabilities(t *testing.T) {
	expected := []corev1.Capability{"SYSLOG", "SYS_CHROOT"}

	scc := &securityv1.SecurityContextConstraints{
		DefaultAddCapabilities: []corev1.Capability{"SYSLOG", "SYS_CHROOT"},
	}

	if actual := extractDefaultAddCapabilities(scc); !reflect.DeepEqual(actual, expected) {
		t.Errorf("extractDefaultAddCapabilities() expected to return %#v but got %#v", expected, actual)
	}
}

func TestExtractRequiredDropCapabilities(t *testing.T) {
	expected := []corev1.Capability{"KILL", "MKNOD"}

	scc := &securityv1.SecurityContextConstraints{
		RequiredDropCapabilities: []corev1.Capability{"KILL", "MKNOD"},
	}

	if actual := extractRequiredDropCapabilities(scc); !reflect.DeepEqual(actual, expected) {
		t.Errorf("extractRequiredDropCapabilities() expected to return %#v but got %#v", expected, actual)
	}
}
func TestExtractAllowedCapabilities(t *testing.T) {
	expected := []corev1.Capability{"SYS_CHROOT"}

	scc := &securityv1.SecurityContextConstraints{
		AllowedCapabilities: []corev1.Capability{"SYS_CHROOT"},
	}

	if actual := extractAllowedCapabilities(scc); !reflect.DeepEqual(actual, expected) {
		t.Errorf("extractAllowedCapabilities() expected to return %#v but got %#v", expected, actual)
	}
}

func TestExtractAllowedCapabilitiesWithWildcard(t *testing.T) {
	expected := []corev1.Capability{"*"}

	scc := &securityv1.SecurityContextConstraints{
		AllowedCapabilities: []corev1.Capability{securityv1.AllowAllCapabilities},
	}

	if actual := extractAllowedCapabilities(scc); !reflect.DeepEqual(actual, expected) {
		t.Errorf("extractAllowedCapabilities() expected to return %#v but got %#v", expected, actual)
	}
}
func TestExtractVolumes(t *testing.T) {
	expected := []policy.FSType{policy.Secret, policy.EmptyDir}

	scc := &securityv1.SecurityContextConstraints{
		Volumes: []securityv1.FSType{securityv1.FSTypeSecret, securityv1.FSTypeEmptyDir},
	}

	if actual := extractVolumes(scc); !reflect.DeepEqual(actual, expected) {
		t.Errorf("extractVolumes() expected to return %#v but got %#v", expected, actual)
	}
}

func TestExtractVolumesWithWildcard(t *testing.T) {
	expected := []policy.FSType{policy.All}

	scc := &securityv1.SecurityContextConstraints{
		Volumes: []securityv1.FSType{securityv1.FSTypeAll},
	}

	if actual := extractVolumes(scc); !reflect.DeepEqual(actual, expected) {
		t.Errorf("extractVolumes() expected to return %#v but got %#v", expected, actual)
	}
}

func TestExtractVolumesWithNoneType(t *testing.T) {
	scc := &securityv1.SecurityContextConstraints{
		Volumes: []securityv1.FSType{securityv1.FSTypeNone},
	}

	if volumes := extractVolumes(scc); len(volumes) > 0 {
		t.Errorf("extractVolumes() expected to return an empty list but got %#v", volumes)
	}
}

func TestExtractHostNetwork(t *testing.T) {
	expected := true

	scc := &securityv1.SecurityContextConstraints{
		AllowHostNetwork: expected,
	}

	if actual := extractHostNetwork(scc); actual != expected {
		t.Errorf("extractHostNetwork() expected to return %t but got %t", expected, actual)
	}
}

func TestExtractHostPorts(t *testing.T) {
	expected := []policy.HostPortRange{
		{Min: 0, Max: 65535},
	}

	scc := &securityv1.SecurityContextConstraints{
		AllowHostPorts: true,
	}

	if actual := extractHostPorts(scc); !reflect.DeepEqual(actual, expected) {
		t.Errorf("extractHostPorts() expected to return %#v but got %#v", expected, actual)
	}
}

func TestExtractHostPID(t *testing.T) {
	expected := true

	scc := &securityv1.SecurityContextConstraints{
		AllowHostPID: expected,
	}

	if actual := extractHostPID(scc); actual != expected {
		t.Errorf("extractHostPID() expected to return %t but got %t", expected, actual)
	}
}

func TestExtractHostIPC(t *testing.T) {
	expected := true

	scc := &securityv1.SecurityContextConstraints{
		AllowHostIPC: expected,
	}

	if actual := extractHostIPC(scc); actual != expected {
		t.Errorf("extractHostIPC() expected to return %t but got %t", expected, actual)
	}
}

func TestExtractSELinuxWithRunAsAny(t *testing.T) {
	expected := policy.SELinuxStrategyOptions{
		Rule: policy.SELinuxStrategyRunAsAny,
	}

	scc := &securityv1.SecurityContextConstraints{
		SELinuxContext: securityv1.SELinuxContextStrategyOptions{
			Type: securityv1.SELinuxStrategyRunAsAny,
		},
	}

	actual, err := extractSELinux(scc)
	if err != nil {
		t.Fatalf("extractSELinux() failed with unexpected error: %v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("extractSELinux() expected to return %#v but got %#v", expected, actual)
	}
}

func TestExtractSELinuxWithMustRunAs(t *testing.T) {
	expected := policy.SELinuxStrategyOptions{
		Rule: policy.SELinuxStrategyMustRunAs,
		SELinuxOptions: &corev1.SELinuxOptions{
			User:  "user",
			Role:  "role",
			Type:  "type",
			Level: "level",
		},
	}

	scc := &securityv1.SecurityContextConstraints{
		SELinuxContext: securityv1.SELinuxContextStrategyOptions{
			Type: securityv1.SELinuxStrategyMustRunAs,
			SELinuxOptions: &corev1.SELinuxOptions{
				User:  "user",
				Role:  "role",
				Type:  "type",
				Level: "level",
			},
		},
	}

	actual, err := extractSELinux(scc)
	if err != nil {
		t.Fatalf("extractSELinux() failed with unexpected error: %v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("extractSELinux() expected to return %#v but got %#v", expected, actual)
	}
}

func TestExtractRunAsUserWithRunAsAny(t *testing.T) {
	expected := policy.RunAsUserStrategyOptions{
		Rule: policy.RunAsUserStrategyRunAsAny,
	}

	scc := &securityv1.SecurityContextConstraints{
		RunAsUser: securityv1.RunAsUserStrategyOptions{
			Type: securityv1.RunAsUserStrategyRunAsAny,
		},
	}

	actual, err := extractRunAsUser(scc)
	if err != nil {
		t.Fatalf("extractRunAsUser() failed with unexpected error: %v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("extractRunAsUser() expected to return %#v but got %#v", expected, actual)
	}
}

func TestExtractRunAsUserWithMustRunAsNonRoot(t *testing.T) {
	expected := policy.RunAsUserStrategyOptions{
		Rule: policy.RunAsUserStrategyMustRunAsNonRoot,
	}

	scc := &securityv1.SecurityContextConstraints{
		RunAsUser: securityv1.RunAsUserStrategyOptions{
			Type: securityv1.RunAsUserStrategyMustRunAsNonRoot,
		},
	}

	actual, err := extractRunAsUser(scc)
	if err != nil {
		t.Fatalf("extractRunAsUser() failed with unexpected error: %v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("extractRunAsUser() expected to return %#v but got %#v", expected, actual)
	}
}

func TestExtractRunAsUserWithMustRunAs(t *testing.T) {
	var expectedUID int64 = 1000
	expected := policy.RunAsUserStrategyOptions{
		Rule: policy.RunAsUserStrategyMustRunAs,
		Ranges: []policy.IDRange{
			{
				Min: expectedUID,
				Max: expectedUID,
			},
		},
	}

	scc := &securityv1.SecurityContextConstraints{
		RunAsUser: securityv1.RunAsUserStrategyOptions{
			Type: securityv1.RunAsUserStrategyMustRunAs,
			UID:  &expectedUID,
		},
	}

	actual, err := extractRunAsUser(scc)
	if err != nil {
		t.Fatalf("extractRunAsUser() failed with unexpected error: %v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("extractRunAsUser() expected to return %#v but got %#v", expected, actual)
	}
}

func TestExtractRunAsUserWithMustRunAsRange(t *testing.T) {
	var expectedMinUID int64 = 1000
	var expectedMaxUID int64 = 2000
	expected := policy.RunAsUserStrategyOptions{
		Rule: policy.RunAsUserStrategyMustRunAs,
		Ranges: []policy.IDRange{
			{
				Min: expectedMinUID,
				Max: expectedMaxUID,
			},
		},
	}

	scc := &securityv1.SecurityContextConstraints{
		RunAsUser: securityv1.RunAsUserStrategyOptions{
			Type:        securityv1.RunAsUserStrategyMustRunAsRange,
			UIDRangeMin: &expectedMinUID,
			UIDRangeMax: &expectedMaxUID,
		},
	}

	actual, err := extractRunAsUser(scc)
	if err != nil {
		t.Fatalf("extractRunAsUser() failed with unexpected error: %v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("extractRunAsUser() expected to return %#v but got %#v", expected, actual)
	}
}

func TestExtractSupplementalGroupsWithRunAsAny(t *testing.T) {
	expected := policy.SupplementalGroupsStrategyOptions{
		Rule: policy.SupplementalGroupsStrategyRunAsAny,
	}

	scc := &securityv1.SecurityContextConstraints{
		SupplementalGroups: securityv1.SupplementalGroupsStrategyOptions{
			Type: securityv1.SupplementalGroupsStrategyRunAsAny,
		},
	}

	actual, err := extractSupplementalGroups(scc)
	if err != nil {
		t.Fatalf("extractSupplementalGroups() failed with unexpected error: %v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("extractSupplementalGroups() expected to return %#v but got %#v", expected, actual)
	}
}

func TestExtractSupplementalGroupsWithMustRunAs(t *testing.T) {
	expected := policy.SupplementalGroupsStrategyOptions{
		Rule: policy.SupplementalGroupsStrategyMustRunAs,
		Ranges: []policy.IDRange{
			{
				Min: 500,
				Max: 1000,
			},
			{
				Min: 2000,
				Max: 10000,
			},
		},
	}

	scc := &securityv1.SecurityContextConstraints{
		SupplementalGroups: securityv1.SupplementalGroupsStrategyOptions{
			Type: securityv1.SupplementalGroupsStrategyMustRunAs,
			Ranges: []securityv1.IDRange{
				{
					Min: 500,
					Max: 1000,
				},
				{
					Min: 2000,
					Max: 10000,
				},
			},
		},
	}

	actual, err := extractSupplementalGroups(scc)
	if err != nil {
		t.Fatalf("extractSupplementalGroups() failed with unexpected error: %v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("extractSupplementalGroups() expected to return %#v but got %#v", expected, actual)
	}
}

func TestExtractFSGroupWithRunAsAny(t *testing.T) {
	expected := policy.FSGroupStrategyOptions{
		Rule: policy.FSGroupStrategyRunAsAny,
	}

	scc := &securityv1.SecurityContextConstraints{
		FSGroup: securityv1.FSGroupStrategyOptions{
			Type: securityv1.FSGroupStrategyRunAsAny,
		},
	}

	actual, err := extractFSGroup(scc)
	if err != nil {
		t.Fatalf("extractFSGroup() failed with unexpected error: %v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("extractFSGroup() expected to return %#v but got %#v", expected, actual)
	}
}

func TestExtractFSGroupsWithMustRunAs(t *testing.T) {
	expected := policy.FSGroupStrategyOptions{
		Rule: policy.FSGroupStrategyMustRunAs,
		Ranges: []policy.IDRange{
			{
				Min: 500,
				Max: 1000,
			},
			{
				Min: 2000,
				Max: 10000,
			},
		},
	}

	scc := &securityv1.SecurityContextConstraints{
		FSGroup: securityv1.FSGroupStrategyOptions{
			Type: securityv1.FSGroupStrategyMustRunAs,
			Ranges: []securityv1.IDRange{
				{
					Min: 500,
					Max: 1000,
				},
				{
					Min: 2000,
					Max: 10000,
				},
			},
		},
	}

	actual, err := extractFSGroup(scc)
	if err != nil {
		t.Fatalf("extractFSGroup() failed with unexpected error: %v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("extractFSGroup() expected to return %#v but got %#v", expected, actual)
	}
}

func TestExtractReadOnlyRootFilesystem(t *testing.T) {
	expected := true

	scc := &securityv1.SecurityContextConstraints{
		ReadOnlyRootFilesystem: expected,
	}

	if actual := extractReadOnlyRootFilesystem(scc); actual != expected {
		t.Errorf("extractReadOnlyRootFilesystem() expected to return %t but got %t", expected, actual)
	}
}

func TestExtractAllowedFlexVolumes(t *testing.T) {
	expected := []policy.AllowedFlexVolume{
		{
			Driver: "example/foo",
		},
		{
			Driver: "example/bar",
		},
	}

	scc := &securityv1.SecurityContextConstraints{
		AllowedFlexVolumes: []securityv1.AllowedFlexVolume{
			{
				Driver: "example/foo",
			},
			{
				Driver: "example/bar",
			},
		},
	}

	if actual := extractAllowedFlexVolumes(scc); !reflect.DeepEqual(actual, expected) {
		t.Errorf("extractAllowedFlexVolumes() expected to return %#v but got %#v", expected, actual)
	}
}

func TestExtractSeccompProfilesWithEmptyProfiles(t *testing.T) {
	scc := &securityv1.SecurityContextConstraints{
		SeccompProfiles: nil,
	}

	annotations := make(map[string]string)
	extractSeccompProfiles(scc, annotations)

	defaultProfile, hasDefaultProfile := annotations[seccomp.DefaultProfileAnnotationKey]
	if hasDefaultProfile {
		t.Errorf("extractSeccompProfiles() expected that annotation %q won't be set but it has value %q", seccomp.DefaultProfileAnnotationKey, defaultProfile)
	}

	allowedProfiles, hasAllowedProfiles := annotations[seccomp.AllowedProfilesAnnotationKey]
	if hasAllowedProfiles {
		t.Errorf("extractSeccompProfiles() expected that annotation %q won't be set but it has value %q", seccomp.AllowedProfilesAnnotationKey, allowedProfiles)
	}
}

func TestExtractSeccompProfilesWithWildcard(t *testing.T) {
	scc := &securityv1.SecurityContextConstraints{
		SeccompProfiles: []string{"*"},
	}

	annotations := make(map[string]string)
	extractSeccompProfiles(scc, annotations)

	defaultProfile, hasDefaultProfile := annotations[seccomp.DefaultProfileAnnotationKey]
	if hasDefaultProfile {
		t.Errorf("extractSeccompProfiles() expected that annotation %q won't be set but it has value %q", seccomp.DefaultProfileAnnotationKey, defaultProfile)
	}

	allowedProfiles, hasAllowedProfiles := annotations[seccomp.AllowedProfilesAnnotationKey]
	if !hasAllowedProfiles {
		t.Errorf("extractSeccompProfiles() expected to set annotation %q but it is not set", seccomp.AllowedProfilesAnnotationKey)
	} else if allowedProfiles != seccomp.AllowAny {
		t.Errorf("extractSeccompProfiles() expected annotation %q to be %q but it has value %q", seccomp.AllowedProfilesAnnotationKey, seccomp.AllowAny, allowedProfiles)
	}
}

func TestExtractSeccompProfilesWithMultipleProfiles(t *testing.T) {
	expectedAllowedProfiles := "docker/default,localhost/super-secure"
	expectedDefaultProfile := "docker/default"
	scc := &securityv1.SecurityContextConstraints{
		SeccompProfiles: []string{"docker/default", "localhost/super-secure"},
	}

	annotations := make(map[string]string)
	extractSeccompProfiles(scc, annotations)

	defaultProfile, hasDefaultProfile := annotations[seccomp.DefaultProfileAnnotationKey]
	if !hasDefaultProfile {
		t.Errorf("extractSeccompProfiles() expected to set annotation %q but it is not set", seccomp.DefaultProfileAnnotationKey)
	} else if defaultProfile != expectedDefaultProfile {
		t.Errorf("extractSeccompProfiles() expected annotation %q to be %q but it has value %q", seccomp.DefaultProfileAnnotationKey, expectedDefaultProfile, defaultProfile)
	}

	allowedProfiles, hasAllowedProfiles := annotations[seccomp.AllowedProfilesAnnotationKey]
	if !hasAllowedProfiles {
		t.Errorf("extractSeccompProfiles() expected to set annotation %q but it is not set", seccomp.AllowedProfilesAnnotationKey)
	} else if allowedProfiles != expectedAllowedProfiles {
		t.Errorf("extractSeccompProfiles() expected annotation %q to be %q but it has value %q", seccomp.AllowedProfilesAnnotationKey, expectedAllowedProfiles, allowedProfiles)
	}
}

func TestExtractSeccompProfilesWithWildcardAndNamedProfiles(t *testing.T) {
	expectedAllowedProfiles := seccomp.AllowAny
	expectedDefaultProfile := "docker/default"
	scc := &securityv1.SecurityContextConstraints{
		SeccompProfiles: []string{"*", "docker/default"},
	}

	annotations := make(map[string]string)
	extractSeccompProfiles(scc, annotations)

	defaultProfile, hasDefaultProfile := annotations[seccomp.DefaultProfileAnnotationKey]
	if !hasDefaultProfile {
		t.Errorf("extractSeccompProfiles() expected to set annotation %q but it is not set", seccomp.DefaultProfileAnnotationKey)
	} else if defaultProfile != expectedDefaultProfile {
		t.Errorf("extractSeccompProfiles() expected annotation %q to be %q but it has value %q", seccomp.DefaultProfileAnnotationKey, expectedDefaultProfile, defaultProfile)
	}

	allowedProfiles, hasAllowedProfiles := annotations[seccomp.AllowedProfilesAnnotationKey]
	if !hasAllowedProfiles {
		t.Errorf("extractSeccompProfiles() expected to set annotation %q but it is not set", seccomp.AllowedProfilesAnnotationKey)
	} else if allowedProfiles != expectedAllowedProfiles {
		t.Errorf("extractSeccompProfiles() expected annotation %q to be %q but it has value %q", seccomp.AllowedProfilesAnnotationKey, expectedAllowedProfiles, allowedProfiles)
	}
}

func TestExtractSysctls(t *testing.T) {
	expected := "net.ipv4.route.*,kernel.msg*"
	sysctlsAnnotation := internalextensions.SysctlsPodSecurityPolicyAnnotationKey

	scc := &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				sysctlsAnnotation: expected,
			},
		},
	}

	annotations := make(map[string]string)
	extractSysctls(scc, annotations)

	actual, hasAnnotation := annotations[sysctlsAnnotation]
	if !hasAnnotation {
		t.Errorf("extractSysctls() expected to set annotation %q but it is not set", sysctlsAnnotation)
	} else if actual != expected {
		t.Errorf("extractSysctls() expected annotation %q to be %q but it has value %q", sysctlsAnnotation, expected, actual)
	}
}

func TestExtractPriority(t *testing.T) {
	var priority int32 = 10
	expected := fmt.Sprintf("%d", priority)
	scc := &securityapi.SecurityContextConstraints{
		Priority: &priority,
	}

	annotations := make(map[string]string)
	extractPriority(scc, annotations)

	value, hasAnnotation := annotations[pspPriorityAnnotationKey]
	if !hasAnnotation {
		t.Fatalf("expected to have annotation %q but it is not set", pspPriorityAnnotationKey)
	}

	if value != expected {
		t.Errorf("expected annotation %q to have value \"%v\" but it has value \"%v\"", pspPriorityAnnotationKey, expected, value)
	}
}
