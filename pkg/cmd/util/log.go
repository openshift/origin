package util

import (
	"k8s.io/klog"

	"github.com/openshift/library-go/pkg/serviceability"
)

// InitLogrus sets the logrus trace level based on the glog trace level.
func InitLogrus() {
	switch {
	case bool(klog.V(4)):
		serviceability.InitLogrus("DEBUG")
	case bool(klog.V(2)):
		serviceability.InitLogrus("INFO")
	case bool(klog.V(0)):
		serviceability.InitLogrus("WARN")
	}
}
