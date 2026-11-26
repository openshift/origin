package edge_topologies

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	etcdv1 "github.com/openshift/api/etcd/v1"
	"github.com/openshift/api/features"
	"github.com/openshift/origin/test/extended/edge_topologies/utils/apis"
	"github.com/openshift/origin/test/extended/edge_topologies/utils/services"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
)

const (
	tnfObservabilityTimeout = 5 * time.Minute
	tnfCollectorNamespace   = "openshift-etcd"
	tnfCollectorName        = "pacemaker-status-collector"
	tnfCollectorVersionEnv  = "OPERATOR_IMAGE_VERSION"
	alertAgentsObserved     = "AlertAgentsObserved"
	featureGateUnresolved   = "FeatureGateUnresolved"
)

type tnfAlertFailure struct {
	node      string // Empty means the cluster-wide registration failed on every node.
	agent     etcdv1.PacemakerClusterAlertAgentName
	condition string
	reason    string
}

// Use the collector's release, not any enabled entry from an in-progress upgrade.
func tnfObservabilityEnabled(ctx context.Context, oc *exutil.CLI) (bool, error) {
	cron, err := oc.AdminKubeClient().BatchV1().CronJobs(tnfCollectorNamespace).Get(ctx, tnfCollectorName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	version := ""
	for _, container := range cron.Spec.JobTemplate.Spec.Template.Spec.Containers {
		for _, env := range container.Env {
			if env.Name == tnfCollectorVersionEnv {
				version = env.Value
			}
		}
	}
	if version == "" || version == "0.0.1-snapshot" {
		cv, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		version = cv.Status.Desired.Version
		if version == "" && len(cv.Status.History) > 0 {
			version = cv.Status.History[0].Version
		}
	}
	gate, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	for _, details := range gate.Status.FeatureGates {
		if version != "" && details.Version == version {
			for _, enabled := range details.Enabled {
				if enabled.Name == features.FeatureGateTNFObservability {
					return true, nil
				}
			}
			return false, nil
		}
	}
	return false, fmt.Errorf("feature gates are unresolved for collector release %q", version)
}

func expectTNFAlertStatus(pc *etcdv1.PacemakerCluster, nodes []string, failures ...tnfAlertFailure) error {
	if pc.Status.Nodes == nil || len(*pc.Status.Nodes) != len(nodes) {
		return fmt.Errorf("expected status for exactly %d nodes", len(nodes))
	}
	clusterHealthy := metav1.ConditionTrue
	for _, name := range nodes {
		var node *etcdv1.PacemakerClusterNodeStatus
		for i := range *pc.Status.Nodes {
			if (*pc.Status.Nodes)[i].NodeName == name {
				node = &(*pc.Status.Nodes)[i]
				break
			}
		}
		if node == nil || len(node.AlertAgents) != 2 {
			return fmt.Errorf("expected both alert agents on each control-plane node")
		}
		if observed := meta.FindStatusCondition(node.Conditions, alertAgentsObserved); observed != nil &&
			observed.Status == metav1.ConditionFalse && observed.Reason == featureGateUnresolved {
			return fmt.Errorf("node %s retained its alert-agent status because the feature gate is unresolved", name)
		}
		nodeHealthy := metav1.ConditionTrue
		for _, agentName := range []etcdv1.PacemakerClusterAlertAgentName{etcdv1.PacemakerClusterAlertAgentNameTaint, etcdv1.PacemakerClusterAlertAgentNameUntaint} {
			var agent *etcdv1.PacemakerClusterAlertAgentStatus
			for i := range node.AlertAgents {
				if node.AlertAgents[i].Name == agentName {
					agent = &node.AlertAgents[i]
					break
				}
			}
			if agent == nil || len(agent.Conditions) != 3 {
				return fmt.Errorf("expected three conditions for agent %s", agentName)
			}
			for _, condition := range []string{etcdv1.AlertAgentConfiguredConditionType, etcdv1.AlertAgentAvailableConditionType, etcdv1.AlertAgentHealthyConditionType} {
				want, reason := metav1.ConditionTrue, ""
				for _, failure := range failures {
					if (failure.node == "" || failure.node == name) && failure.agent == agentName && (condition == failure.condition || condition == etcdv1.AlertAgentHealthyConditionType) {
						want = metav1.ConditionFalse
						nodeHealthy = metav1.ConditionFalse
						if condition == failure.condition {
							reason = failure.reason
						} else {
							reason = etcdv1.AlertAgentHealthyReasonUnhealthy
						}
					}
				}
				got := meta.FindStatusCondition(agent.Conditions, condition)
				if got == nil || got.Status != want || (reason != "" && got.Reason != reason) {
					return fmt.Errorf("agent %s: expected %s=%s reason=%q, got %v", agentName, condition, want, reason, got)
				}
			}
		}
		for _, condition := range []string{etcdv1.NodeAlertAgentsHealthyConditionType, etcdv1.NodeHealthyConditionType} {
			if err := apis.ExpectNodeCondition(pc, name, condition, nodeHealthy); err != nil {
				return err
			}
		}
		if nodeHealthy == metav1.ConditionFalse {
			clusterHealthy = metav1.ConditionFalse
		}
	}
	return apis.ExpectClusterCondition(pc, etcdv1.ClusterHealthyConditionType, clusterHealthy)
}

func waitForTNFAlertStatus(ctx context.Context, oc *exutil.CLI, nodes []string, after time.Time, failures ...tnfAlertFailure) error {
	return retryTNFOperation(ctx, 5*time.Second, tnfObservabilityTimeout, func(context.Context) error {
		pc, err := apis.GetPacemakerCluster(oc)
		if err != nil {
			return err
		}
		if !pc.Status.LastUpdated.After(after) {
			return fmt.Errorf("waiting for a new Pacemaker collection")
		}
		return expectTNFAlertStatus(pc, nodes, failures...)
	})
}

type tnfAlertConfig struct {
	Alerts []struct {
		ID     string `json:"id"`
		Path   string `json:"path"`
		Select struct {
			Fencing bool `json:"fencing"`
			Nodes   bool `json:"nodes"`
		} `json:"select"`
	} `json:"alerts"`
}

func expectTNFAlertRegistrations(output string) error {
	var config tnfAlertConfig
	if err := json.Unmarshal([]byte(output), &config); err != nil {
		return fmt.Errorf("invalid pcs alert configuration JSON")
	}
	for id, path := range map[string]string{services.TaintAlertID: services.TaintAlertScriptPath, services.UntaintAlertID: services.UntaintAlertScriptPath} {
		found := false
		for _, alert := range config.Alerts {
			if alert.ID == id {
				found = true
				if alert.Path != path || alert.Select.Fencing != (id == services.TaintAlertID) || alert.Select.Nodes != (id == services.UntaintAlertID) {
					return fmt.Errorf("unexpected path or event selection for %s", id)
				}
			}
		}
		if !found {
			return fmt.Errorf("missing alert %s", id)
		}
	}
	return nil
}

func tnfAlertXPath(id string) string {
	return fmt.Sprintf("/cib/configuration/alerts/alert[@id='%s']", id)
}

func backupTNFAlertRegistration(ctx context.Context, pcs *tnfPCSRunner, node, id string) {
	xpath := tnfAlertXPath(id)
	backup := "/var/tmp/tnf-alert-" + string(uuid.NewUUID())
	g.DeferCleanup(func() {
		err := retryTNFOperation(context.Background(), tnfPollInterval, tnfCommandTimeout, func(ctx context.Context) error {
			_, err := pcs.hostOutput(ctx, node, "bash", "-ceu", `if test -s "$2"; then
	if cibadmin --query --xpath "$1" >/dev/null 2>&1; then
		cibadmin --replace --xpath "$1" --xml-file "$2"
	else
		cibadmin --modify --scope alerts --xml-file "$2"
	fi
fi
rm -f -- "$2"`, "--", xpath, backup)
			return err
		})
		o.Expect(err).To(o.Succeed(), "restore alert registration %s", id)
	})
	// Save the original registration, including its metadata and recipients.
	_, err := pcs.hostOutput(ctx, node, "bash", "-ceu", `umask 077
set -o noclobber
cibadmin --query --xpath "$1" > "$2"`, "--", xpath, backup)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func expectTNFAlertUnregistered(ctx context.Context, pcs *tnfPCSRunner, node, id string) {
	output, err := pcs.output(ctx, node, "alert", "config", "--output-format", "json")
	o.Expect(err).NotTo(o.HaveOccurred())
	var config tnfAlertConfig
	o.Expect(json.Unmarshal([]byte(output), &config)).To(o.Succeed())
	o.Expect(config.Alerts).NotTo(o.BeNil(), "pcs must return an alerts array")
	for _, alert := range config.Alerts {
		o.Expect(alert.ID).NotTo(o.Equal(id), "the collector must leave the injected missing registration unrepaired")
	}
}
