package metrics

import (
	"net/http"
	"net/http/pprof"

	"k8s.io/kubernetes/pkg/healthz"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
)

// Listen starts a server for health, metrics, and profiling on the provided listen port.
// It will terminate the process if the server fails. Metrics and profiling are only exposed
// if username and password are provided and the user's input matches.
func Listen(listenAddr string, username, password string) {
	go func() {
		mux := http.NewServeMux()
		healthz.InstallHandler(mux)

		// TODO: exclude etcd and other unused metrics

		// never enable profiling or metrics without protection
		if len(username) > 0 && len(password) > 0 {
			protected := http.NewServeMux()
			protected.HandleFunc("/debug/pprof/", pprof.Index)
			protected.HandleFunc("/debug/pprof/profile", pprof.Profile)
			protected.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
			protected.Handle("/metrics", prometheus.Handler())
			mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
				if u, p, ok := req.BasicAuth(); !ok || u != username || p != password {
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
				protected.ServeHTTP(w, req)
			})
		}

		server := &http.Server{
			Addr:    listenAddr,
			Handler: mux,
		}
		glog.Fatal(server.ListenAndServe())
	}()

}
