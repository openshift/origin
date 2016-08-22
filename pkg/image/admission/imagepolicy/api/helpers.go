package api

// RequestsResolution returns true if you should attempt to resolve image pull specs
func RequestsResolution(imageResolutionType ImageResolutionType) bool {
	switch imageResolutionType {
	case RequiredRewrite, Required, AttemptRewrite, Attempt:
		return true
	}
	return false
}

// FailOnResolutionFailure returns true if you should fail when resolution fails
func FailOnResolutionFailure(imageResolutionType ImageResolutionType) bool {
	switch imageResolutionType {
	case RequiredRewrite, Required:
		return true
	}
	return false
}

// RewriteImagePullSpec returns true if you should rewrite image pull specs when resolution succeeds
func RewriteImagePullSpec(imageResolutionType ImageResolutionType) bool {
	switch imageResolutionType {
	case RequiredRewrite, AttemptRewrite:
		return true
	}
	return false
}
