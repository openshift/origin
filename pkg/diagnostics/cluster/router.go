package cluster

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrs "k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	osclient "github.com/openshift/origin/pkg/client"
	osapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

// ClusterRouter is a Diagnostic to check that there is a working router.
type ClusterRouter struct {
	KubeClient *kclient.Client
	OsClient   *osclient.Client
}

const (
	ClusterRouterName = "ClusterRouter"

	routerName = "router"

	clientAccessError = `Client error while retrieving router records. Client retrieved records
during discovery, so this is likely to be a transient error. Try running
diagnostics again. If this message persists, there may be a permissions
problem with getting router records. The error was:

(%T) %[1]v`

	clGetRtNone = `
There is no "%s" DeploymentConfig. The router may have been named
something different, in which case this warning may be ignored.

A router is not strictly required; however it is needed for accessing
pods from external networks and its absence likely indicates an incomplete
installation of the cluster.

Use the 'oadm router' command to create a router.
`
	clGetRtFailed = `
Client error while retrieving "%s" DC. Client retrieved records
before, so this is likely to be a transient error. Try running
diagnostics again. If this message persists, there may be a permissions
problem with getting records. The error was:

(%[2]T) %[2]v`

	clRtNoPods = `
The "%s" DeploymentConfig exists but has no running pods, so it
is not available. Apps will not be externally accessible via the router.`

	clRtPodLog = `
Failed to read the logs for the "%s" pod belonging to
the router deployment. This is not a problem by itself but prevents
diagnostics from looking for errors in those logs. The error encountered
was:
%s`

	clRtPodConn = `
Recent pod logs for the "%s" pod belonging to
the router deployment indicated a problem requesting route information
from the master. This prevents the router from functioning, so
applications will not be externally accessible via the router.

There are many reasons for this request to fail, including invalid
credentials, DNS failures, master outages, and so on. Examine the
following error message from the router pod logs to determine the
cause of the problem:

%s
Time: %s`
)

func (d *ClusterRouter) Name() string {
	return "ClusterRouterName"
}

func (d *ClusterRouter) Description() string {
	return "Check there is a working router"
}

func (d *ClusterRouter) CanRun() (bool, error) {
	if d.KubeClient == nil || d.OsClient == nil {
		return false, errors.New("must have kube and os client")
	}
	can, err := adminCan(d.OsClient, authorizationapi.AuthorizationAttributes{
		Namespace:    kapi.NamespaceDefault,
		Verb:         "get",
		Resource:     "dc",
		ResourceName: routerName,
	})
	if err != nil {
		return false, types.DiagnosticError{"DClu2010", fmt.Sprintf(clientAccessError, err), err}
	} else if !can {
		return false, types.DiagnosticError{"DClu2011", "Client does not have cluster-admin access", err}
	}
	return true, nil
}

func (d *ClusterRouter) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult(ClusterRouterName)
	if dc := d.getRouterDC(r); dc != nil {
		// Check that it actually has running pod(s) selected
		if podList := d.getRouterPods(dc, r); podList != nil {
			for _, pod := range podList.Items {
				// Check the logs for that pod for common issues (credentials, DNS resolution failure)
				d.checkRouterLogs(&pod, r)
			}
		}
	}
	return r
}

func (d *ClusterRouter) getRouterDC(r types.DiagnosticResult) *osapi.DeploymentConfig {
	dc, err := d.OsClient.DeploymentConfigs(kapi.NamespaceDefault).Get(routerName)
	if err != nil && reflect.TypeOf(err) == reflect.TypeOf(&kerrs.StatusError{}) {
		r.Warn("DClu2001", err, fmt.Sprintf(clGetRtNone, routerName))
		return nil
	} else if err != nil {
		r.Error("DClu2002", err, fmt.Sprintf(clGetRtFailed, routerName, err))
		return nil
	}
	r.Debug("DClu2003", fmt.Sprintf("Found default router DC"))
	return dc
}

func (d *ClusterRouter) getRouterPods(dc *osapi.DeploymentConfig, r types.DiagnosticResult) *kapi.PodList {
	pods, err := d.KubeClient.Pods(kapi.NamespaceDefault).List(labels.SelectorFromSet(dc.Template.ControllerTemplate.Selector), fields.Everything())
	if err != nil {
		r.Error("DClu2004", err, fmt.Sprintf("Finding pods for '%s' DeploymentConfig failed. This should never happen. Error: (%[2]T) %[2]v", routerName, err))
		return nil
	}
	running := []kapi.Pod{}
	for _, pod := range pods.Items {
		if pod.Status.Phase != kapi.PodRunning {
			r.Debug("DClu2005", fmt.Sprintf("router pod with name %s is not running", pod.ObjectMeta.Name))
		} else {
			running = append(running, pod)
			r.Debug("DClu2006", fmt.Sprintf("Found running router pod with name %s", pod.ObjectMeta.Name))
		}
	}
	pods.Items = running
	if len(running) == 0 {
		r.Error("DClu2007", nil, fmt.Sprintf(clRtNoPods, routerName))
		return nil
	}
	return pods
}

// It's like a ReadCloser that gives back lines of text and you still have to Close().
type lineScanner struct {
	Scanner    *bufio.Scanner
	ReadCloser io.ReadCloser
}

func (s *lineScanner) Scan() bool   { return s.Scanner.Scan() }
func (s *lineScanner) Text() string { return s.Scanner.Text() }
func (s *lineScanner) Close() error { return s.ReadCloser.Close() }

func (d *ClusterRouter) getPodLogScanner(pod *kapi.Pod) (*lineScanner, error) {
	readCloser, err := d.KubeClient.RESTClient.Get().
		Namespace(pod.ObjectMeta.Namespace).
		Name(pod.ObjectMeta.Name).
		Resource("pods").SubResource("log").
		Param("follow", "false").
		Param("container", pod.Spec.Containers[0].Name).
		Stream()
	if err != nil {
		return nil, err
	}
	return &lineScanner{bufio.NewScanner(readCloser), readCloser}, nil
}

// http://golang.org/pkg/time/#Parse
// reference time is Mon Jan 2 15:04:05 -0700 MST 2006
var referenceTimestampLayout = "2006-01-02T15:04:05.000000000Z"

func (d *ClusterRouter) checkRouterLogs(pod *kapi.Pod, r types.DiagnosticResult) {
	scanner, err := d.getPodLogScanner(pod)
	if err != nil {
		r.Warn("DClu2008", err, fmt.Sprintf(clRtPodLog, pod.ObjectMeta.Name, fmt.Sprintf("(%T) %[1]v", err)))
		return
	}
	defer scanner.Close()

	for scanner.Scan() {
		matches := regexp.MustCompile(`^(\S+).*Failed to list \*api.Route: (.*)`).FindStringSubmatch(scanner.Text())
		if len(matches) > 0 {
			stamp, err := time.Parse(referenceTimestampLayout, matches[1])
			// router checks every second. error only if failure is recent.
			// of course... we cannot always trust the local clock.
			if err == nil && time.Since(stamp).Seconds() < 30.0 {
				r.Error("DClu2009", nil, fmt.Sprintf(clRtPodConn, pod.ObjectMeta.Name, matches[2], matches[1]))
				break
			}
		}
	}
}
