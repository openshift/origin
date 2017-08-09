package v2

// APIVersion represents a specific version of the OSB API.
type APIVersion struct {
	label string
	order byte
}

// AtLeast returns whether the API version is greater than or equal to the
// given API version.
func (v APIVersion) AtLeast(test APIVersion) bool {
	return v.order >= test.order
}

// HeaderValue returns the value that should be sent in the API version header
// for this API version.
func (v APIVersion) HeaderValue() string {
	return v.label
}

const (
	// internalAPIVersion2_11 represents the 2.11 version of the Open Service
	// Broker API.
	internalAPIVersion2_11 = "2.11"

	// internalAPIVersion2_11 represents the 2.11 version of the Open Service
	// Broker API.
	internalAPIVersion2_12 = "2.12"
)

func Version2_12() APIVersion {
	return APIVersion{label: internalAPIVersion2_12, order: 1}
}

func Version2_11() APIVersion {
	return APIVersion{label: internalAPIVersion2_12, order: 0}
}

// LatestAPIVersion returns the latest supported API version in the current
// release of this library.
func LatestAPIVersion() APIVersion {
	return Version2_12()
}
