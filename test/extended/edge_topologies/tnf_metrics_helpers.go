package edge_topologies

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/openshift/origin/test/extended/edge_topologies/utils"
	exutil "github.com/openshift/origin/test/extended/util"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	tnfMetricsTimeout = 3 * time.Minute
	tnfPollInterval   = 10 * time.Second
	tnfCommandTimeout = 2 * time.Minute
)

type prometheusQueryResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Data   struct {
		Result []struct {
			Metric map[string]string `json:"metric"`
		} `json:"result"`
	} `json:"data"`
}

type prometheusRulesResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Data   struct {
		Groups []struct {
			Rules []struct {
				Name string `json:"name"`
			} `json:"rules"`
		} `json:"groups"`
	} `json:"data"`
}

type metricKey struct {
	name     string
	node     string
	resource string
}

var (
	expectedTNFAlertRules = []string{
		"TNFClusterInMaintenance",
		"TNFNodeCountMismatch",
		"TNFNodeFencingDegraded",
		"TNFNodeFencingUnavailable",
		"TNFNodeInMaintenance",
		"TNFNodeOffline",
		"TNFNodeStandby",
		"TNFNodeUnclean",
		"TNFResourceDisabled",
		"TNFResourceFailed",
		"TNFResourceStopped",
		"TNFResourceUnmanaged",
	}
	clusterMetricNames = []string{
		"tnf_cluster_healthy",
		"tnf_cluster_in_service",
		"tnf_cluster_node_count_as_expected",
	}
	nodeMetricNames = []string{
		"tnf_node_healthy",
		"tnf_node_online",
		"tnf_node_in_service",
		"tnf_node_active",
		"tnf_node_ready",
		"tnf_node_clean",
		"tnf_node_member",
		"tnf_node_fencing_available",
		"tnf_node_fencing_healthy",
	}
	resourceMetricNames = []string{
		"tnf_resource_healthy",
		"tnf_resource_in_service",
		"tnf_resource_managed",
		"tnf_resource_enabled",
		"tnf_resource_operational",
		"tnf_resource_active",
		"tnf_resource_started",
		"tnf_resource_schedulable",
	}
)

func expectedHealthyTNFGauges(nodes []string) map[metricKey]float64 {
	expected := map[metricKey]float64{}
	for _, name := range clusterMetricNames {
		expected[metricKey{name: name}] = 1
	}
	for _, node := range nodes {
		for _, name := range nodeMetricNames {
			expected[metricKey{name: name, node: node}] = 1
		}
		for _, resource := range []string{"Etcd", "Kubelet"} {
			for _, name := range resourceMetricNames {
				expected[metricKey{name: name, node: node, resource: resource}] = 1
			}
		}
	}
	return expected
}

func clusterMaintenanceTNFGauges(nodes []string) map[metricKey]float64 {
	expected := map[metricKey]float64{
		{name: "tnf_cluster_healthy"}:    0,
		{name: "tnf_cluster_in_service"}: 0,
	}
	for _, node := range nodes {
		expected[metricKey{name: "tnf_node_fencing_healthy", node: node}] = 0
		for _, resource := range []string{"Etcd", "Kubelet"} {
			for _, name := range []string{"tnf_resource_healthy", "tnf_resource_in_service", "tnf_resource_managed"} {
				expected[metricKey{name: name, node: node, resource: resource}] = 0
			}
		}
	}
	return expected
}

func resourceUnmanagedTNFGauges(nodes []string) map[metricKey]float64 {
	expected := map[metricKey]float64{}
	for _, node := range nodes {
		for _, name := range []string{"tnf_resource_managed", "tnf_resource_healthy"} {
			expected[metricKey{name: name, node: node, resource: "Etcd"}] = 0
		}
	}
	return expected
}

func nodeMaintenanceTNFGauges(node string) map[metricKey]float64 {
	expected := map[metricKey]float64{
		{name: "tnf_cluster_healthy"}:             0,
		{name: "tnf_node_healthy", node: node}:    0,
		{name: "tnf_node_in_service", node: node}: 0,
	}
	for _, resource := range []string{"Etcd", "Kubelet"} {
		for _, name := range []string{"tnf_resource_healthy", "tnf_resource_in_service", "tnf_resource_managed"} {
			expected[metricKey{name: name, node: node, resource: resource}] = 0
		}
	}
	return expected
}

func fenceDisabledTNFGauges(node string) map[metricKey]float64 {
	return map[metricKey]float64{
		{name: "tnf_cluster_healthy"}:                    0,
		{name: "tnf_node_fencing_available", node: node}: 0,
		{name: "tnf_node_fencing_healthy", node: node}:   0,
		{name: "tnf_node_healthy", node: node}:           0,
	}
}

