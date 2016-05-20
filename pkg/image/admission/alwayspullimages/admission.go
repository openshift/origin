package alwayspullimages

import (
	"io"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/admission"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/plugin/pkg/admission/alwayspullimages"

	configlatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
)

func init() {
	admission.RegisterPlugin("AlwaysPullImages", func(client clientset.Interface, config io.Reader) (admission.Interface, error) {
		activated, err := configlatest.IsAdmissionPluginActivated(config, false)
		if err != nil {
			return nil, err
		}
		if !activated {
			glog.Infof("Admission plugin %q is not enabled so it will be off.", "AlwaysPullImages")
			return nil, nil
		}
		return alwayspullimages.NewAlwaysPullImages(), nil
	})
}
