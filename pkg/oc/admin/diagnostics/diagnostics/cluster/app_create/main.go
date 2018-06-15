package app_create

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	kvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/kubernetes/pkg/apis/authorization"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	authorizationtypedclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/authorization/internalversion"

	appsclient "github.com/openshift/origin/pkg/apps/generated/internalclientset"
	oauthorizationtypedclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset/typed/authorization/internalversion"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/log"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/types"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/util"
	osclientcmd "github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	projectclient "github.com/openshift/origin/pkg/project/generated/internalclientset/typed/project/internalversion"
	routeclient "github.com/openshift/origin/pkg/route/generated/internalclientset"
)

// AppCreate is a Diagnostic to create an application and test that it deploys correctly.
type AppCreate struct {
	PreventModification bool
	KubeClient          kclientset.Interface
	ProjectClient       projectclient.ProjectInterface
	RouteClient         *routeclient.Clientset
	RoleBindingClient   oauthorizationtypedclient.RoleBindingsGetter
	AppsClient          *appsclient.Clientset
	SARClient           authorizationtypedclient.SelfSubjectAccessReviewsGetter
	Factory             *osclientcmd.Factory

	// from parameters specific to this diagnostic:
	// specs for the project where the diagnostic will put all test items
	nodeSelector string
	project      string
	projectBase  string
	keepProject  bool
	// run a build and deploy the result if successful
	checkBuild bool
	keepBuild  bool
	// deploy an app, service, and route
	appName               string
	appImage              string
	appPort               int
	deployTimeout         int64
	keepApp               bool
	routeHost             string
	routePort             int
	routeAdmissionTimeout int64
	// connection testing parameters
	httpTimeout        int64
	httpRetries        int
	skipServiceConnect bool // service SDN may not be visible from client
	skipRouteTest      bool // may not expect acceptance (e.g. router may not be running)
	skipRouteConnect   bool // DNS/network may not be as expected for client to connect to route
	// misc
	writeResultDir string
	label          map[string]string // for selecting components later
	labelSelector  string            // for selecting components later

	// diagnostic state
	out    types.DiagnosticResult
	result appCreateResult
}

// create/tests results and timings
type appCreateResult struct {
	BeginTime     jsonTime     `json:"beginTime"`     // time when diagnostic begins
	PrepDuration  jsonDuration `json:"prepDuration"`  // time required to prepare project for app creation
	EndTime       jsonTime     `json:"endTime"`       // time when all tests completed
	TotalDuration jsonDuration `json:"totalDuration"` // interval between BeginTime and EndTime
	Success       bool         `json:"success"`       // overallresult

	App     appCreateComponentResult `json:"app"`
	Service appCreateComponentResult `json:"service"`
	Route   appCreateComponentResult `json:"route"`
}

type appCreateComponentResult struct {
	BeginTime       jsonTime     `json:"beginTime"`   // begin time for create/test of this component
	CreatedTime     jsonTime     `json:"createdTime"` // time component creation completed (or failed)
	CreatedDuration jsonDuration `json:"createdDuration"`
	ReadyTime       jsonTime     `json:"readyTime"`     // time at which component is considered ready (or failed)
	ReadyDuration   jsonDuration `json:"readyDuration"` // interval between created and ready
	TestTime        jsonTime     `json:"testTime"`      // time at which test is considered succeeded/failed
	TestDuration    jsonDuration `json:"testDuration"`  // interval between ready and test success/failure
	EndTime         jsonTime     `json:"endTime"`       // time when component create/test completed
	TotalDuration   jsonDuration `json:"totalDuration"` // interval between BeginTime and EndTime
	Required        bool         `json:"required"`      // was component actually required so result counts
	Success         bool         `json:"success"`       // overall component result (if required at all)
}

// using this type to have duration reported as null or seconds.
type jsonDuration int64

func (d jsonDuration) MarshalJSON() ([]byte, error) {
	duration := time.Duration(d)
	encoding := "null"
	if duration != 0 {
		encoding = fmt.Sprintf("%f", duration.Seconds())
	}
	return []byte(encoding), nil
}

func (d jsonDuration) String() string {
	return time.Duration(d).String()
}

// using this type to have time reported as null when not set
type jsonTime time.Time

func (t jsonTime) MarshalJSON() ([]byte, error) {
	it := time.Time(t)
	if it.IsZero() {
		return []byte("null"), nil
	}
	return it.MarshalJSON()
}
func (t jsonTime) IsZero() bool {
	return time.Time(t).IsZero()
}
func (t jsonTime) Sub(sub jsonTime) jsonDuration {
	return jsonDuration(time.Time(t).Sub(time.Time(sub)))
}