func parseTNFGauges(exposition string) (map[metricKey]float64, error) {
	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(strings.NewReader(exposition))
	if err != nil {
		return nil, err
	}

	gauges := map[metricKey]float64{}
	for name, family := range families {
		if !strings.HasPrefix(name, "tnf_") || family.GetType() != dto.MetricType_GAUGE {
			continue
		}
		for _, metric := range family.GetMetric() {
			key := metricKey{name: name}
			for _, label := range metric.GetLabel() {
				switch label.GetName() {
				case "node":
					key.node = label.GetValue()
				case "resource":
					key.resource = label.GetValue()
				}
			}
			gauges[key] = metric.GetGauge().GetValue()
		}
	}
	return gauges, nil
}

func parseFiringTNFAlerts(response string) ([]string, error) {
	result := prometheusQueryResponse{}
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("parse Prometheus response: %w", err)
	}
	if result.Status != "success" {
		return nil, fmt.Errorf("Prometheus query failed: %s", result.Error)
	}

	alerts := make([]string, 0, len(result.Data.Result))
	for _, sample := range result.Data.Result {
		alerts = append(alerts, sample.Metric["alertname"])
	}
	return alerts, nil
}

func parseLoadedTNFAlertRules(response string) ([]string, error) {
	result := prometheusRulesResponse{}
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("parse Prometheus rules response: %w", err)
	}
	if result.Status != "success" {
		return nil, fmt.Errorf("Prometheus rules query failed: %s", result.Error)
	}

	rules := []string{}
	for _, group := range result.Data.Groups {
		for _, rule := range group.Rules {
			if strings.HasPrefix(rule.Name, "TNF") {
				rules = append(rules, rule.Name)
			}
		}
	}
	sort.Strings(rules)
	return rules, nil
}

func tnfGaugeMismatches(actual, expected map[metricKey]float64) []string {
	mismatches := []string{}
	for key, expectedValue := range expected {
		actualValue, found := actual[key]
		if !found {
			mismatches = append(mismatches, fmt.Sprintf("%s is missing", key))
			continue
		}
		if actualValue != expectedValue {
			mismatches = append(mismatches, fmt.Sprintf("%s=%v, want %v", key, actualValue, expectedValue))
		}
	}
	sort.Strings(mismatches)
	return mismatches
}

func tnfGaugesWithOverrides(baseline, overrides map[metricKey]float64) map[metricKey]float64 {
	expected := make(map[metricKey]float64, len(baseline))
	for key, value := range baseline {
		expected[key] = value
	}
	for key, value := range overrides {
		expected[key] = value
	}
	return expected
}

func (key metricKey) String() string {
	labels := []string{}
	if key.node != "" {
		labels = append(labels, fmt.Sprintf("node=%q", key.node))
	}
	if key.resource != "" {
		labels = append(labels, fmt.Sprintf("resource=%q", key.resource))
	}
	if len(labels) == 0 {
		return key.name
	}
	return fmt.Sprintf("%s{%s}", key.name, strings.Join(labels, ","))
}

