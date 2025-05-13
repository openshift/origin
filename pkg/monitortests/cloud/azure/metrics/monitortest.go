package azuremetricsanalyzer

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/monitor/query/azlogs"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/operationalinsights/armoperationalinsights"
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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/cloud-provider-azure/pkg/provider"
	"sigs.k8s.io/yaml"
)

const (
	// avgOSDiskQueueDepthThreshold defines the threshold for average OS Disk Queue Depth metric.
	// If the metric average is over the threshold, an interval will be created. This can be adjusted
	// based on value collected in real test environment.
	avgOSDiskQueueDepthThreshold = 3.0
	lbAvailabilityThreshold      = 99
)

type azureMetricsCollector struct {
	adminRESTConfig    *rest.Config
	flakeErr           error
	notSupportedReason error
	resourceGroup      string
	subscriptionID     string
	workspaceName      string
	workspaceID        string
	credential         *azidentity.DefaultAzureCredential
}

// metricTest is used to group test data such as azure metrics query params and threshold
type metricTest struct {
	interval       string
	upperThreshold float64
	lowerThreshold float64
}

func NewAzureMetricsCollector() monitortestframework.MonitorTest {
	return &azureMetricsCollector{}
}

func createLogAnalyticsWorkspace(ctx context.Context, credential *azidentity.DefaultAzureCredential, subscriptionID, resourceGroup, workspaceName, location string) (string, error) {
	client, err := armoperationalinsights.NewWorkspacesClient(subscriptionID, credential, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create operational insights client: %v", err)
	}

	sku := armoperationalinsights.WorkspaceSKUNameEnumStandalone
	pollerResponse, err := client.BeginCreateOrUpdate(ctx, resourceGroup, workspaceName, armoperationalinsights.Workspace{
		Location: &location,
		Properties: &armoperationalinsights.WorkspaceProperties{
			SKU: &armoperationalinsights.WorkspaceSKU{
				Name: &sku,
			},
			RetentionInDays: nil, // You can specify retention days if needed
		},
	}, nil)
	if err != nil {
		return "", fmt.Errorf("failed to start workspace creation: %v", err)
	}

	// Wait for the operation to complete
	response, err := pollerResponse.PollUntilDone(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to complete workspace creation: %v", err)
	}

	logrus.Infof("Workspace %s created; ID: %s", *response.Workspace.Name, *response.Workspace.Properties.CustomerID)
	return *response.Workspace.Properties.CustomerID, nil
}

func deleteLogAnalyticsWorkspace(ctx context.Context, credential *azidentity.DefaultAzureCredential, subscriptionID, resourceGroup, workspaceName string) error {
	client, err := armoperationalinsights.NewWorkspacesClient(subscriptionID, credential, nil)
	if err != nil {
		return fmt.Errorf("failed to create operational insights client: %v", err)
	}

	pollerResponse, err := client.BeginDelete(ctx, resourceGroup, workspaceName, nil)
	if err != nil {
		return fmt.Errorf("failed to start workspace deletion: %v", err)
	}

	_, err = pollerResponse.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to complete workspace deletion: %v", err)
	}

	logrus.Infof("Workspace deleted successfully.")
	return nil
}

func getLoadBalancerID(ctx context.Context, credential *azidentity.DefaultAzureCredential, subscriptionID, resourceGroup, loadBalancerName string) (string, error) {
	client, err := armnetwork.NewLoadBalancersClient(subscriptionID, credential, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create load balancers client: %v", err)
	}

	// Retrieve the load balancer
	lb, err := client.Get(ctx, resourceGroup, loadBalancerName, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get load balancer: %v", err)
	}

	// Return the ARM ID
	return *lb.ID, nil
}

