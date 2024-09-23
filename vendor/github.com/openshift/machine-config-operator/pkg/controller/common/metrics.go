package common

import (
	"context"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog/v2"
)

const (
	// DefaultBindAddress is the port for the metrics listener
	DefaultBindAddress = ":8797"
)

// MCC Metrics
var (
	// OSImageURLOverride tells whether cluster is using default OS image or has been overridden by user
	OSImageURLOverride = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "os_image_url_override",
			Help: "state of OS image override",
		}, []string{"pool"})

	// MCCDrainErr logs failed drain
	MCCDrainErr = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mcc_drain_err",
			Help: "logs failed drain",
		}, []string{"node"})
	// MCCPoolAlert logs when the pool configuration changes in a way the user should know.
	MCCPoolAlert = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mcc_pool_alert",
			Help: "pool status alert",
		}, []string{"node"})
)

func RegisterMCCMetrics() error {
	err := RegisterMetrics([]prometheus.Collector{
		OSImageURLOverride,
		MCCDrainErr,
		MCCPoolAlert,
	})

	if err != nil {
		return fmt.Errorf("could not register machine-config-controller metrics: %w", err)
	}

	MCCDrainErr.Reset()

	return nil
}

func RegisterMetrics(metrics []prometheus.Collector) error {
	for _, metric := range metrics {
		err := prometheus.Register(metric)
		if err != nil {
			return err
		}
	}

	return nil
}

// StartMetricsListener is metrics listener via http on localhost
func StartMetricsListener(addr string, stopCh <-chan struct{}, registerFunc func() error) {
	if addr == "" {
		addr = DefaultBindAddress
	}

	klog.Info("Registering Prometheus metrics")
	if err := registerFunc(); err != nil {
		klog.Errorf("unable to register metrics: %v", err)
		// No sense in continuing starting the listener if this fails
		return
	}

	klog.Infof("Starting metrics listener on %s", addr)
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	s := http.Server{Addr: addr, Handler: mux}

	go func() {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.Errorf("metrics listener exited with error: %v", err)
		}
	}()
	<-stopCh
	if err := s.Shutdown(context.Background()); err != nil {
		if err != http.ErrServerClosed {
			klog.Errorf("error stopping metrics listener: %v", err)
		}
	} else {
		klog.Infof("Metrics listener successfully stopped")
	}
}
