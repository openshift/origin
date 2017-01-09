package util

const (
	ProductOpenShift = `OpenShift`
)

// GetProductName chooses appropriate product for a binary name.
func GetProductName(binaryName string) string {
	return ProductOpenShift
}

// GetPlatformName returns an appropriate platform name for given binary name.
// Platform name can be used as a headline in command's usage.
func GetPlatformName(binaryName string) string {
	return "OpenShift Application Platform"
}

// GetDistributionName returns an appropriate Kubernetes distribution name.
// Distribution name can be used in relation to some feature set in command's
// usage string (e.g. <distribution name> allows you to build, run, etc.).
func GetDistributionName(binaryName string) string {
	return "OpenShift distribution of Kubernetes"
}
