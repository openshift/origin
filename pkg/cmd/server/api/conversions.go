package api

func init() {
	Scheme.AddDefaultingFuncs(
		func(obj *MasterConfig) {
			// Historically, the clientCA was incorrectly used as the master's server cert CA bundle
			// If missing from the config, migrate the ClientCA into that field
			if obj.OAuthConfig != nil && obj.OAuthConfig.MasterCA == nil {
				s := obj.ServingInfo.ClientCA
				// The final value of OAuthConfig.MasterCA should never be nil
				obj.OAuthConfig.MasterCA = &s
			}
		},
	)
}
