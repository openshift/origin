package watchapiservices

import (
	"context"
	"fmt"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	apiregistrationv1client "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/typed/apiregistration/v1"
	"time"
)

func startAPIServiceMonitoring(ctx context.Context, m monitorapi.RecorderWriter, client *apiregistrationv1client.ApiregistrationV1Client) {
	apiServiceChangeFns := []func(newAPIService, oldAPIService *apiregistrationv1.APIService) []monitorapi.Interval{
		func(newAPIService, oldAPIService *apiregistrationv1.APIService) []monitorapi.Interval {
			var intervals []monitorapi.Interval
			if oldAPIService != nil {
				return intervals
			}
			if newAPIService.Spec.Service == nil {
				return intervals
			}

			intervals = append(intervals,
				monitorapi.NewInterval(monitorapi.SourceAPIServiceMonitor, monitorapi.Info).
					Locator(monitorapi.NewLocator().APIService(newAPIService.Name, newAPIService.Spec.Service.Namespace)).
					Message(monitorapi.NewMessage().Reason(monitorapi.APIServiceCreated).
						HumanMessage("APIService created")).
					Build(newAPIService.ObjectMeta.CreationTimestamp.Time, newAPIService.ObjectMeta.CreationTimestamp.Time))
			return intervals
		},

		func(newAPIService, oldAPIService *apiregistrationv1.APIService) []monitorapi.Interval {
			var intervals []monitorapi.Interval
			now := time.Now()

			oldHasPhase := oldAPIService != nil && findCondition(oldAPIService.Status.Conditions, apiregistrationv1.Available) != nil
			newHasPhase := newAPIService != nil && findCondition(newAPIService.Status.Conditions, apiregistrationv1.Available) != nil
			oldAvailableConditionStatus := "<missing>"
			newAvailableConditionStatus := "<missing>"
			newReason := "<missing>"
			if oldHasPhase {
				condition := findCondition(oldAPIService.Status.Conditions, apiregistrationv1.Available)
				oldAvailableConditionStatus = string(condition.Status)
			}
			if newHasPhase {
				condition := findCondition(newAPIService.Status.Conditions, apiregistrationv1.Available)
				newAvailableConditionStatus = string(condition.Status)
				newReason = condition.Reason
			}

			namespaceName := "<unknown>"
			oldHasService := oldAPIService != nil && oldAPIService.Spec.Service != nil
			newHasService := newAPIService != nil && newAPIService.Spec.Service != nil
			if oldHasService {
				namespaceName = oldAPIService.Spec.Service.Namespace
			}
			if newHasService {
				namespaceName = newAPIService.Spec.Service.Namespace
			}
			if !oldHasService && !newHasService {
				return intervals
			}

			apiServiceName := "<missing>"
			if oldAPIService != nil {
				apiServiceName = oldAPIService.Name
			}
			if newAPIService != nil {
				apiServiceName = newAPIService.Name
			}

			if oldAvailableConditionStatus != newAvailableConditionStatus {
				intervals = append(intervals,
					monitorapi.NewInterval(monitorapi.SourceAPIServiceMonitor, monitorapi.Info).
						Locator(monitorapi.NewLocator().APIService(apiServiceName, namespaceName)).
						Message(monitorapi.NewMessage().Reason(monitorapi.APIServiceConditionChanged).
							WithAnnotation(monitorapi.AnnotationCondition, string(apiregistrationv1.Available)).
							WithAnnotation(monitorapi.AnnotationPreviousStatus, oldAvailableConditionStatus).
							WithAnnotation(monitorapi.AnnotationStatus, newAvailableConditionStatus).
							HumanMessage(fmt.Sprintf(".status.conditions[%q] changed from %s to %s, reason=%s", string(apiregistrationv1.Available), oldAvailableConditionStatus, newAvailableConditionStatus, newReason))).
						Build(now, now))
			}
			return intervals
		},

		func(newAPIService, oldAPIService *apiregistrationv1.APIService) []monitorapi.Interval {
			var intervals []monitorapi.Interval
			if newAPIService != nil {
				return intervals
			}
			if oldAPIService.Spec.Service == nil {
				return intervals
			}

			now := time.Now()
			intervals = append(intervals,
				monitorapi.NewInterval(monitorapi.SourceAPIServiceMonitor, monitorapi.Info).
					Locator(monitorapi.NewLocator().APIService(oldAPIService.Name, oldAPIService.Spec.Service.Namespace)).
					Message(monitorapi.NewMessage().Reason(monitorapi.APIServiceDeletedInAPI).
						HumanMessage("APIService deleted")).
					Build(now, now))
			return intervals
		},
	}

	listWatch := cache.NewListWatchFromClient(client.RESTClient(), "apiservices", "", fields.Everything())
	customStore := monitortestlibrary.NewMonitoringStore(
		"apiservices",
		toCreateFns(apiServiceChangeFns),
		toUpdateFns(apiServiceChangeFns),
		toDeleteFns(apiServiceChangeFns),
		m,
		m,
	)
	reflector := cache.NewReflector(listWatch, &apiregistrationv1.APIService{}, customStore, 0)
	go reflector.Run(ctx.Done())
}

func findCondition(conditions []apiregistrationv1.APIServiceCondition, conditionType apiregistrationv1.APIServiceConditionType) *apiregistrationv1.APIServiceCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}

	return nil
}

func toCreateFns(apiserviceUpdateFns []func(newAPIService, oldAPIService *apiregistrationv1.APIService) []monitorapi.Interval) []monitortestlibrary.ObjCreateFunc {
	ret := []monitortestlibrary.ObjCreateFunc{}

	for i := range apiserviceUpdateFns {
		fn := apiserviceUpdateFns[i]
		ret = append(ret, func(obj interface{}) []monitorapi.Interval {
			return fn(obj.(*apiregistrationv1.APIService), nil)
		})
	}

	return ret
}

func toDeleteFns(apiserviceUpdateFns []func(newAPIService, oldAPIService *apiregistrationv1.APIService) []monitorapi.Interval) []monitortestlibrary.ObjDeleteFunc {
	ret := []monitortestlibrary.ObjDeleteFunc{}

	for i := range apiserviceUpdateFns {
		fn := apiserviceUpdateFns[i]
		ret = append(ret, func(obj interface{}) []monitorapi.Interval {
			return fn(nil, obj.(*apiregistrationv1.APIService))
		})
	}
	return ret
}
func toUpdateFns(apiserviceUpdateFns []func(newAPIService, oldAPIService *apiregistrationv1.APIService) []monitorapi.Interval) []monitortestlibrary.ObjUpdateFunc {
	ret := []monitortestlibrary.ObjUpdateFunc{}

	for i := range apiserviceUpdateFns {
		fn := apiserviceUpdateFns[i]
		ret = append(ret, func(obj, oldObj interface{}) []monitorapi.Interval {
			if oldObj == nil {
				return fn(obj.(*apiregistrationv1.APIService), nil)
			}
			return fn(obj.(*apiregistrationv1.APIService), oldObj.(*apiregistrationv1.APIService))
		})
	}

	return ret
}