func configureDiagnosticSettings(ctx context.Context, credential *azidentity.DefaultAzureCredential, loadBalancerID, workspaceResourceID string) error {
	client, err := armmonitor.NewDiagnosticSettingsClient(credential, nil)
	if err != nil {
		return fmt.Errorf("failed to create diagnostic settings client: %v", err)
	}

	// Create diagnostic settings
	settingsName := "LoadBalancerDiagnosticSettings"
	_, err = client.CreateOrUpdate(ctx, loadBalancerID, settingsName, armmonitor.DiagnosticSettingsResource{
		Properties: &armmonitor.DiagnosticSettings{
			WorkspaceID: &workspaceResourceID,
			Logs: []*armmonitor.LogSettings{
				{
					Category: to.Ptr("LoadBalancerHealthEvent"),
					Enabled:  to.Ptr(true),
				},
			},
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to create diagnostic settings: %v", err)
	}

	return nil
}

func (w *azureMetricsCollector) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig
	kubeClient, err := kubernetes.NewForConfig(w.adminRESTConfig)
	if err != nil {
		return err
	}
	isMicroShift, err := exutil.IsMicroShiftCluster(kubeClient)
	if err != nil {
		return fmt.Errorf("unable to determine if cluster is MicroShift: %v", err)
	}
	if isMicroShift {
		w.notSupportedReason = &monitortestframework.NotSupportedError{
			Reason: "platform MicroShift not supported",
		}
		return w.notSupportedReason
	}

	// Only collect if we are on azure
	oc := exutil.NewCLI("cloudmetrics").AsAdmin()
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return err
	}
	if infra.Spec.PlatformSpec.Type != configv1.AzurePlatformType {
		reason := fmt.Sprintf("platform %s not supported", infra.Spec.PlatformSpec.Type)
		w.notSupportedReason = &monitortestframework.NotSupportedError{
			Reason: reason,
		}
		return w.notSupportedReason
	}
	// get resource group
	w.resourceGroup = infra.Status.PlatformStatus.Azure.ResourceGroupName

	// get subscription ID
	cm, err := oc.KubeClient().CoreV1().ConfigMaps("openshift-config").Get(context.Background(), "cloud-provider-config", metav1.GetOptions{})
	if err != nil {
		return err
	}
	data, ok := cm.Data["config"]
	if !ok {
		return fmt.Errorf("No cloud provider config was set in openshift-config/cloud-provider-config")
	}
	config := &provider.Config{}
	if err := yaml.Unmarshal([]byte(data), config); err != nil {
		return err
	}
	w.subscriptionID = config.SubscriptionID

	azureutil.ExportAzureCredentials()

	// create azure metrics client
	w.credential, err = azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		logrus.WithError(err).Error("default azure credential does not exist")
		// we do not want to fail this because of missing azure credentials
		w.flakeErr = &monitortestframework.FlakeError{Err: err}
		return w.flakeErr
	}

	// randomize name
	w.workspaceName = infra.Status.InfrastructureName
	w.workspaceID, err = createLogAnalyticsWorkspace(ctx, w.credential, w.subscriptionID, w.resourceGroup, w.workspaceName, config.Location)
	if err != nil {
		logrus.WithError(err).Error("failed to create log-analytics workspace")
		// we do not want to fail this because of azure tooling failures
		w.flakeErr = &monitortestframework.FlakeError{Err: err}
		return w.flakeErr
	}
	workspaceResourceID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.OperationalInsights/workspaces/%s", w.subscriptionID, w.resourceGroup, w.workspaceName)
	loadBalancerName := fmt.Sprintf("%s-internal", infra.Status.InfrastructureName)
	loadBalancerID, err := getLoadBalancerID(ctx, w.credential, w.subscriptionID, w.resourceGroup, loadBalancerName)
	logrus.Infof("got load balancer ID %s", loadBalancerID)
	if err != nil {
		logrus.WithError(err).Error("failed to get load balancer ID")
		// we do not want to fail this because of azure tooling failures
		w.flakeErr = &monitortestframework.FlakeError{Err: err}
		return w.flakeErr
	}
	err = configureDiagnosticSettings(ctx, w.credential, loadBalancerID, workspaceResourceID)
	if err != nil {
		logrus.WithError(err).Error("failed to configure diagnostic settings")
		// we do not want to fail this because of azure tooling failures
		w.flakeErr = &monitortestframework.FlakeError{Err: err}
		return w.flakeErr
	}
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
			interval:       "PT1M",
			upperThreshold: avgOSDiskQueueDepthThreshold,
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
						if d.Average != nil && *d.Average > test.upperThreshold {
							message := fmt.Sprintf("Average value of %.2f for metric %s is over the threshold of %.2f", *d.Average, metric, test.upperThreshold)
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

func fetchLBMetrics(ctx context.Context, client *armmonitor.MetricsClient, subscriptionID, resourceGroup, lbName string, startTime time.Time) ([]monitorapi.Interval, error) {
	ret := monitorapi.Intervals{}
	// metricsMap maps a metric name to test parameters. It includes query parameter such as metric intervals.
	// It also includes thresholds that will be compared with the time series instances.
	metricsMap := map[string]metricTest{
		"VipAvailability": {
			interval:       "PT1M",
			lowerThreshold: lbAvailabilityThreshold,
		},
		"DipAvailability": {
			interval:       "PT1M",
			lowerThreshold: lbAvailabilityThreshold,
		},
	}

	logrus.Infof("Fetching load balancer metrics")
	for metric, test := range metricsMap {
		resourceID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/loadBalancers/%s", subscriptionID, resourceGroup, lbName)
		// Specify the time range and interval to query
		timeRange := fmt.Sprintf("PT%dH", int(time.Now().Sub(startTime).Hours())+3)
		resp, err := client.List(ctx, resourceID, &armmonitor.MetricsClientListOptions{
			Timespan:        &timeRange,
			Interval:        &test.interval,
			Metricnames:     &metric,
			Metricnamespace: nil,
		})
		if err != nil {
			logrus.WithError(err).Error("error getting load balancer metrics")
			return nil, err
		}
		for _, value := range resp.Value {
			for _, ts := range value.Timeseries {
				for _, d := range ts.Data {
					if d.Average != nil && *d.Average < test.lowerThreshold {
						message := fmt.Sprintf("Average value of %.2f for metric %s is under the threshold of %.2f", *d.Average, metric, test.lowerThreshold)
						ret = append(ret, monitorapi.NewInterval(monitorapi.SourceCloudMetrics, monitorapi.Warning).
							Locator(monitorapi.NewLocator().CloudNodeMetric(lbName, metric)).
							Message(monitorapi.NewMessage().Reason(monitorapi.CloudMetricsLBAvailability).HumanMessage(message)).
							Display().
							Build(d.TimeStamp.Add(-1*time.Minute), *d.TimeStamp),
						)
					}
				}
			}
		}
	}
	return ret, nil
}

func fetchLBLogs(ctx context.Context, credential *azidentity.DefaultAzureCredential, workspaceID string, startTime time.Time) ([]monitorapi.Interval, error) {
	logrus.Infof("fetch load balancer health events for workspace %s", workspaceID)
	ret := monitorapi.Intervals{}
	client, err := azlogs.NewClient(credential, nil)
	if err != nil {
		return ret, fmt.Errorf("failed to create log query client: %v", err)
	}

	query := `
		ALBHealthEvent
		| project TimeGenerated, operationName, HealthEventType, Severity, LoadBalancerResourceId, Description, FrontendIP, SourceSystem, Type
		| order by TimeGenerated desc
	`
	response, err := client.QueryWorkspace(ctx, workspaceID, azlogs.QueryBody{
		Query:    &query,
		Timespan: to.Ptr(azlogs.NewTimeInterval(startTime, time.Now())),
	}, nil)
	if err != nil {
		return ret, fmt.Errorf("failed to query workspace: %v", err)
	}

	for _, row := range response.Tables[0].Rows {
		timestamp, ok := row[0].(time.Time)
		if !ok {
			logrus.Warningf("error converting timestamp for row %+v", row)
			continue
		}
		operationName := row[1]
		eventType := row[2]
		severity := row[3]
		lbResourceID := row[4]
		description := row[5]
		if severity == "Critical" || severity == "Error" || severity == "Warning" {
			message := fmt.Sprintf("Load Balancer Health Event for %s: event type: %s severity: %s operation: %s with description: %s", lbResourceID, eventType, severity, operationName, description)
			ret = append(ret, monitorapi.NewInterval(monitorapi.SourceCloudMetrics, monitorapi.Warning).
				Locator(monitorapi.NewLocator().NodeFromName(lbResourceID.(string))).
				Message(monitorapi.NewMessage().Reason(monitorapi.CloudMetricsLBHealthEvent).HumanMessage(message)).
				Display().
				Build(timestamp.Add(-1*time.Minute), timestamp),
			)
		}
	}

	return []monitorapi.Interval{}, nil
}

// CollectData collects azure metrics. Since azure metrics are collected to facilitate debugging, some errors (like cloud throttling) are not considered fatal.
// We will simply log the error and return nil to the caller.
func (w *azureMetricsCollector) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if w.notSupportedReason != nil {
		return nil, nil, w.notSupportedReason
	}
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
	config := &provider.Config{}
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

	// get LB Name
	lbName := fmt.Sprintf("%s-internal", infra.Status.InfrastructureName)
	intervals, err = fetchLBMetrics(ctx, client, subscriptionID, resourceGroup, lbName, beginning)
	if err != nil {
		logrus.WithError(err).Error("failed to fetch azure load balancer metrics")
		w.flakeErr = &monitortestframework.FlakeError{Err: err}
		return nil, nil, w.flakeErr
	}
	ret = append(ret, intervals...)

	intervals, err = fetchLBLogs(ctx, w.credential, w.workspaceID, beginning)
	if err != nil {
		logrus.WithError(err).Error("failed to fetch azure load balancer logs")
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

func (w *azureMetricsCollector) Cleanup(ctx context.Context) error {
	if w.notSupportedReason != nil {
		return w.notSupportedReason
	}
	return deleteLogAnalyticsWorkspace(ctx, w.credential, w.subscriptionID, w.resourceGroup, w.workspaceName)
}
