package watchrequestcountscollector

import (
	"context"
	"fmt"
	apiserverclientv1 "github.com/openshift/client-go/apiserver/clientset/versioned/typed/apiserver/v1"
	"github.com/openshift/origin/pkg/dataloader"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type watchRequestCountSerializer struct {
	monitorStartTime time.Time
	adminRESTConfig  *rest.Config
}

func NewWatchRequestCountSerializer() monitortestframework.MonitorTest {
	return &watchRequestCountSerializer{}
}

func (w *watchRequestCountSerializer) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *watchRequestCountSerializer) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.monitorStartTime = time.Now()
	w.adminRESTConfig = adminRESTConfig
	return nil
}

func (w *watchRequestCountSerializer) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, nil
}

func (w *watchRequestCountSerializer) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (constructedIntervals monitorapi.Intervals, err error) {
	return nil, nil
}

func (w *watchRequestCountSerializer) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (w *watchRequestCountSerializer) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	oc := exutil.NewCLIWithoutNamespace("api-requests")

	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		logrus.WithError(err).Warn("unable to get cluster infrastructure")
		return nil
	}

	watchRequestCounts, err := GetWatchRequestCounts(ctx, oc)
	if err != nil {
		logrus.WithError(err).Warn("unable to get watch request counts")
		return nil
	}

	// infra.Status.ControlPlaneTopology, infra.Spec.PlatformSpec.Type, operator, value
	rows := make([]map[string]string, 0)
	for _, item := range watchRequestCounts {
		operator := strings.Split(item.Operator, ":")[3]
		rows = append(rows, map[string]string{"ControlPlaneTopology": string(infra.Status.ControlPlaneTopology), "PlatformType": string(infra.Spec.PlatformSpec.Type), "Operator": operator, "WatchRequestCount": strconv.FormatInt(item.Count, 10)})
	}

	dataFile := dataloader.DataFile{
		TableName: "operator_watch_requests",
		Schema:    map[string]dataloader.DataType{"ControlPlaneTopology": dataloader.DataTypeString, "PlatformType": dataloader.DataTypeString, "Operator": dataloader.DataTypeString, "WatchRequestCount": dataloader.DataTypeInteger},
		Rows:      rows,
	}
	fileName := filepath.Join(storageDir, fmt.Sprintf("operator-watch-requests%s-%s", timeSuffix, dataloader.AutoDataLoaderSuffix))
	err = dataloader.WriteDataFile(fileName, dataFile)
	if err != nil {
		logrus.WithError(err).Warnf("unable to write data file: %s", fileName)
		return nil
	}

	return nil
}

func (w *watchRequestCountSerializer) Cleanup(ctx context.Context) error {
	return nil
}

type OperatorKey struct {
	NodeName string
	Operator string
	Hour     int
}

type RequestCount struct {
	NodeName string
	Operator string
	Count    int64
	Hour     int
}

func GetWatchRequestCounts(ctx context.Context, oc *exutil.CLI) ([]*RequestCount, error) {

	apirequestCountClient, err := apiserverclientv1.NewForConfig(oc.AdminConfig())
	if err != nil {
		logrus.WithError(err).Warn("unable to initialize apirequestCountClient")
		return nil, err
	}

	apiRequestCounts, err := apirequestCountClient.APIRequestCounts().List(ctx, metav1.ListOptions{})

	watchRequestCounts := []*RequestCount{}
	watchRequestCountsMap := map[OperatorKey]*RequestCount{}

	for _, apiRequestCount := range apiRequestCounts.Items {
		if apiRequestCount.Status.RequestCount <= 0 {
			continue
		}
		for hourIdx, perResourceAPIRequestLog := range apiRequestCount.Status.Last24h {
			if perResourceAPIRequestLog.RequestCount > 0 {
				for _, perNodeCount := range perResourceAPIRequestLog.ByNode {
					if perNodeCount.RequestCount <= 0 {
						continue
					}
					for _, perUserCount := range perNodeCount.ByUser {
						if perUserCount.RequestCount <= 0 {
							continue
						}
						// take only operators into account
						if !strings.HasSuffix(perUserCount.UserName, "-operator") {
							continue
						}
						for _, verb := range perUserCount.ByVerb {
							if verb.Verb != "watch" || verb.RequestCount == 0 {
								continue
							}
							key := OperatorKey{
								NodeName: perNodeCount.NodeName,
								Operator: perUserCount.UserName,
								Hour:     hourIdx,
							}
							// group requests by a resource (the number of watchers in the code does not change
							// so much as the number of requests)
							if _, exists := watchRequestCountsMap[key]; exists {
								watchRequestCountsMap[key].Count += verb.RequestCount
							} else {
								watchRequestCountsMap[key] = &RequestCount{
									NodeName: perNodeCount.NodeName,
									Operator: perUserCount.UserName,
									Count:    verb.RequestCount,
									Hour:     hourIdx,
								}
							}
						}
					}
				}
			}
		}
	}

	// take maximum from all hours through all nodes
	watchRequestCountsMapMax := map[OperatorKey]*RequestCount{}
	for _, requestCount := range watchRequestCountsMap {
		key := OperatorKey{
			Operator: requestCount.Operator,
		}
		if _, exists := watchRequestCountsMapMax[key]; exists {
			if watchRequestCountsMapMax[key].Count < requestCount.Count {
				watchRequestCountsMapMax[key].Count = requestCount.Count
				watchRequestCountsMapMax[key].NodeName = requestCount.NodeName
				watchRequestCountsMapMax[key].Hour = requestCount.Hour
			}
		} else {
			watchRequestCountsMapMax[key] = requestCount
		}
	}

	// sort the requsts counts so it's easy to see the biggest offenders
	for _, requestCount := range watchRequestCountsMapMax {
		watchRequestCounts = append(watchRequestCounts, requestCount)
	}

	sort.Slice(watchRequestCounts, func(i int, j int) bool {
		return watchRequestCounts[i].Count > watchRequestCounts[j].Count
	})

	return watchRequestCounts, nil
}
