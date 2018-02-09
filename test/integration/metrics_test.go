package integration

import (
	"regexp"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

var metricsRegexp = regexp.MustCompile("(?m)^# HELP ([^ ]*)")

func TestMetrics(t *testing.T) {
	expectedMetrics := []string{
		"openshift_template_instance_total",
	}

	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatal(err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}

	var missingMetrics []string
	err = wait.Poll(time.Second, 30*time.Second, func() (bool, error) {
		missingMetrics = []string{}

		b, err := clusterAdminClient.Discovery().RESTClient().Get().RequestURI("/metrics").DoRaw()
		if err != nil {
			return false, err
		}

		metrics := map[string]struct{}{}
		for _, match := range metricsRegexp.FindAllStringSubmatch(string(b), -1) {
			metrics[match[1]] = struct{}{}
		}

		for _, metric := range expectedMetrics {
			if _, ok := metrics[metric]; !ok {
				missingMetrics = append(missingMetrics, metric)
			}
		}

		return len(missingMetrics) == 0, nil
	})
	if len(missingMetrics) > 0 {
		t.Error(missingMetrics)
	}
	if err != nil {
		t.Error(err)
	}
}
