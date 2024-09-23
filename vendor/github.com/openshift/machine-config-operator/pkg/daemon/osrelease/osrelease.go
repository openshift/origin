package osrelease

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ashcrow/osrelease"
)

// OS Release Paths
const (
	EtcOSReleasePath string = "/etc/os-release"
	LibOSReleasePath string = "/usr/lib/os-release"
)

// OS IDs
const (
	coreos string = "coreos"
	fedora string = "fedora"
	rhel   string = "rhel"
	rhcos  string = "rhcos"
	scos   string = "scos"
)

// OperatingSystem is a wrapper around a subset of the os-release fields
// and also tracks whether ostree is in use.
type OperatingSystem struct {
	// id is the ID field from the os-release
	id string
	// variantID is the VARIANT_ID field from the os-release
	variantID string
	// version is the VERSION, RHEL_VERSION, or VERSION_ID field from the os-release
	version string
	// osrelease is the underlying struct from github.com/ashcrow/osrelease
	osrelease osrelease.OSRelease
}

func newOperatingSystem(etcPath, libPath string) (OperatingSystem, error) {
	ret := OperatingSystem{}

	or, err := osrelease.NewWithOverrides(etcPath, libPath)
	if err != nil {
		return ret, err
	}

	ret.id = or.ID
	ret.variantID = or.VARIANT_ID
	ret.version = getOSVersion(or)
	ret.osrelease = or

	return ret, nil
}

// Returns the underlying OSRelease struct if additional parameters are needed.
func (os OperatingSystem) OSRelease() osrelease.OSRelease {
	return os.osrelease
}

// IsLikeRHEL is true if the OS is RHEL-like.
func (os OperatingSystem) IsLikeRHEL() bool {
	if os.osrelease.ID == "rhel" {
		return true
	}
	for _, v := range strings.Split(os.osrelease.ID_LIKE, " ") {
		if v == "rhel" {
			return true
		}
	}
	return false
}

// BaseVersion gets the VERSION_ID field, but prefers RHEL_VERSION if it exists
// as it does for RHEL CoreOS.
func (os OperatingSystem) BaseVersion() string {
	return os.version
}

// BaseVersionMajor returns the first number in a `.` separated BaseVersion.
// For example with VERSION_ID=9.2, this will return 9.
func (os OperatingSystem) BaseVersionMajor() string {
	return strings.Split(os.BaseVersion(), ".")[0]
}

// IsEL is true if the OS is an Enterprise Linux variant of CoreOS
// i.e. RHEL CoreOS (RHCOS) or CentOS Stream CoreOS (SCOS)
func (os OperatingSystem) IsEL() bool {
	return os.id == rhcos || os.id == scos
}

// IsEL8 is true if the OS is RHCOS 8 or SCOS 8
func (os OperatingSystem) IsEL8() bool {
	return os.IsEL() && strings.HasPrefix(os.version, "8.") || os.version == "8"
}

// IsEL9 is true if the OS is RHCOS 9 or SCOS 9
func (os OperatingSystem) IsEL9() bool {
	return os.IsEL() && strings.HasPrefix(os.version, "9.") || os.version == "9"
}

// IsFCOS is true if the OS is Fedora CoreOS
func (os OperatingSystem) IsFCOS() bool {
	return os.id == fedora && os.variantID == coreos
}

// IsSCOS is true if the OS is SCOS
func (os OperatingSystem) IsSCOS() bool {
	return os.id == scos
}

// IsCoreOSVariant is true if the OS is FCOS or a derivative (ostree+Ignition)
// which includes SCOS and RHCOS.
func (os OperatingSystem) IsCoreOSVariant() bool {
	// In RHCOS8 the variant id is not specified. SCOS (future RHCOS9) and FCOS have VARIANT_ID=coreos.
	return os.variantID == coreos || os.IsEL()
}

// IsLikeTraditionalRHEL7 is true if the OS is traditional RHEL7 or CentOS7:
// yum based + kickstart/cloud-init (not Ignition).
func (os OperatingSystem) IsLikeTraditionalRHEL7() bool {
	// Today nothing else is going to show up with a version ID of 7
	if len(os.version) > 2 {
		return strings.HasPrefix(os.version, "7.")
	}
	return os.version == "7"
}

// ToPrometheusLabel returns a value we historically fed to Prometheus
func (os OperatingSystem) ToPrometheusLabel() string {
	// We historically upper cased this
	return strings.ToUpper(os.id)
}

// GetHostRunningOS reads os-release to generate the OperatingSystem data.
func GetHostRunningOS() (OperatingSystem, error) {
	return newOperatingSystem(EtcOSReleasePath, LibOSReleasePath)
}

// GetHostRunningOSFromRoot reads the os-release data from an alternative root
func GetHostRunningOSFromRoot(root string) (OperatingSystem, error) {
	etcPath := filepath.Join(root, EtcOSReleasePath)
	libPath := filepath.Join(root, LibOSReleasePath)
	return newOperatingSystem(etcPath, libPath)
}

// Generates the OperatingSystem data from strings which contain the desired
// content. Mostly useful for testing purposes.
func LoadOSRelease(etcOSReleaseContent, libOSReleaseContent string) (OperatingSystem, error) {
	tempDir, err := os.MkdirTemp("", "")
	if err != nil {
		return OperatingSystem{}, err
	}

	defer os.RemoveAll(tempDir)

	etcOSReleasePath := filepath.Join(tempDir, "etc-os-release")
	libOSReleasePath := filepath.Join(tempDir, "lib-os-release")

	if err := os.WriteFile(etcOSReleasePath, []byte(etcOSReleaseContent), 0o644); err != nil {
		return OperatingSystem{}, err
	}

	if err := os.WriteFile(libOSReleasePath, []byte(libOSReleaseContent), 0o644); err != nil {
		return OperatingSystem{}, err
	}

	return newOperatingSystem(etcOSReleasePath, libOSReleasePath)
}

// Determines the OS version based upon the contents of the RHEL_VERSION, VERSION or VERSION_ID fields.
func getOSVersion(or osrelease.OSRelease) string {
	// If we have the RHEL_VERSION field, we should use that value instead.
	if rhelVersion, ok := or.ADDITIONAL_FIELDS["RHEL_VERSION"]; ok {
		return rhelVersion
	}

	// If we have the OPENSHIFT_VERSION field, we can compute the OS version.
	if openshiftVersion, ok := or.ADDITIONAL_FIELDS["OPENSHIFT_VERSION"]; ok {
		// Move the "." from the middle of the OpenShift version to the end; e.g., 4.12 becomes 412.
		openshiftVersion := strings.ReplaceAll(openshiftVersion, ".", "") + "."
		if strings.HasPrefix(or.VERSION, openshiftVersion) {
			// Strip the OpenShift Version prefix from the VERSION field, if it is found.
			return strings.ReplaceAll(or.VERSION, openshiftVersion, "")
		}
	}

	// Fallback to the VERSION_ID field
	return or.VERSION_ID
}
