package templaterouter

import (
	"fmt"
	"time"

	routeapi "github.com/openshift/origin/pkg/route/api"

	"github.com/golang/glog"
)

type AnnotationsFunc func(route *routeapi.Route) []string
type AnnotationValidator func(s string) bool

const (
	HAProxyBase          = "openshift.io/route.haproxy."
	HAProxyTimeoutServer = HAProxyBase + "timeout-server"
)

func AnnotationFuncFor(t string) AnnotationsFunc {
	switch t {
	case AnnotationStrategyHAProxy:
		return HAProxy
	default:
		return NoOp
	}
}

func HAProxy(route *routeapi.Route) []string {
	cfg := []string{}
	for allowed, validator := range AllowedAnnotations(AnnotationStrategyHAProxy) {
		if val, ok := route.Annotations[allowed]; ok {
			if validator(val) {
				cfg = append(cfg, fmt.Sprintf("%s %s", HAProxyAnnotationToConfig(allowed), val))
			} else {
				glog.V(4).Infof("skipping annotation %s due to invalid value %s", allowed, val)
			}
		}
	}
	return cfg
}

func NoOp(route *routeapi.Route) []string {
	return []string{}
}

func AllowedAnnotations(t string) map[string]AnnotationValidator {
	switch t {
	case AnnotationStrategyHAProxy:
		return map[string]AnnotationValidator{
			HAProxyTimeoutServer: isValidTimeout,
		}
	default:
		return map[string]AnnotationValidator{}
	}
}

func HAProxyAnnotationToConfig(s string) string {
	switch s {
	case HAProxyTimeoutServer:
		return "timeout server"
	}
	return ""
}

func isValidTimeout(s string) bool {
	_, err := time.ParseDuration(s)
	return err == nil
}