func retryTNFOperation(ctx context.Context, interval, timeout time.Duration, operation func(context.Context) error) error {
	var lastErr error
	err := wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(ctx context.Context) (bool, error) {
		if err := operation(ctx); err != nil {
			lastErr = err
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("operation did not succeed within %s; last error: %v: %w", timeout, lastErr, err)
	}
	return nil
}

func runTNFOC(ctx context.Context, oc *exutil.CLI, args ...string) (string, error) {
	commandCtx, cancel := context.WithTimeout(ctx, tnfCommandTimeout)
	defer cancel()

	cmd, stdout, stderr, err := oc.AsAdmin().WithoutNamespace().Run(args[0]).Args(args[1:]...).Background()
	if err != nil {
		return "", fmt.Errorf("start oc %s: %w", strings.Join(args, " "), err)
	}
	if err := waitForTNFCommand(commandCtx, cmd); err != nil {
		return "", fmt.Errorf("oc %s: %w: stdout=%s stderr=%s", strings.Join(args, " "), err, strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func waitForTNFCommand(ctx context.Context, cmd *exec.Cmd) error {
	resultCh := make(chan error, 1)
	go func() {
		resultCh <- cmd.Wait()
	}()

	select {
	case err := <-resultCh:
		return err
	case <-ctx.Done():
		killErr := cmd.Process.Kill()
		waitErr := <-resultCh
		if killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
			return fmt.Errorf("%w; kill command: %v; wait command: %v", ctx.Err(), killErr, waitErr)
		}
		return ctx.Err()
	}
}

func runTNFPCS(ctx context.Context, oc *exutil.CLI, node string, args ...string) error {
	ocArgs := []string{"debug", "-q", "node/" + node, "--", "chroot", "/host", "pcs"}
	ocArgs = append(ocArgs, args...)
	_, err := runTNFOC(ctx, oc, ocArgs...)
	return err
}

func queryTNFGauges(ctx context.Context, oc *exutil.CLI) (map[metricKey]float64, error) {
	const command = `TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token); curl -skS --fail -H "Authorization: Bearer ${TOKEN}" https://localhost:8443/metrics`
	output, err := runTNFOC(ctx, oc, "exec", "-n", "openshift-etcd-operator", "deploy/etcd-operator", "-c", "etcd-operator", "--", "sh", "-c", command)
	if err != nil {
		return nil, err
	}
	return parseTNFGauges(output)
}

func waitForTNFGauges(ctx context.Context, oc *exutil.CLI, expected map[metricKey]float64) error {
	lastObservation := "metrics were not queried"
	err := wait.PollUntilContextTimeout(ctx, tnfPollInterval, tnfMetricsTimeout, true, func(ctx context.Context) (bool, error) {
		actual, err := queryTNFGauges(ctx, oc)
		if err != nil {
			lastObservation = err.Error()
			return false, nil
		}
		mismatches := tnfGaugeMismatches(actual, expected)
		if len(mismatches) > 0 {
			lastObservation = strings.Join(mismatches, "; ")
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("TNF metrics did not reach expected values within %s: %s: %w", tnfMetricsTimeout, lastObservation, err)
	}
	return nil
}

func verifyTNFPrometheusRule(ctx context.Context, oc *exutil.CLI, prometheusPod string) error {
	lastObservation := "Prometheus rules were not queried"
	err := wait.PollUntilContextTimeout(ctx, tnfPollInterval, tnfMetricsTimeout, true, func(ctx context.Context) (bool, error) {
		output, err := runPrometheusCurl(ctx, oc, prometheusPod, "http://localhost:9090/api/v1/rules?type=alert")
		if err != nil {
			lastObservation = err.Error()
			return false, nil
		}
		rules, err := parseLoadedTNFAlertRules(output)
		if err != nil {
			lastObservation = err.Error()
			return false, nil
		}
		if strings.Join(rules, ",") != strings.Join(expectedTNFAlertRules, ",") {
			lastObservation = fmt.Sprintf("loaded TNF alert rules = %v, want %v", rules, expectedTNFAlertRules)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("TNF PrometheusRule was not loaded within %s: %s: %w", tnfMetricsTimeout, lastObservation, err)
	}
	return nil
}

func firingTNFAlerts(ctx context.Context, oc *exutil.CLI, prometheusPod string) ([]string, error) {
	const queryURL = `http://localhost:9090/api/v1/query?query=ALERTS%7Balertname%3D~%22TNF.%2A%22%2Calertstate%3D%22firing%22%7D`
	output, err := runPrometheusCurl(ctx, oc, prometheusPod, queryURL)
	if err != nil {
		return nil, err
	}
	return parseFiringTNFAlerts(output)
}

func waitForNoFiringTNFAlerts(ctx context.Context, oc *exutil.CLI, prometheusPod string) error {
	lastObservation := "Prometheus alerts were not queried"
	err := wait.PollUntilContextTimeout(ctx, tnfPollInterval, tnfMetricsTimeout, true, func(ctx context.Context) (bool, error) {
		alerts, err := firingTNFAlerts(ctx, oc, prometheusPod)
		if err != nil {
			lastObservation = err.Error()
			return false, nil
		}
		if len(alerts) > 0 {
			lastObservation = fmt.Sprintf("firing TNF alerts: %v", alerts)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("TNF alerts did not clear within %s: %s: %w", tnfMetricsTimeout, lastObservation, err)
	}
	return nil
}

func runPrometheusCurl(ctx context.Context, oc *exutil.CLI, prometheusPod, url string) (string, error) {
	return runTNFOC(ctx, oc, "exec", "-n", "openshift-monitoring", prometheusPod, "-c", "prometheus", "--", "curl", "-sS", "--fail", url)
}

func waitForTNFClusterHealthy(oc *exutil.CLI) error {
	return utils.IsClusterHealthyWithTimeout(oc, tnfMetricsTimeout)
}
