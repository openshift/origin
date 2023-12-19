package azuremetricsanalyzer

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"
	azureutil "github.com/openshift/origin/test/extended/util/azure"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/objx"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/legacy-cloud-providers/azure"
	"sigs.k8s.io/yaml"
)

const (
	// avgOSDiskQueueDepthThreshold defines the threshold for average OS Disk Queue Depth metric.
	// If the metric average is over the threshold, an interval will be created. This can be adjusted
	// based on value collected in real test environment.
	avgOSDiskQueueDepthThreshold = 3.0
)

type azureMetricsCollector struct {
	adminRESTConfig *rest.Config
	flakeErr        error
}

// metricTest is used to group test data such as azure metrics query params and threshold
type metricTest struct {
	interval     string
	avgThreshold float64
}

func NewAzureMetricsCollector() monitortestframework.MonitorTest {
	return &azureMetricsCollector{}
}

func (w *azureMetricsCollector) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig
	return nil
}

func objects(from *objx.Value) []objx.Map {
	var values []objx.Map
	switch {
	case from.IsObjxMapSlice():
		return from.ObjxMapSlice()
	case from.IsInterSlice():
		for _, i := range from.InterSlice() {
			if msi, ok := i.(map[string]interface{}); ok {
				values = append(values, objx.Map(msi))
			}
		}
	}
	return values
}

func getAllVMs(ctx context.Context, oc *exutil.CLI) ([]string, error) {
	allVMs := []string{}
	machineClient := oc.AdminDynamicClient().Resource(schema.GroupVersionResource{Group: "machine.openshift.io", Resource: "machines", Version: "v1beta1"})
	obj, err := machineClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	machineList := objx.Map(obj.UnstructuredContent())
	items := objects(machineList.Get("items"))
	for _, machine := range items {
		machineName := machine.Get("metadata.name").String()
		allVMs = append(allVMs, machineName)
	}
	return allVMs, nil
}

func fetchExtrenuousVMMetrics(ctx context.Context, oc *exutil.CLI, client *armmonitor.MetricsClient, subscriptionID, resourceGroup string, startTime time.Time) ([]monitorapi.Interval, error) {
	allVMs, err := getAllVMs(ctx, oc)
	if err != nil {
		return nil, err
	}
	return fetchExtrenuousMetrics(ctx, allVMs, client, subscriptionID, resourceGroup, startTime)
}

func fetchExtrenuousMetrics(ctx context.Context, allVMs []string, client *armmonitor.MetricsClient, subscriptionID, resourceGroup string, startTime time.Time) ([]monitorapi.Interval, error) {
	ret := monitorapi.Intervals{}
	// metricsMap maps a metric name to test parameters. It includes query parameter such as metric intervals.
	// It also includes thresholds that will be compared with the time series instances.
	metricsMap := map[string]metricTest{
		"OS Disk Queue Depth": {
			interval:     "PT1M",
			avgThreshold: avgOSDiskQueueDepthThreshold,
		},
	}

	for _, machineName := range allVMs {
		resourceID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/virtualMachines/%s", subscriptionID, resourceGroup, machineName)
		// Specify the time range and interval to query
		timeRange := fmt.Sprintf("PT%dH", int(time.Now().Sub(startTime).Hours())+1)

		for metric, test := range metricsMap {
			resp, err := client.List(ctx, resourceID, &armmonitor.MetricsClientListOptions{
				Timespan:        &timeRange,
				Interval:        &test.interval,
				Metricnames:     &metric,
				Metricnamespace: nil,
			})
			if err != nil {
				logrus.WithError(err).Error("error getting metrics")
				return nil, err
			}
			for _, value := range resp.Value {
				for _, ts := range value.Timeseries {
					for _, d := range ts.Data {
						if d.Average != nil && *d.Average > test.avgThreshold {
							message := fmt.Sprintf("Average value of %.2f for metric %s is over the threshold of %.2f", *d.Average, metric, test.avgThreshold)
							ret = append(ret, monitorapi.NewInterval(monitorapi.SourceCloudMetrics, monitorapi.Warning).
								Locator(monitorapi.NewLocator().CloudNodeMetric(machineName, metric)).
								Message(monitorapi.NewMessage().Reason(monitorapi.CloudMetricsExtrenuous).HumanMessage(message)).
								Display().
								Build(d.TimeStamp.Add(-1*time.Minute), *d.TimeStamp),
							)
						}
					}
				}
			}
		}
	}
	return ret, nil
}

// CollectData collects azure metrics. Since azure metrics are collected to facilitate debugging, some errors (like cloud throttling) are not considered fatal.
// We will simply log the error and return nil to the caller.
func (w *azureMetricsCollector) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	// Only collect if we are on azure
	oc := exutil.NewCLI("cloudmetrics").AsAdmin()
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	if infra.Spec.PlatformSpec.Type != configv1.AzurePlatformType {
		return nil, nil, nil
	}
	ret := monitorapi.Intervals{}
	// get resource group
	resourceGroup := infra.Status.PlatformStatus.Azure.ResourceGroupName

	// get subscription ID
	cm, err := oc.KubeClient().CoreV1().ConfigMaps("openshift-config").Get(context.Background(), "cloud-provider-config", metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	data, ok := cm.Data["config"]
	if !ok {
		return nil, nil, fmt.Errorf("No cloud provider config was set in openshift-config/cloud-provider-config")
	}
	config := &azure.Config{}
	if err := yaml.Unmarshal([]byte(data), config); err != nil {
		return nil, nil, err
	}
	subscriptionID := config.SubscriptionID

	azureutil.ExportAzureCredentials()

	// create azure metrics client
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		logrus.WithError(err).Error("default azure credential does not exist")
		// we do not want to fail this because of missing azure credentials
		w.flakeErr = &monitortestframework.FlakeError{Err: err}
		return nil, nil, w.flakeErr
	}
	clientFactory, err := armmonitor.NewClientFactory(subscriptionID, cred, nil)
	if err != nil {
		logrus.WithError(err).Error("failed to create azure metric client")
		w.flakeErr = &monitortestframework.FlakeError{Err: err}
		return nil, nil, w.flakeErr
	}
	client := clientFactory.NewMetricsClient()

	intervals, err := fetchExtrenuousVMMetrics(ctx, oc, client, subscriptionID, resourceGroup, beginning)
	if err != nil {
		logrus.WithError(err).Error("failed to fetch azure metrics")
		w.flakeErr = &monitortestframework.FlakeError{Err: err}
		return nil, nil, w.flakeErr
	}
	ret = append(ret, intervals...)

	return ret, nil, nil
}

func (*azureMetricsCollector) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (*azureMetricsCollector) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (*azureMetricsCollector) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*azureMetricsCollector) Cleanup(ctx context.Context) error {
	return nil
}