const (
	AppCreateName = "AppCreate"

	AppCreateProjectBaseDefault                 = "openshift-diagnostic-appcreate-"
	AppCreateAppNameDefault                     = "diagnostic-appcreate"
	AppCreateAppPortDefault                     = 8080
	AppCreateTimeoutDefault               int64 = 120
	AppCreateHttpTimeoutDefault           int64 = 500
	AppCreateHttpRetriesDefault                 = 10
	AppCreateRouteAdmissionTimeoutDefault int64 = 10
)

func (d *AppCreate) Name() string {
	return AppCreateName
}

func (d *AppCreate) Description() string {
	return "Create an application and test that it deploys correctly."
}

func (d *AppCreate) Requirements() (client bool, host bool) {
	return true, false
}

func NewDefaultAppCreateDiagnostic() *AppCreate {
	return &AppCreate{
		projectBase: AppCreateProjectBaseDefault,
		checkBuild:  true,
		appName:     AppCreateAppNameDefault,
		appImage:    getDefaultAppImage(),
		appPort:     AppCreateAppPortDefault,
		httpTimeout: AppCreateHttpTimeoutDefault,
		httpRetries: AppCreateHttpRetriesDefault,
	}
}

func (d *AppCreate) AvailableParameters() []types.Parameter {
	return []types.Parameter{
		{"project", "Project name to use instead of generating from project-base", &d.project, ""},
		{"project-base", "Base name to create randomized project name", &d.projectBase, AppCreateProjectBaseDefault},
		{"keep-project", "Do not delete randomized project when complete", &d.keepProject, false},
		{"app-name", "Name for the test application to be created", &d.appName, AppCreateAppNameDefault},
		{"app-image", "Image for the test application to be created", &d.appImage, getDefaultAppImage()},
		{"app-port", "Port at which the test application listens", &d.appPort, AppCreateAppPortDefault},
		{"route-host", "Create specific route instead of default", &d.routeHost, ""},
		{"route-port", "Router port to use for route connection test", &d.routePort, 80},
		{"deploy-timeout", "Seconds to wait for the app to be ready", &d.deployTimeout, AppCreateTimeoutDefault},
		{"admission-timeout", "Seconds to wait for the route to be admitted by a router", &d.routeAdmissionTimeout, AppCreateRouteAdmissionTimeoutDefault},
		{"skip-service-connect", "Do not test connecting to the service", &d.skipServiceConnect, false},
		{"skip-route-test", "Do not test route at all", &d.skipRouteTest, false},
		{"skip-route-connect", "Do not test connecting to the route", &d.skipRouteConnect, false},
		{"http-timeout", "Milliseconds to wait for an HTTP request to the app", &d.httpTimeout, AppCreateHttpTimeoutDefault},
		{"http-retries", "Number of times to retry an HTTP request to the app", &d.httpRetries, AppCreateHttpRetriesDefault},
		{"node-selector", "Node selector for where the test app should land", &d.nodeSelector, ""},
		{"keep-app", "Do not delete the test app when complete", &d.keepApp, false},
		{"result-dir", "Directory in which to write result details if desired", &d.writeResultDir, ""},
	}
}

func getDefaultAppImage() string {
	template := variable.NewDefaultImageTemplate()
	return template.ExpandOrDie("deployer")
}

func (d *AppCreate) Complete(logger *log.Logger) error {
	// project management
	d.keepProject = d.keepProject || d.keepApp // don't delete project if keeping app
	if d.project == "" && d.projectBase == "" {
		return fmt.Errorf("%s project name cannot be empty", AppCreateName)
	}
	if d.project == "" {
		// generate a project if not specified
		d.project = names.SimpleNameGenerator.GenerateName(d.projectBase)
	} else {
		// when an existing project is specified, deleting it is likely to surprise the user, so don't
		d.keepProject = true
	}
	if errs := kvalidation.IsDNS1123Label(d.project); len(errs) > 0 {
		return fmt.Errorf("invalid project name '%s' for AppCreate: %v", d.project, errs)
	}
	// TODO: also test that route is valid under DNS952

	// app management
	if d.appName == "" {
		return fmt.Errorf("%s app name cannot be empty", AppCreateName)
	}
	if errs := kvalidation.IsDNS1123Label(d.appName); len(errs) > 0 {
		return fmt.Errorf("invalid app name '%s' for AppCreate: %v", d.appName, errs)
	}
	if err := kvalidation.IsValidPortNum(d.appPort); err != nil {
		return fmt.Errorf("invalid app port %d for AppCreate: %v", d.appPort, err)
	}
	d.label = map[string]string{"app": d.appName}
	d.labelSelector = fmt.Sprintf("app=%s", d.appName)

	d.skipRouteConnect = d.skipRouteConnect || d.skipRouteTest // don't try to connect to route if skipping route test

	return nil
}

