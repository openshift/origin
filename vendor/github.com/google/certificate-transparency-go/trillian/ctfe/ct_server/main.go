// Copyright 2016 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// The ct_server binary runs the CT personality.
package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/coreos/etcd/clientv3"
	etcdnaming "github.com/coreos/etcd/clientv3/naming"
	"github.com/golang/glog"
	"github.com/google/certificate-transparency-go/trillian/ctfe"
	"github.com/google/certificate-transparency-go/trillian/ctfe/configpb"
	"github.com/google/certificate-transparency-go/trillian/util"
	"github.com/google/trillian"
	"github.com/google/trillian/monitoring/opencensus"
	"github.com/google/trillian/monitoring/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
	"github.com/tomasen/realip"
	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/naming"

	// Register PEMKeyFile, PrivateKey and PKCS11Config ProtoHandlers
	_ "github.com/google/trillian/crypto/keys/der/proto"
	_ "github.com/google/trillian/crypto/keys/pem/proto"
	_ "github.com/google/trillian/crypto/keys/pkcs11/proto"
)

// Global flags that affect all log instances.
var (
	httpEndpoint       = flag.String("http_endpoint", "localhost:6962", "Endpoint for HTTP (host:port)")
	metricsEndpoint    = flag.String("metrics_endpoint", "localhost:6963", "Endpoint for serving metrics; if left empty, metrics will be visible on --http_endpoint")
	rpcBackend         = flag.String("log_rpc_server", "localhost:8090", "Backend specification; comma-separated list or etcd service name (if --etcd_servers specified). If unset backends are specified in config (as a LogMultiConfig proto)")
	rpcDeadline        = flag.Duration("rpc_deadline", time.Second*10, "Deadline for backend RPC requests")
	getSTHInterval     = flag.Duration("get_sth_interval", time.Second*180, "Interval between internal get-sth operations (0 to disable)")
	logConfig          = flag.String("log_config", "", "File holding log config in text proto format")
	maxGetEntries      = flag.Int64("max_get_entries", 0, "Max number of entries we allow in a get-entries request (0=>use default 1000)")
	etcdServers        = flag.String("etcd_servers", "", "A comma-separated list of etcd servers")
	etcdHTTPService    = flag.String("etcd_http_service", "trillian-ctfe-http", "Service name to announce our HTTP endpoint under")
	etcdMetricsService = flag.String("etcd_metrics_service", "trillian-ctfe-metrics-http", "Service name to announce our HTTP metrics endpoint under")
	tracing            = flag.Bool("tracing", false, "If true opencensus Stackdriver tracing will be enabled. See https://opencensus.io/.")
	tracingProjectID   = flag.String("tracing_project_id", "", "project ID to pass to stackdriver. Can be empty for GCP, consult docs for other platforms.")
	tracingPercent     = flag.Int("tracing_percent", 0, "Percent of requests to be traced. Zero is a special case to use the DefaultSampler")
	quotaRemote        = flag.Bool("quota_remote", true, "Enable requesting of quota for IP address sending incoming requests")
	quotaIntermediate  = flag.Bool("quota_intermediate", true, "Enable requesting of quota for intermediate certificates in sumbmitted chains")
	handlerPrefix      = flag.String("handler_prefix", "", "If set e.g. to '/logs' will prefix all handlers that don't define a custom prefix")
)

