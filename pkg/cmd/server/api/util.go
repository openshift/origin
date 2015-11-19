package api

// IsBuildDisabled returns true if the builder feature is disabled.
func IsBuildEnabled(config *MasterConfig) bool {
	return !config.DisabledFeatures.Has(FeatureBuilder)
}
