package openshiftkubeapiserver

import (
	"k8s.io/klog"
	"net/http"
	"strings"

	"k8s.io/apiserver/pkg/server/healthz"
)

// LogRequestToHealthz logs a user-agent that uses /healthz endpoint
var LogRequestToHealthz healthz.HealthChecker = &logRequestToHealthz{}

type logRequestToHealthz struct {}

func (l *logRequestToHealthz) Name() string {
	return "logRequestToHealthz"
}

func (l *logRequestToHealthz) Check(r *http.Request) error {
	userAgent := r.Header.Get("User-Agent")

	// it seems that kubelet identifies itself as "kube-probe/1.18+",
	if len(userAgent) > 0 && !strings.Contains(userAgent, "kube-probe") {
		klog.V(2).Infof("User-Agent:%q called %q path", userAgent, r.URL.Path)
	}

	return nil
}
