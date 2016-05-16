package controller

import (
	"github.com/golang/glog"

	routeapi "github.com/openshift/origin/pkg/route/api"
)

// RejectionRecorder is an object capable of recording why a route was rejected.
type RejectionRecorder interface {
	RecordRouteRejection(route *routeapi.Route, reason, message string)
}

var LogRejections = logRecorder{}

type logRecorder struct{}

func (_ logRecorder) RecordRouteRejection(route *routeapi.Route, reason, message string) {
	glog.V(4).Infof("Rejected route %s: %s: %s", route.Name, reason, message)
}
