package app_create

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"

	route "github.com/openshift/origin/pkg/route/apis/route"
)

func (d *AppCreate) createAndCheckRoute() {
	result := &d.result.Route
	result.BeginTime = jsonTime(time.Now())
	defer recordTrial(result)
	if !d.createRoute() {
		return
	}
	if d.skipRouteTest {
		d.out.Debug("DCluAC021", "skipping route test as requested")
		result.Success = true
		return
	}
	result.Success = d.checkRoute()
}

// create the route for the service
func (d *AppCreate) createRoute() bool {
	defer recordTime(&d.result.Route.CreatedTime)
	appRoute := &route.Route{
		ObjectMeta: metav1.ObjectMeta{Name: d.appName, Labels: d.label},
		Spec: route.RouteSpec{
			Host: d.routeHost,
			To: route.RouteTargetReference{
				Kind:   "Service",
				Name:   d.appName,
				Weight: nil,
			},
		},
	}
	if _, err := d.RouteClient.Route().Routes(d.project).Create(appRoute); err != nil {
		d.out.Error("DCluAC022", err, fmt.Sprintf("%s: Creating route '%s' failed:\n%v", now(), d.appName, err))
		return false
	}
	return true
}

// check that the route is admitted and can be reached
func (d *AppCreate) checkRoute() bool {

	if !d.checkRouteAdmitted() {
		return false
	}

	if d.skipRouteConnect {
		d.out.Debug("DCluAC023", "skipping route connection test as requested")
		return true
	}
	return d.checkRouteConnection()
}

func (d *AppCreate) checkRouteAdmitted() bool {
	defer recordTime(&d.result.Route.ReadyTime)

	// set up a watch for route admission
	d.out.Debug("DCluAC024", fmt.Sprintf("%s: Waiting for route to be admitted by a router", now()))
	watcher, err := d.RouteClient.Route().Routes(d.project).Watch(metav1.ListOptions{FieldSelector: "metadata.name=" + d.appName, TimeoutSeconds: &d.routeAdmissionTimeout})
	if err != nil {
		d.out.Error("DCluAC025", err, fmt.Sprintf(`
%s: Failed to establish a watch for '%s' route to be admitted by a router:
  %v
This may be a transient error. Check the master API logs for anomalies near this time.
  		`, now(), d.appName, err))
		return false
	}
	defer stopWatcher(watcher)

	// test for the result of the watch
	for event := range watcher.ResultChan() {
		ready, err := isRouteAdmitted(event)
		if err != nil {
			d.out.Error("DCluAC026", err, fmt.Sprintf(`
%s: Error while watching for route to be admitted by a router:
  %v
This may be a transient error. Check the master API logs for anomalies near this time.
  			`, now(), err))
			return false
		}
		if ready {
			d.out.Debug("DCluAC027", fmt.Sprintf("%s: Route has been admitted by a router", now()))
			return true
		}
	}
	d.out.Error("DCluAC028", nil, fmt.Sprintf(`
%s: Route was not admitted by a router before timeout (%d sec)
Diagnostics waited for the '%s' route to be admitted by a router (making the
application available via that route) after the test app started running.
However, this did not occur within the timeout.
Some of the reasons why this may fail include:
  * There is no router running to accept routes
  * The available router(s) are configured not to accept the route
  * The router simply needs longer to admit the route (you can increase the timeout)
  * The app stopped responding or was killed
	`, now(), d.routeAdmissionTimeout, d.appName))
	return false
}

func isRouteAdmitted(event watch.Event) (bool, error) {
	switch event.Type {
	case watch.Deleted:
		return false, errors.NewNotFound(schema.GroupResource{Resource: "routes"}, "")
	}
	switch r := event.Object.(type) {
	case *route.Route:
		for _, ingress := range r.Status.Ingress {
			for _, cond := range ingress.Conditions {
				if cond.Type == route.RouteAdmitted && cond.Status == "True" {
					return true, nil
				}
			}
		}
		return false, nil
	}
	return false, nil
}

// check that we get the expected HTTP response from the route
func (d *AppCreate) checkRouteConnection() bool {
	defer recordTime(&d.result.Route.TestTime)
	r, err := d.RouteClient.Route().Routes(d.project).Get(d.appName, metav1.GetOptions{})
	if err != nil {
		d.out.Error("DCluAC029", err, fmt.Sprintf("%s: Error retrieving %s route: %v", now(), d.appName, err))
		return false
	}
	url := fmt.Sprintf("http://%s:%d/", r.Spec.Host, d.routePort)
	if err := d.checkHttp(url, d.httpTimeout, d.httpRetries); err != nil {
		d.out.Error("DCluAC030", err, fmt.Sprintf(`
%s: Request to route %s with timeout %dms failed after %d tries.
  Last error was: %v
Diagnostics attempted to connect to the admitted route for the test application,
expecting to receive a successful response with HTTP code 200. This did not happen
within the given timeout.
Some of the reasons why this may fail include:
  * The host running this diagnostic is not configured to resolve the route host via DNS
    (try running from a different host, or skip the route connection test)
  * The router has not yet started routing the route's host after admitting it
    (try increasing the diagnostic timeout or number of retries)
  * The pod stopped or was killed after starting successfully (check pod/node logs)
  * The pod is responding with a non-200 HTTP code (or, not quickly enough / at all)
  * Cluster networking problems prevent the router from connecting to the service
  		`, now(), url, d.httpTimeout, d.httpRetries+1, err))
		return false
	}
	d.out.Info("DCluAC031", fmt.Sprintf("%s: Request to route %s succeeded", now(), url))
	return true
}
