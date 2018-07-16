package app_create

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/apis/core"
)

const (
	serviceEndpointTimeout = 10 // seconds; failsafe to wait for endpoint to appear, which is normally instantaneous.
)

func (d *AppCreate) createAndCheckService() bool {
	result := &d.result.Service
	result.BeginTime = jsonTime(time.Now())
	defer recordTrial(result)
	if !d.createService() || !d.checkServiceEndpoint() {
		return false
	}
	if d.skipServiceConnect {
		d.out.Debug("DCluAC012", "skipping service connection test as requested")
		result.Success = true
	} else {
		result.Success = d.checkServiceConnect()
	}
	return result.Success
}

// create the service for the app
func (d *AppCreate) createService() bool {
	defer recordTime(&d.result.Service.CreatedTime)
	service := &kapi.Service{
		ObjectMeta: metav1.ObjectMeta{Name: d.appName, Labels: d.label},
		Spec: kapi.ServiceSpec{
			Type:     kapi.ServiceTypeClusterIP,
			Selector: d.label,
			Ports: []kapi.ServicePort{
				{
					Protocol:   kapi.ProtocolTCP,
					Port:       8080,
					TargetPort: intstr.FromInt(d.appPort),
				},
			},
		},
	}
	if _, err := d.KubeClient.Core().Services(d.project).Create(service); err != nil {
		d.out.Error("DCluAC013", err, fmt.Sprintf("%s: Creating service '%s' failed:\n%v", now(), d.appName, err))
		return false
	}
	return true
}

// wait for the service to establish endpoints
func (d *AppCreate) checkServiceEndpoint() bool {
	defer recordTime(&d.result.Service.ReadyTime)

	// set up a watcher for endpoints on the service
	timeout := int64(serviceEndpointTimeout)
	d.out.Debug("DCluAC014", fmt.Sprintf("%s: Waiting for service to establish endpoints", now()))
	watcher, err := d.KubeClient.Core().Endpoints(d.project).Watch(metav1.ListOptions{FieldSelector: "metadata.name=" + d.appName, TimeoutSeconds: &timeout})
	if err != nil {
		d.out.Error("DCluAC015", err, fmt.Sprintf(`
%s: Failed to establish a watch for '%s' service to be ready:
  %v
This may be a transient error. Check the master API logs for anomalies near this time.
        `, now(), d.appName, err))
		return false
	}
	defer stopWatcher(watcher)

	// and wait for the results of the watch
	for event := range watcher.ResultChan() {
		ready, err := doesServiceHaveEndpoint(event)
		if err != nil {
			d.out.Error("DCluAC016", err, fmt.Sprintf(`
%s: Error while watching for service endpoint:
  %v
This may be a transient error. Check the master API logs for anomalies near this time.
			`, now(), err))
			return false
		}
		if ready {
			d.out.Debug("DCluAC017", fmt.Sprintf("%s: Service has endpoint", now()))
			return true
		}
	}
	d.out.Error("DCluAC018", nil, fmt.Sprintf(`
%s: Service did not find endpoint before timeout (%d sec)
This is very unusual after the app has a running pod; it should be investigated.
	`, now(), serviceEndpointTimeout))
	return false
}

// Returns false until the service has at least one endpoint.
// Will return an error if the service is deleted or any other error occurs.
func doesServiceHaveEndpoint(event watch.Event) (bool, error) {
	switch event.Type {
	case watch.Deleted:
		return false, errors.NewNotFound(schema.GroupResource{Resource: "services"}, "")
	}
	switch ep := event.Object.(type) {
	case *kapi.Endpoints:
		ss := ep.Subsets
		if len(ss) == 0 || len(ss[0].Addresses) < 1 {
			return false, nil
		}
		return true, nil
	}
	return false, nil
}

// check we can actually get a response from the service
func (d *AppCreate) checkServiceConnect() bool {
	defer recordTime(&d.result.Service.TestTime)
	service, err := d.KubeClient.Core().Services(d.project).Get(d.appName, metav1.GetOptions{})
	if err != nil {
		d.out.Error("DCluAC018", err, fmt.Sprintf("%s: Error retrieving %s service: %v", now(), d.appName, err))
		return false
	}

	url := fmt.Sprintf("http://%s:8080/", service.Spec.ClusterIP)
	if err := d.checkHttp(url, d.httpTimeout, d.httpRetries); err != nil {
		d.out.Error("DCluAC019", err, fmt.Sprintf(`
%s: Request to service %s with timeout %dms failed after %d tries.
  Last error was: %v
Diagnostics attempted to connect to the service address for the test application,
expecting to receive a successful response with HTTP code 200. This did not happen
within the given timeout.
Some of the reasons why this may fail include:
  * The host running this diagnostic is not part of the cluster SDN
    (try running from a master, or skip the service connection test)
  * The pod stopped or was killed after starting successfully (check pod/node logs)
  * The pod is responding with a non-200 HTTP code (or, not quickly enough / at all)
  * Cluster networking problems prevent connecting to the service
  		`, now(), url, d.httpTimeout, d.httpRetries+1, err))
		return false
	}
	d.out.Info("DCluAC020", fmt.Sprintf("%s: Request to service address %s succeeded", now(), url))
	return true
}
