package main

import (
	"flag"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kubeclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/version/verflag"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/golang/glog"
	osclient "github.com/openshift/origin/pkg/client"
	routeapi "github.com/openshift/origin/pkg/route/api"
	"github.com/openshift/origin/plugins/lb/router"
)

// LBManager is responsible for synchronizing endpoint objects stored
// in the system with actual running pods.
type LBManager struct {
	epWatcher    kubeclient.EndpointsInterface
	routeWatcher osclient.Interface
	*sync.Mutex
	syncTime     <-chan time.Time
}

// NewLBManager creates a new LBManager.
func NewLBManager(epWatcher kubeclient.Interface, routeWatcher osclient.Interface) *LBManager {
	lm := &LBManager{
		epWatcher:    epWatcher,
		routeWatcher: routeWatcher,
	}
	return lm
}

// Run begins watching and syncing.
func (lm *LBManager) Run(period time.Duration) {
	lm.syncTime = time.Tick(period)
	resourceVersion := uint64(0)
	go util.Forever(func() { lm.watchEps(&resourceVersion) }, period)
	go util.Forever(func() { lm.watchRoutes(&resourceVersion) }, period)
}

// resourceVersion is a pointer to the resource version to use/update.
func (lm *LBManager) watchRoutes(resourceVersion *uint64) {
	//fmt.Printf("Checking existing routes: \n")
	watching, err := lm.routeWatcher.WatchRoutes(
		labels.Everything(),
		labels.Everything(),
		*resourceVersion,
	)
	if err != nil {
		glog.Errorf("Unexpected failure to watch: %v", err)
		time.Sleep(5 * time.Second)
		return
	}

	if *debug == true {
		fmt.Printf("Now entering watch mode.\n")
	}
	for {
		select {
		case <-lm.syncTime:
			//lm.synchronize()
			if *debug == true {
				fmt.Printf(".")
			}
		case event, open := <-watching.ResultChan():
			if !open {
				// watchChannel has been closed, or something else went
				// wrong with our etcd watch call. Let the util.Forever()
				// that called us call us again.
				return
			}
			rc, ok := event.Object.(*routeapi.Route)
			if !ok {
				glog.Errorf("unexpected object: %#v", event.Object)
				continue
			}
			*resourceVersion = rc.ResourceVersion + 1
			// Sync even if this is a deletion event, to ensure that we leave
			// it in the desired state.
			//glog.Infof("About to sync from watch: %v", *rc)
			lm.Lock()
			lm.syncRouteHandler(event.Type, *rc)
			lm.Unlock()
		}
	}
}

// resourceVersion is a pointer to the resource version to use/update.
func (lm *LBManager) watchEps(resourceVersion *uint64) {
	watching, err := lm.epWatcher.WatchEndpoints(
		labels.Everything(),
		labels.Everything(),
		*resourceVersion,
	)
	if err != nil {
		glog.Errorf("Unexpected failure to watch: %v", err)
		time.Sleep(5 * time.Second)
		return
	}

	if *debug == true {
		fmt.Printf("Now entering watch mode.\n")
	}
	for {
		select {
		case <-lm.syncTime:
			//lm.synchronize()
			if *debug == true {
				fmt.Printf(".")
			}
		case event, open := <-watching.ResultChan():
			if !open {
				// watchChannel has been closed, or something else went
				// wrong with our etcd watch call. Let the util.Forever()
				// that called us call us again.
				return
			}
			rc, ok := event.Object.(*api.Endpoints)
			if !ok {
				glog.Errorf("unexpected object: %#v", event.Object)
				continue
			}
			*resourceVersion = rc.ResourceVersion + 1
			// Sync even if this is a deletion event, to ensure that we leave
			// it in the desired state.
			//glog.Infof("About to sync from watch: %v", *rc)
			lm.Lock()
			lm.syncEpHandler(event.Type, *rc)
			lm.Unlock()
		}
	}
}

func (lm *LBManager) syncRouteHandler(event watch.EventType, app routeapi.Route) {
	fmt.Printf("App Name : %s\n", app.ServiceName)
	fmt.Printf("\tAlias : %s\n", app.Host)
	fmt.Printf("\tEvent : %s\n", event)

	_, ok := router.FindFrontend(app.ServiceName)
	if !ok {
		router.CreateFrontend(app.ServiceName, "")
	}

	if event == "ADDED" || event == "MODIFIED" {
		fmt.Printf("Modifying routes for %s\n", app.ServiceName)
		router.AddAlias(app.Host, app.ServiceName)
	}
	router.WriteConfig()
	router.ReloadRouter()
}

func (lm *LBManager) syncEpHandler(event watch.EventType, app api.Endpoints) {
	fmt.Printf("App Name : %s\n", app.ID)
	fmt.Printf("\tNumber of endpoints : %d\n", len(app.Endpoints))
	for i, e := range app.Endpoints {
		fmt.Printf("\tEndpoint %d : %s\n", i, e)
	}
	_, ok := router.FindFrontend(app.ID)
	if !ok {
		router.CreateFrontend(app.ID, "") //"www."+app.ID+".com"
	}

	// Delete the endpoints only
	router.DeleteBackends(app.ID)

	if event == "ADDED" || event == "MODIFIED" {
		fmt.Printf("Modifying endpoints for %s\n", app.ID)
		eps := make([]router.Endpoint, len(app.Endpoints))
		for i, e := range app.Endpoints {
			ep := router.Endpoint{}
			if strings.Contains(e, ":") {
				e_arr := strings.Split(e, ":")
				ep.IP = e_arr[0]
				ep.Port = e_arr[1]
			} else if e == "" {
				continue
			} else {
				ep.IP = e
				ep.Port = "80"
			}
			eps[i] = ep
		}
		router.AddRoute(app.ID, "", "", nil, eps)
	}
	router.WriteConfig()
	router.ReloadRouter()
}

var (
	master = flag.String("master", "", "The address of the Kubernetes API server")
	debug  = flag.Bool("verbose", false, "Boolean flag to turn on debug messages")
)

func main() {
	flag.Parse()
	util.InitLogs()
	defer util.FlushLogs()

	verflag.PrintAndExitIfRequested()

	if len(*master) == 0 {
		glog.Fatal("usage: lb -master <master>")
	}

	auth := &kubeclient.AuthInfo{User: "vagrant", Password: "vagrant"}
	kubeClient, err := kubeclient.New(*master, "", auth)
	if err != nil {
		glog.Fatalf("Invalid -master: %v", err)
	}

	osClient, _ := osclient.New(*master, "", auth)
	controllerManager := NewLBManager(kubeClient, osClient)
	controllerManager.Run(10 * time.Second)
	select {}
}