func main() {
	flag.Parse()
	ctx := context.Background()

	if *maxGetEntries > 0 {
		ctfe.MaxGetEntriesAllowed = *maxGetEntries
	}

	var cfg *configpb.LogMultiConfig
	var err error
	// Get log config from file before we start. This is a different proto
	// type if we're using a multi backend configuration (no rpcBackend set
	// in flags). The single-backend config is converted to a multi config so
	// they can be treated the same.
	if len(*rpcBackend) > 0 {
		var cfgs []*configpb.LogConfig
		if cfgs, err = ctfe.LogConfigFromFile(*logConfig); err == nil {
			cfg = ctfe.ToMultiLogConfig(cfgs, *rpcBackend)
		}
	} else {
		cfg, err = ctfe.MultiLogConfigFromFile(*logConfig)
	}

	if err != nil {
		glog.Exitf("Failed to read config: %v", err)
	}

	beMap, err := ctfe.ValidateLogMultiConfig(cfg)
	if err != nil {
		glog.Exitf("Invalid config: %v", err)
	}

	glog.CopyStandardLogTo("WARNING")
	glog.Info("**** CT HTTP Server Starting ****")

	metricsAt := *metricsEndpoint
	if metricsAt == "" {
		metricsAt = *httpEndpoint
	}

	dialOpts := []grpc.DialOption{grpc.WithInsecure()}
	if len(*etcdServers) > 0 {
		// Use etcd to provide endpoint resolution.
		cfg := clientv3.Config{Endpoints: strings.Split(*etcdServers, ","), DialTimeout: 5 * time.Second}
		client, err := clientv3.New(cfg)
		if err != nil {
			glog.Exitf("Failed to connect to etcd at %v: %v", *etcdServers, err)
		}
		etcdRes := &etcdnaming.GRPCResolver{Client: client}
		dialOpts = append(dialOpts, grpc.WithBalancer(grpc.RoundRobin(etcdRes)))

		// Also announce ourselves.
		updateHTTP := naming.Update{Op: naming.Add, Addr: *httpEndpoint}
		updateMetrics := naming.Update{Op: naming.Add, Addr: metricsAt}
		glog.Infof("Announcing our presence in %v with %+v", *etcdHTTPService, updateHTTP)
		etcdRes.Update(ctx, *etcdHTTPService, updateHTTP)
		glog.Infof("Announcing our presence in %v with %+v", *etcdMetricsService, updateMetrics)
		etcdRes.Update(ctx, *etcdMetricsService, updateMetrics)

		byeHTTP := naming.Update{Op: naming.Delete, Addr: *httpEndpoint}
		byeMetrics := naming.Update{Op: naming.Delete, Addr: metricsAt}
		defer func() {
			glog.Infof("Removing our presence in %v with %+v", *etcdHTTPService, byeHTTP)
			etcdRes.Update(ctx, *etcdHTTPService, byeHTTP)
			glog.Infof("Removing our presence in %v with %+v", *etcdMetricsService, byeMetrics)
			etcdRes.Update(ctx, *etcdMetricsService, byeMetrics)
		}()
	} else if strings.Contains(*rpcBackend, ",") {
		glog.Infof("Using FixedBackendResolver")
		// Use a fixed endpoint resolution that just returns the addresses configured on the command line.
		res := util.FixedBackendResolver{}
		dialOpts = append(dialOpts, grpc.WithBalancer(grpc.RoundRobin(res)))
	} else {
		glog.Infof("Using regular DNS resolver")
		dialOpts = append(dialOpts, grpc.WithBalancerName(roundrobin.Name))
	}

	// Dial all our log backends.
	clientMap := make(map[string]trillian.TrillianLogClient)
	for _, be := range beMap {
		glog.Infof("Dialling backend: %v", be)
		if len(beMap) == 1 {
			// If there's only one of them we use the blocking option as we can't
			// serve anything until connected.
			dialOpts = append(dialOpts, grpc.WithBlock())
		}
		conn, err := grpc.Dial(be.BackendSpec, dialOpts...)
		if err != nil {
			glog.Exitf("Could not dial RPC server: %v: %v", be, err)
		}
		defer conn.Close()
		clientMap[be.Name] = trillian.NewTrillianLogClient(conn)
	}

	// Allow cross-origin requests to all handlers registered on corsMux.
	// This is safe for CT log handlers because the log is public and
	// unauthenticated so cross-site scripting attacks are not a concern.
	corsMux := http.NewServeMux()
	corsHandler := cors.AllowAll().Handler(corsMux)
	http.Handle("/", corsHandler)

	// Register handlers for all the configured logs using the correct RPC
	// client.
	for _, c := range cfg.LogConfigs.Config {
		if err := setupAndRegister(ctx, clientMap[c.LogBackendName], *rpcDeadline, c, corsMux, *handlerPrefix); err != nil {
			glog.Exitf("Failed to set up log instance for %+v: %v", cfg, err)
		}
	}

	// Return a 200 on the root, for GCE default health checking :/
	corsMux.HandleFunc("/", func(resp http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/" {
			resp.WriteHeader(http.StatusOK)
		} else {
			resp.WriteHeader(http.StatusNotFound)
		}
	})

	if metricsAt != *httpEndpoint {
		// Run a separate handler for metrics.
		go func() {
			mux := http.NewServeMux()
			mux.Handle("/metrics", promhttp.Handler())
			metricsServer := http.Server{Addr: metricsAt, Handler: mux}
			err := metricsServer.ListenAndServe()
			glog.Warningf("Metrics server exited: %v", err)
		}()
	} else {
		// Handle metrics on the DefaultServeMux.
		http.Handle("/metrics", promhttp.Handler())
	}

	if *getSTHInterval > 0 {
		// Regularly update the internal STH for each log so our metrics stay up-to-date with any tree head
		// changes that are not triggered by us.
		for _, c := range cfg.LogConfigs.Config {
			// TODO(pavelkalinnikov): Update mirror STHs when there is a way to.
			if c.IsMirror {
				continue
			}
			ticker := time.NewTicker(*getSTHInterval)
			go func(c *configpb.LogConfig) {
				glog.Infof("start internal get-sth operations on log %v (%d)", c.Prefix, c.LogId)
				for t := range ticker.C {
					glog.V(1).Infof("tick at %v: force internal get-sth for log %v (%d)", t, c.Prefix, c.LogId)
					if err := ctfe.PingTreeHead(ctx, clientMap[c.LogBackendName], c.LogId, c.Prefix); err != nil {
						glog.Warningf("failed to retrieve tree head for log %v (%d): %v", c.Prefix, c.LogId, err)
					}
				}
			}(c)
		}
	}

	// If we're enabling tracing we need to use an instrumented http.Handler.
	var handler http.Handler
	if *tracing {
		handler, err = opencensus.EnableHTTPServerTracing(*tracingProjectID, *tracingPercent)
		if err != nil {
			glog.Exitf("Failed to initialize stackdriver / opencensus tracing: %v", err)
		}
	}

	// Bring up the HTTP server and serve until we get a signal not to.
	srv := http.Server{Addr: *httpEndpoint, Handler: handler}
	shutdownWG := new(sync.WaitGroup)
	go awaitSignal(func() {
		shutdownWG.Add(1)
		defer shutdownWG.Done()
		// Allow 60s for any pending requests to finish then terminate any stragglers
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
		defer cancel()
		glog.Info("Shutting down HTTP server...")
		srv.Shutdown(ctx)
		glog.Info("HTTP server shutdown")
	})

	err = srv.ListenAndServe()
	if err != http.ErrServerClosed {
		glog.Warningf("Server exited: %v", err)
	}
	// Wait will only block if the function passed to awaitSignal was called,
	// in which case it'll block until the HTTP server has gracefully shutdown
	shutdownWG.Wait()
	glog.Flush()
}

