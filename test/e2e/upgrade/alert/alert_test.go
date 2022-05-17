package alert

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"
	helper "github.com/openshift/origin/test/extended/util/prometheus"
)

func getClient() *exutil.CLI {
	home_dir := os.Getenv("HOME")
	os.Setenv("KUBECONFIG", fmt.Sprintf("%s/Downloads/cluster-bot-2022-05-17-155322.kubeconfig.txt", home_dir))
	oc := exutil.NewCLIWithoutNamespace("default")
	return oc
}

// Test_redhatOperatorPodsNotPending is a simple unit test to ensure the are no panics.
func Test_redhatOperatorPodsNotPending(t *testing.T) {
	oc := getClient()
	now := time.Now()
	val := redhatOperatorPodsNotPending(now, oc.AdminKubeClient())
	fmt.Printf("Any true of false value is ok: val=%v\n", val)
}

// Test_firedDueToImagePullBackoff is a simple unit test to ensure the are no panics.
func Test_firedDueToImagePullBackoff(t *testing.T) {
	oc := getClient()
	prometheusClient := oc.NewPrometheusClient(context.TODO())

	start := time.Now()
	time.Sleep(2 * time.Second)
	testDuration := time.Now().Sub(start).Round(time.Second)
	firingAlertQuery := fmt.Sprintf(`
	sort_desc(
	count_over_time(ALERTS{alertstate="firing",severity!="info",alertname!~"Watchdog|AlertmanagerReceiversNotConfigured"}[%[1]s:1s])
	) > 0
	`, testDuration)
	_, err := helper.RunQuery(context.TODO(), prometheusClient, firingAlertQuery)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		t.Error("Failed to do Prometheus query")
	}
	testData := []string{
		`{namespace="openshift-marketplace", pod="redhat-operators-65mdd", severity="warning"}`,
		`{namespace="openshift-cluster-csi-drivers", pod="aws-ebs-csi-driver-node-zrnpk", severity="warning"}`,
		`{namespace="kube-system", pod="bootstrap-kube-controller-manager-ip-10-0-78-205", severity="warning"}`,
	}
	for _, data := range testData {
		val := kPNRDueToImagePullBackoff(data, start, oc.AdminKubeClient())
		fmt.Printf("Any true of false value is ok: val=%v\n", val)
	}
}