func (d *AppCreate) CanRun() (bool, error) {
	if d.SARClient == nil || d.AppsClient == nil || d.KubeClient == nil || d.ProjectClient == nil || d.RoleBindingClient == nil {
		return false, fmt.Errorf("missing at least one client")
	}
	if d.PreventModification {
		return false, fmt.Errorf("requires modifications: create a project and application")
	}
	return util.UserCan(d.SARClient, &authorization.ResourceAttributes{
		Verb:     "create",
		Group:    kapi.GroupName,
		Resource: "namespace",
	})
}

func (d *AppCreate) Check() types.DiagnosticResult {
	d.out = types.NewDiagnosticResult(AppCreateName)
	done := make(chan bool, 1)

	// Jump straight to clean up if there is an interrupt/terminate signal while running diagnostic
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		d.out.Warn("DCluAC001", nil, "Received interrupt; aborting diagnostic")
		done <- true
	}()

	// The actual diagnostic logic
	go func() {
		d.result.BeginTime = jsonTime(time.Now())
		defer func() {
			d.result.EndTime = jsonTime(time.Now())
			d.result.TotalDuration = d.result.EndTime.Sub(d.result.BeginTime)
			done <- true
		}()
		if !d.prepareForApp() || !d.createAndCheckAppDC() || !d.createAndCheckService() {
			return // without success
			// NOTE: even if we won't try to connect, we still create the service to test for endpoints
		}
		if d.skipRouteTest {
			d.out.Debug("DCluAC002", "skipping route creation and testing as requested")
			// however if we just skip connection testing we still create and test for admission
		} else {
			d.createAndCheckRoute()
		}
		d.result.Success = allSucceeded(&d.result.App, &d.result.Service, &d.result.Route)
	}()

	<-done // wait until either finishes
	signal.Stop(sig)
	d.logResult()
	d.cleanup()
	return d.out
}

func allSucceeded(components ...*appCreateComponentResult) bool {
	for _, comp := range components {
		if comp.Required && !comp.Success {
			return false
		}
	}
	return true
}

func now() string {
	return time.Now().Format("15:04:05.999")
}

func recordTime(at *jsonTime) {
	*at = jsonTime(time.Now())
}

func recordTrial(result *appCreateComponentResult) {
	result.EndTime = jsonTime(time.Now())
	result.TotalDuration = result.EndTime.Sub(result.BeginTime)
	result.Required = true
	if result.CreatedTime.IsZero() {
		return
	}
	result.CreatedDuration = result.CreatedTime.Sub(result.BeginTime)
	if result.ReadyTime.IsZero() {
		return
	}
	result.ReadyDuration = result.ReadyTime.Sub(result.CreatedTime)
	if result.TestTime.IsZero() {
		return
	}
	result.TestDuration = result.TestTime.Sub(result.ReadyTime)
}

func stopWatcher(watcher watch.Interface) {
	watcher.Stop()
}

func (d *AppCreate) checkHttp(url string, timeout int64, retry int) error {
	timeoutDuration := time.Millisecond * time.Duration(timeout)
	client := &http.Client{Timeout: timeoutDuration}
	var requestErr error = nil
	start := time.Now()
	for try := 0; try <= retry; try++ {
		if requestErr != nil { // wait to retry if quick response in previous try
			time.Sleep(timeoutDuration - time.Since(start))
		}
		start = time.Now()
		d.out.Debug("DCluAC032", fmt.Sprintf("%s: waiting %dms for an HTTP response from %s", now(), timeout, url))
		response, err := client.Get(url)
		respondedTime := time.Since(start)
		if err != nil {
			d.out.Debug("DCluAC033", fmt.Sprintf("%s: Request to %s returned an error or timed out in %v: %v", now(), url, respondedTime, err))
			requestErr = err
			continue
		}
		response.Body.Close()
		if response.StatusCode != 200 {
			requestErr = fmt.Errorf("Saw HTTP response %d", response.StatusCode)
			d.out.Debug("DCluAC034", fmt.Sprintf("%s: Request to %s returned non-200 status code after %v: %v", now(), url, respondedTime, requestErr))
			continue
		}
		d.out.Debug("DCluAC035", fmt.Sprintf("%s: Completed HTTP request to %s successfully in %v on try #%d", now(), url, respondedTime, try))
		return nil
	}
	return requestErr
}
