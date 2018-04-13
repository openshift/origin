package origin

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"runtime"

	"github.com/golang/glog"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

func StartProfiler() {
	if cmdutil.Env("OPENSHIFT_PROFILE", "") == "web" {
		go func() {
			serveMux := http.NewServeMux()
			serveMux.HandleFunc("/debug/pprof/", pprof.Index)
			serveMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
			serveMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
			serveMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
			serveMux.HandleFunc("/debug/pprof/trace", pprof.Trace)

			runtime.SetBlockProfileRate(1)
			profilePort := cmdutil.Env("OPENSHIFT_PROFILE_PORT", "6060")
			profileHost := cmdutil.Env("OPENSHIFT_PROFILE_HOST", "127.0.0.1")
			glog.Infof(fmt.Sprintf("Starting profiling endpoint at http://%s:%s/debug/pprof/", profileHost, profilePort))
			server := http.Server{
				Addr:    fmt.Sprintf("%s:%s", profileHost, profilePort),
				Handler: serveMux,
			}
			glog.Fatal(server.ListenAndServe())
		}()
	}
}