// awaitSignal waits for standard termination signals, then runs the given
// function; it should be run as a separate goroutine.
func awaitSignal(doneFn func()) {
	// Arrange notification for the standard set of signals used to terminate a server
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// Now block main and wait for a signal
	sig := <-sigs
	glog.Warningf("Signal received: %v", sig)
	glog.Flush()

	doneFn()
}

func setupAndRegister(ctx context.Context, client trillian.TrillianLogClient, deadline time.Duration, cfg *configpb.LogConfig, mux *http.ServeMux, globalHandlerPrefix string) error {
	if cfg.IsMirror {
		return errors.New("mirrors are not supported")
	}

	opts := ctfe.InstanceOptions{
		Config:        cfg,
		Client:        client,
		Deadline:      deadline,
		MetricFactory: prometheus.MetricFactory{},
		RequestLog:    new(ctfe.DefaultRequestLog),
	}
	if *quotaRemote {
		glog.Info("Enabling quota for requesting IP")
		opts.RemoteQuotaUser = realip.FromRequest
	}
	if *quotaIntermediate {
		glog.Info("Enabling quota for intermediate certificates")
		opts.CertificateQuotaUser = ctfe.QuotaUserForCert
	}
	// Full handler pattern will be of the form "/logs/yyz/ct/v1/add-chain", where "/logs" is the
	// HandlerPrefix and "yyz" is the c.Prefix for this particular log. Use the default
	// HandlerPrefix unless the log config overrides it. The custom prefix in
	// the log configuration intended for use in migration scenarios where logs
	// have an existing URL path that differs from the global one. For example
	// if all new logs are served on "/logs/log/..." and a previously existing
	// log is at "/log/..." this is now supported.
	lhp := globalHandlerPrefix
	if len(opts.Config.OverrideHandlerPrefix) > 0 {
		glog.Infof("Log with prefix: %s is using a custom HandlerPrefix: %s", opts.Config.Prefix, opts.Config.OverrideHandlerPrefix)
		lhp = "/" + strings.Trim(opts.Config.OverrideHandlerPrefix, "/")
	}
	handlers, err := ctfe.SetUpInstance(ctx, opts)
	if err != nil {
		return err
	}
	for path, handler := range *handlers {
		mux.Handle(lhp+path, handler)
	}
	return nil
}
