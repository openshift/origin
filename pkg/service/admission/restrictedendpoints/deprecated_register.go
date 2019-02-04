package restrictedendpoints

import (
	"io"

	"github.com/golang/glog"
	"k8s.io/apiserver/pkg/admission"
)

// Deprecated  we need this to ratchet a rename across our operator
func DeprecatedRegisterRestrictedEndpoints(plugins *admission.Plugins) {
	plugins.Register("openshift.io/RestrictedEndpointsAdmission",
		func(config io.Reader) (admission.Interface, error) {
			pluginConfig, err := readConfig(config)
			if err != nil {
				return nil, err
			}
			if pluginConfig == nil {
				glog.Infof("Admission plugin %q is not configured so it will be disabled.", RestrictedEndpointsPluginName)
				return nil, nil
			}
			restrictedNetworks, err := ParseSimpleCIDRRules(pluginConfig.RestrictedCIDRs)
			if err != nil {
				// should have been caught with validation
				return nil, err
			}

			return NewRestrictedEndpointsAdmission(restrictedNetworks), nil
		})
}
