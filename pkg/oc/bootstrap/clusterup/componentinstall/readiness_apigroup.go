package componentinstall

import (
	"net/http"
	"reflect"
	"time"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/sets"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	aggregatorapiv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
	aggregatorclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
)

func WaitForAPIs(clientConfig *rest.Config, names ...string) error {
	// wait until the openshift apiservices are ready
	return wait.PollImmediate(time.Second, 5*time.Minute, func() (bool, error) {
		requiredNames := sets.NewString(names...)

		aggregatorClient, err := aggregatorclient.NewForConfig(clientConfig)
		if err != nil {
			return false, nil
		}

		unready := []string{}
		rawDiscoveryUrls := []string{}
		apiServices, err := aggregatorClient.ApiregistrationV1beta1().APIServices().List(metav1.ListOptions{})
		if err != nil {
			return false, err
		}

		found := []string{}
		for _, apiService := range apiServices.Items {
			if !requiredNames.Has(apiService.Name) {
				continue
			}
			found = append(found, apiService.Name)
			glog.V(5).Infof("found: %v\n", apiService.Name)

			for _, condition := range apiService.Status.Conditions {
				if condition.Type == aggregatorapiv1beta1.Available && condition.Status != aggregatorapiv1beta1.ConditionTrue {
					glog.V(4).Infof("waiting for readiness: %v %#v\n", apiService.Name, condition)
					unready = append(unready, apiService.Name)
					continue
				}

				rawDiscoveryUrls = append(rawDiscoveryUrls, "/apis/"+apiService.Spec.Group+"/"+apiService.Spec.Version)
			}
		}
		if len(unready) > 0 {
			glog.V(3).Infof("waiting for readiness: %#v\n", unready)
			return false, nil
		}
		// we can reasonably expect to find at least one hit.  Hit this edge with the operator.
		if foundSet := sets.NewString(found...); !reflect.DeepEqual(requiredNames, foundSet) {
			glog.V(3).Infof("missing: %v", requiredNames.Difference(foundSet).List())
			return false, nil
		}

		missingURLs := []string{}
		for _, url := range rawDiscoveryUrls {
			statusCode := 0
			aggregatorClient.Discovery().RESTClient().Get().AbsPath(url).Do().StatusCode(&statusCode)
			if statusCode != http.StatusOK {
				glog.V(3).Infof("waiting for url: %q %v\n", url, statusCode)
				missingURLs = append(missingURLs, url)
			}
		}
		if len(missingURLs) > 0 {
			glog.V(3).Infof("waiting for urls: %#v\n", missingURLs)
			return false, nil
		}

		return true, nil
	})
}
