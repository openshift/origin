package serviceability

import (
	"fmt"
	"net/http"
	"os"
	"runtime"

	_ "net/http/pprof" // include the default Go profiler mux

	"k8s.io/klog/v2"
)

// StartProfiler starts the golang profiler on a port if `web` is specified.  It uses the "standard" openshift env vars
func StartProfiler() {
	if env("OPENSHIFT_PROFILE", "") == "web" {
		go func() {
			runtime.SetBlockProfileRate(1)
			profilePort := env("OPENSHIFT_PROFILE_PORT", "6060")
			profileHost := env("OPENSHIFT_PROFILE_HOST", "127.0.0.1")
			klog.Infof(fmt.Sprintf("Starting profiling endpoint at http://%s:%s/debug/pprof/", profileHost, profilePort))
			klog.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%s", profileHost, profilePort), nil))
		}()
	}
}

// env returns an environment variable or a default value if not specified.
func env(key string, defaultValue string) string {
	val := os.Getenv(key)
	if len(val) == 0 {
		return defaultValue
	}
	return val
}
