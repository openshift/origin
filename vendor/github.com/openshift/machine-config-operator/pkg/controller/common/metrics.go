package common

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"

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
	// MCCSubControllerState logs the state of the subcontrollers of the MCC
	MCCSubControllerState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mcc_sub_controller_state",
			Help: "state of sub-controllers in the MCC",
		}, []string{"subcontroller", "state", "object"})
)

func RegisterMCCMetrics() error {
	err := RegisterMetrics([]prometheus.Collector{
		OSImageURLOverride,
		MCCDrainErr,
		MCCPoolAlert,
		MCCSubControllerState,
	})

	if err != nil {
		return fmt.Errorf("could not register machine-config-controller metrics: %w", err)
	}

	// Initilize GuageVecs to ensure that metrics of type GuageVec are accessible from the dashboard even if without a logged value
	// Solution to OCPBUGS-20427: https://issues.redhat.com/browse/OCPBUGS-20427
	OSImageURLOverride.WithLabelValues("initialize").Set(0)
	MCCDrainErr.WithLabelValues("initialize").Set(0)
	MCCPoolAlert.WithLabelValues("initialize").Set(0)
	MCCSubControllerState.WithLabelValues("initialize", "initialize", "initialize").Set(0)

	return nil
}

func UpdateStateMetric(metric *prometheus.GaugeVec, labels ...string) {
	metric.WithLabelValues(labels...).SetToCurrentTime()
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
	s := http.Server{
		TLSConfig: &tls.Config{
			MinVersion:   tls.VersionTLS12,
			NextProtos:   []string{"http/1.1"},
			CipherSuites: cipherOrder(),
		},
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
		Addr:         addr,
		Handler:      mux}

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

func cipherOrder() []uint16 {
	var first []uint16
	var second []uint16

	allowable := func(c *tls.CipherSuite) bool {
		// Disallow block ciphers using straight SHA1
		// See: https://tools.ietf.org/html/rfc7540#appendix-A
		if strings.HasSuffix(c.Name, "CBC_SHA") {
			return false
		}
		// 3DES is considered insecure
		if strings.Contains(c.Name, "3DES") {
			return false
		}
		return true
	}

	for _, c := range tls.CipherSuites() {
		for _, v := range c.SupportedVersions {
			if v == tls.VersionTLS13 {
				first = append(first, c.ID)
			}
			if v == tls.VersionTLS12 && allowable(c) {
				inFirst := false
				for _, id := range first {
					if c.ID == id {
						inFirst = true
						break
					}
				}
				if !inFirst {
					second = append(second, c.ID)
				}
			}
		}
	}

	return append(first, second...)
}
