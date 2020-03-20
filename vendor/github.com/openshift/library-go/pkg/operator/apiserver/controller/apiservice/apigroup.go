package apiservice

import (
	"context"
	"fmt"
	"net/http"

	"github.com/openshift/library-go/pkg/operator/events"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/rest"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

func newEndpointPrecondition(kubeInformers kubeinformers.SharedInformerFactory) func(apiServices []*apiregistrationv1.APIService) (bool, error) {
	// this is outside the func so it always registers before the informers start
	endpointsLister := kubeInformers.Core().V1().Endpoints().Lister()

	type coordinate struct {
		namespace string
		name      string
	}

	return func(apiServices []*apiregistrationv1.APIService) (bool, error) {

		coordinates := []coordinate{}
		for _, apiService := range apiServices {
			curr := coordinate{namespace: apiService.Spec.Service.Namespace, name: apiService.Spec.Service.Name}
			exists := false
			for _, j := range coordinates {
				if j == curr {
					exists = true
					break
				}
			}
			if !exists {
				coordinates = append(coordinates, curr)
			}
		}

		for _, curr := range coordinates {
			endpoints, err := endpointsLister.Endpoints(curr.namespace).Get(curr.name)
			if err != nil {
				return false, err
			}
			if len(endpoints.Subsets) == 0 {
				return false, nil
			}

			exists := false
			for _, subset := range endpoints.Subsets {
				if len(subset.Addresses) > 0 {
					exists = true
					break
				}
			}
			if !exists {
				return false, nil
			}
		}

		return true, nil
	}
}

func checkDiscoveryForByAPIServices(recorder events.Recorder, restclient rest.Interface, apiServices []*apiregistrationv1.APIService) []string {
	missingMessages := []string{}
	for _, apiService := range apiServices {
		url := "/apis/" + apiService.Spec.Group + "/" + apiService.Spec.Version

		statusCode := 0
		result := restclient.Get().AbsPath(url).Do(context.TODO()).StatusCode(&statusCode)
		if statusCode != http.StatusOK {
			groupVersionString := fmt.Sprintf("%s.%s", apiService.Spec.Group, apiService.Spec.Version)
			recorder.Warningf("OpenShiftAPICheckFailed", fmt.Sprintf("%q failed with HTTP status code %d (%v)", groupVersionString, statusCode, result.Error()))
			missingMessages = append(missingMessages, fmt.Sprintf("%q is not ready: %d (%v)", groupVersionString, statusCode, result.Error()))
		}
	}

	return missingMessages
}
