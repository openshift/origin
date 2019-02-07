package externalipranger

import (
	"io"

	"github.com/golang/glog"
	"k8s.io/apiserver/pkg/admission"
)

// Deprecated  we need this to ratchet a rename across our operator
func DeprecatedRegisterExternalIP(plugins *admission.Plugins) {
	plugins.Register("ExternalIPRanger",
		func(config io.Reader) (admission.Interface, error) {
			pluginConfig, err := readConfig(config)
			if err != nil {
				return nil, err
			}
			if pluginConfig == nil {
				glog.Infof("Admission plugin %q is not configured so it will be disabled.", ExternalIPPluginName)
				return nil, nil
			}

			// this needs to be moved upstream to be part of core config
			reject, admit, err := ParseRejectAdmitCIDRRules(pluginConfig.ExternalIPNetworkCIDRs)
			if err != nil {
				// should have been caught with validation
				return nil, err
			}

			return NewExternalIPRanger(reject, admit, pluginConfig.AllowIngressIP), nil
		})
}
