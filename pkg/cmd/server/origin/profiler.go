package origin

import (
	"fmt"
	"net/http"
	"runtime"

	"github.com/golang/glog"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

func StartProfiler() {
	if cmdutil.Env("OPENSHIFT_PROFILE", "") == "web" {
		go func() {
			runtime.SetBlockProfileRate(1)
			profilePort := cmdutil.Env("OPENSHIFT_PROFILE_PORT", "6060")
			profileHost := cmdutil.Env("OPENSHIFT_PROFILE_HOST", "127.0.0.1")
			glog.Infof(fmt.Sprintf("Starting profiling endpoint at http://%s:%s/debug/pprof/", profileHost, profilePort))
			glog.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%s", profileHost, profilePort), nil))
		}()
	}
}
