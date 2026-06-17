package clusterinstancetypes

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	machineclient "github.com/openshift/client-go/machine/clientset/versioned"
	"github.com/openshift/origin/pkg/dataloader"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type clusterInstanceTypes struct {
	adminRESTConfig *rest.Config
	data            []instanceTypeRow
}

type instanceTypeRow struct {
	Platform     string `json:"platform"`
	Region       string `json:"region"`
	Role         string `json:"role"`
	InstanceType string `json:"instance_type"`
}

func NewClusterInstanceTypes() monitortestframework.MonitorTest {
	return &clusterInstanceTypes{}
}

func (w *clusterInstanceTypes) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *clusterInstanceTypes) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig
	return nil
}

func (w *clusterInstanceTypes) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	logger := logrus.WithField("MonitorTest", "ClusterInstanceTypes")

	data, err := w.collect(ctx)
	if err != nil {
		logger.WithError(err).Warn("failed to collect instance type data")
		return nil, nil, nil
	}
	w.data = data
	return nil, nil, nil
}

func (*clusterInstanceTypes) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (*clusterInstanceTypes) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (w *clusterInstanceTypes) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	if len(w.data) == 0 {
		return nil
	}

	rows := make([]map[string]string, 0, len(w.data))
	for _, r := range w.data {
		rows = append(rows, map[string]string{
			"Platform":     r.Platform,
			"Region":       r.Region,
			"Role":         r.Role,
			"InstanceType": r.InstanceType,
		})
	}

	dataFile := dataloader.DataFile{
		TableName: "cluster_instance_types",
		Schema: map[string]dataloader.DataType{
			"Platform":     dataloader.DataTypeString,
			"Region":       dataloader.DataTypeString,
			"Role":         dataloader.DataTypeString,
			"InstanceType": dataloader.DataTypeString,
		},
		Rows: rows,
	}

	fileName := filepath.Join(storageDir, fmt.Sprintf("cluster-instance-types%s-%s", timeSuffix, dataloader.AutoDataLoaderSuffix))
	if err := dataloader.WriteDataFile(fileName, dataFile); err != nil {
		return fmt.Errorf("failed to write instance types autodl: %w", err)
	}

	return nil
}

func (*clusterInstanceTypes) Cleanup(ctx context.Context) error {
	return nil
}

func (w *clusterInstanceTypes) collect(ctx context.Context) ([]instanceTypeRow, error) {
	configClient, err := configclient.NewForConfig(w.adminRESTConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create config client: %w", err)
	}

	infra, err := configClient.ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get infrastructure: %w", err)
	}

	if infra.Status.PlatformStatus == nil {
		logrus.Info("skipping instance type collection: platform status not set")
		return nil, nil
	}

	platform := strings.ToLower(string(infra.Status.PlatformStatus.Type))
	if platform != "aws" && platform != "azure" && platform != "gcp" {
		logrus.WithField("platform", platform).Info("skipping instance type collection for unsupported platform")
		return nil, nil
	}

	// Azure doesn't expose region in Infrastructure CR, so we always fall back to node labels
	region := getRegionFromInfrastructure(infra)
	if region == "" {
		kubeClient, err := kubernetes.NewForConfig(w.adminRESTConfig)
		if err != nil {
			logrus.WithError(err).Warn("failed to create kube client for region fallback")
		} else {
			nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			if err != nil {
				logrus.WithError(err).Warn("failed to list nodes for region fallback")
			} else if len(nodes.Items) > 0 {
				region = nodes.Items[0].Labels["topology.kubernetes.io/region"]
			}
		}
	}

	machineClientSet, err := machineclient.NewForConfig(w.adminRESTConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create machine client: %w", err)
	}

	machines, err := machineClientSet.MachineV1beta1().Machines("openshift-machine-api").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list machines: %w", err)
	}

	return buildRows(platform, region, machines.Items), nil
}

func buildRows(platform, region string, machines []machinev1beta1.Machine) []instanceTypeRow {
	seen := map[string]bool{}
	var result []instanceTypeRow

	for i := range machines {
		machine := &machines[i]
		role := "worker"
		if isMaster(machine) {
			role = "control-plane"
		}
		instanceType := extractInstanceType(platform, machine)
		if instanceType == "" {
			continue
		}
		key := role + "/" + instanceType
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, instanceTypeRow{
			Platform:     platform,
			Region:       region,
			Role:         role,
			InstanceType: instanceType,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Role != result[j].Role {
			return result[i].Role < result[j].Role
		}
		return result[i].InstanceType < result[j].InstanceType
	})

	return result
}

func getRegionFromInfrastructure(infra *configv1.Infrastructure) string {
	if infra.Status.PlatformStatus == nil {
		return ""
	}
	switch infra.Status.PlatformStatus.Type {
	case configv1.AWSPlatformType:
		if infra.Status.PlatformStatus.AWS != nil {
			return infra.Status.PlatformStatus.AWS.Region
		}
	case configv1.GCPPlatformType:
		if infra.Status.PlatformStatus.GCP != nil {
			return infra.Status.PlatformStatus.GCP.Region
		}
	}
	return ""
}

func isMaster(machine *machinev1beta1.Machine) bool {
	return machine.Labels["machine.openshift.io/cluster-api-machine-role"] == "master"
}

func extractInstanceType(platform string, machine *machinev1beta1.Machine) string {
	if machine.Spec.ProviderSpec.Value == nil {
		return ""
	}
	raw := machine.Spec.ProviderSpec.Value.Raw

	switch platform {
	case "aws":
		var spec machinev1beta1.AWSMachineProviderConfig
		if err := json.Unmarshal(raw, &spec); err != nil {
			logrus.WithError(err).WithField("machine", machine.Name).Warn("failed to unmarshal AWS provider spec")
			return ""
		}
		return spec.InstanceType
	case "azure":
		var spec machinev1beta1.AzureMachineProviderSpec
		if err := json.Unmarshal(raw, &spec); err != nil {
			logrus.WithError(err).WithField("machine", machine.Name).Warn("failed to unmarshal Azure provider spec")
			return ""
		}
		return spec.VMSize
	case "gcp":
		var spec machinev1beta1.GCPMachineProviderSpec
		if err := json.Unmarshal(raw, &spec); err != nil {
			logrus.WithError(err).WithField("machine", machine.Name).Warn("failed to unmarshal GCP provider spec")
			return ""
		}
		return spec.MachineType
	}
	return ""
}
