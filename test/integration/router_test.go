// +build integration,docker

package integration

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	dockerClient "github.com/fsouza/go-dockerclient"
	"golang.org/x/net/websocket"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1beta3"
	"k8s.io/kubernetes/pkg/watch"
	watchjson "k8s.io/kubernetes/pkg/watch/json"

	routeapi "github.com/openshift/origin/pkg/route/api"
	tr "github.com/openshift/origin/test/integration/router"
	testutil "github.com/openshift/origin/test/util"
)

const (
	defaultRouterImage = "openshift/origin-haproxy-router"

	tcWaitSeconds = 1
	tcRetries     = 3

	dockerWaitSeconds = 1
	dockerRetries     = 3
)

// init ensures docker exists for this test
func init() {
	testutil.RequireDocker()
}

// TestRouter is the table based test for routers.  It will initialize a fake master/client and expect to deploy
// a router image in docker.  It then sends watch events through the simulator and makes http client requests that
// should go through the deployed router and return data from the client simulator.
func TestRouter(t *testing.T) {
	//create a server which will act as a user deployed application that
	//serves http and https as well as act as a master to simulate watches
	fakeMasterAndPod := tr.NewTestHttpService()
	defer fakeMasterAndPod.Stop()

	err := fakeMasterAndPod.Start()
	validateServer(fakeMasterAndPod, t)

	if err != nil {
		t.Fatalf("Unable to start http server: %v", err)
	}

	//deploy router docker container
	dockerCli, err := testutil.NewDockerClient()

	if err != nil {
		t.Fatalf("Unable to get docker client: %v", err)
	}

	routerId, err := createAndStartRouterContainer(dockerCli, fakeMasterAndPod.MasterHttpAddr)

	if err != nil {
		t.Fatalf("Error starting container %s : %v", getRouterImage(), err)
	}

	defer cleanUp(dockerCli, routerId)

	httpEndpoint, err := getEndpoint(fakeMasterAndPod.PodHttpAddr)
	if err != nil {
		t.Fatalf("Couldn't get http endpoint: %v", err)
	}
	httpsEndpoint, err := getEndpoint(fakeMasterAndPod.PodHttpsAddr)
	if err != nil {
		t.Fatalf("Couldn't get https endpoint: %v", err)
	}

	//run through test cases now that environment is set up
	testCases := []struct {
		name              string
		serviceName       string
		endpoints         []kapi.EndpointSubset
		routeAlias        string
		routePath         string
		endpointEventType watch.EventType
		routeEventType    watch.EventType
		protocol          string
		expectedResponse  string
		routeTLS          *routeapi.TLSConfig
		routerUrl         string
	}{
		{
			name:              "non-secure",
			serviceName:       "example",
			endpoints:         []kapi.EndpointSubset{httpEndpoint},
			routeAlias:        "www.example-unsecure.com",
			endpointEventType: watch.Added,
			routeEventType:    watch.Added,
			protocol:          "http",
			expectedResponse:  tr.HelloPod,
			routeTLS:          nil,
			routerUrl:         "0.0.0.0",
		},
		{
			name:              "non-secure-path",
			serviceName:       "example-path",
			endpoints:         []kapi.EndpointSubset{httpEndpoint},
			routeAlias:        "www.example-unsecure.com",
			routePath:         "/test",
			endpointEventType: watch.Added,
			routeEventType:    watch.Added,
			protocol:          "http",
			expectedResponse:  tr.HelloPodPath,
			routeTLS:          nil,
			routerUrl:         "0.0.0.0/test",
		},
		{
			name:              "edge termination",
			serviceName:       "example-edge",
			endpoints:         []kapi.EndpointSubset{httpEndpoint},
			routeAlias:        "www.example-edge.com",
			endpointEventType: watch.Added,
			routeEventType:    watch.Added,
			protocol:          "https",
			expectedResponse:  tr.HelloPod,
			routeTLS: &routeapi.TLSConfig{
				Termination:   routeapi.TLSTerminationEdge,
				Certificate:   tr.ExampleCert,
				Key:           tr.ExampleKey,
				CACertificate: tr.ExampleCACert,
			},
			routerUrl: "0.0.0.0",
		},
		{
			name:              "edge termination path",
			serviceName:       "example-edge-path",
			endpoints:         []kapi.EndpointSubset{httpEndpoint},
			routeAlias:        "www.example-edge.com",
			routePath:         "/test",
			endpointEventType: watch.Added,
			routeEventType:    watch.Added,
			protocol:          "https",
			expectedResponse:  tr.HelloPodPath,
			routeTLS: &routeapi.TLSConfig{
				Termination:   routeapi.TLSTerminationEdge,
				Certificate:   tr.ExampleCert,
				Key:           tr.ExampleKey,
				CACertificate: tr.ExampleCACert,
			},
			routerUrl: "0.0.0.0/test",
		},
		{
			name:              "reencrypt",
			serviceName:       "example-reencrypt",
			endpoints:         []kapi.EndpointSubset{httpsEndpoint},
			routeAlias:        "www.example-reencrypt.com",
			endpointEventType: watch.Added,
			routeEventType:    watch.Added,
			protocol:          "https",
			expectedResponse:  tr.HelloPodSecure,
			routeTLS: &routeapi.TLSConfig{
				Termination:              routeapi.TLSTerminationReencrypt,
				Certificate:              tr.ExampleCert,
				Key:                      tr.ExampleKey,
				CACertificate:            tr.ExampleCACert,
				DestinationCACertificate: tr.ExampleCACert,
			},
			routerUrl: "0.0.0.0",
		},
		{
			name:              "reencrypt path",
			serviceName:       "example-reencrypt-path",
			endpoints:         []kapi.EndpointSubset{httpsEndpoint},
			routeAlias:        "www.example-reencrypt.com",
			routePath:         "/test",
			endpointEventType: watch.Added,
			routeEventType:    watch.Added,
			protocol:          "https",
			expectedResponse:  tr.HelloPodPathSecure,
			routeTLS: &routeapi.TLSConfig{
				Termination:              routeapi.TLSTerminationReencrypt,
				Certificate:              tr.ExampleCert,
				Key:                      tr.ExampleKey,
				CACertificate:            tr.ExampleCACert,
				DestinationCACertificate: tr.ExampleCACert,
			},
			routerUrl: "0.0.0.0/test",
		},
		{
			name:              "passthrough termination",
			serviceName:       "example-passthrough",
			endpoints:         []kapi.EndpointSubset{httpsEndpoint},
			routeAlias:        "www.example-passthrough.com",
			endpointEventType: watch.Added,
			routeEventType:    watch.Added,
			protocol:          "https",
			expectedResponse:  tr.HelloPodSecure,
			routeTLS: &routeapi.TLSConfig{
				Termination: routeapi.TLSTerminationPassthrough,
			},
			routerUrl: "0.0.0.0",
		},
		{
			name:              "websocket unsecure",
			serviceName:       "websocket-unsecure",
			endpoints:         []kapi.EndpointSubset{httpEndpoint},
			routeAlias:        "0.0.0.0:80",
			endpointEventType: watch.Added,
			routeEventType:    watch.Added,
			protocol:          "ws",
			expectedResponse:  "hello-websocket-unsecure",
			routerUrl:         "0.0.0.0:80/echo",
		},
		{
			name:              "ws edge termination",
			serviceName:       "websocket-edge",
			endpoints:         []kapi.EndpointSubset{httpEndpoint},
			routeAlias:        "0.0.0.0:443",
			endpointEventType: watch.Added,
			routeEventType:    watch.Added,
			protocol:          "wss",
			expectedResponse:  "hello-websocket-edge",
			routeTLS: &routeapi.TLSConfig{
				Termination:   routeapi.TLSTerminationEdge,
				Certificate:   tr.ExampleCert,
				Key:           tr.ExampleKey,
				CACertificate: tr.ExampleCACert,
			},
			routerUrl: "0.0.0.0:443/echo",
		},
		{
			name:              "ws passthrough termination",
			serviceName:       "websocket-passthrough",
			endpoints:         []kapi.EndpointSubset{httpsEndpoint},
			routeAlias:        "0.0.0.0:443",
			endpointEventType: watch.Added,
			routeEventType:    watch.Added,
			protocol:          "wss",
			expectedResponse:  "hello-websocket-passthrough",
			routeTLS: &routeapi.TLSConfig{
				Termination: routeapi.TLSTerminationPassthrough,
			},
			routerUrl: "0.0.0.0:443/echo",
		},
	}

	ns := "rotorouter"
	for _, tc := range testCases {
		//simulate the events
		endpointEvent := &watch.Event{
			Type: tc.endpointEventType,

			Object: &kapi.Endpoints{
				ObjectMeta: kapi.ObjectMeta{
					Name:      tc.serviceName,
					Namespace: ns,
				},
				Subsets: tc.endpoints,
			},
		}

		routeEvent := &watch.Event{
			Type: tc.routeEventType,
			Object: &routeapi.Route{
				ObjectMeta: kapi.ObjectMeta{
					Name:      tc.serviceName,
					Namespace: ns,
				},
				Host:        tc.routeAlias,
				Path:        tc.routePath,
				ServiceName: tc.serviceName,
				TLS:         tc.routeTLS,
			},
		}

		fakeMasterAndPod.EndpointChannel <- eventString(endpointEvent)
		fakeMasterAndPod.RouteChannel <- eventString(routeEvent)

		for i := 0; i < tcRetries; i++ {
			//wait for router to pick up configs
			time.Sleep(time.Second * tcWaitSeconds)
			//now verify the route with an http client
			resp, err := getRoute(tc.routerUrl, tc.routeAlias, tc.protocol, tc.expectedResponse)

			if err != nil {
				if i != 2 {
					continue
				}
				t.Errorf("Unable to verify response: %v", err)
			}

			if resp != tc.expectedResponse {
				t.Errorf("TC %s failed! Response body %v did not match expected %v", tc.name, resp, tc.expectedResponse)
			} else {
				//good to go, stop trying
				break
			}
		}

		//clean up
		routeEvent.Type = watch.Deleted
		endpointEvent.Type = watch.Deleted

		fakeMasterAndPod.EndpointChannel <- eventString(endpointEvent)
		fakeMasterAndPod.RouteChannel <- eventString(routeEvent)
	}
}

// TestRouterPathSpecificity tests that the router is matching routes from most specific to least when using
// a combination of path AND host based routes.  It also ensures that a host based route still allows path based
// matches via the host header.
//
// For example, the http server simulator acts as if it has a directory structure like:
// /var/www
//         index.html (Hello Pod)
//         /test
//              index.html (Hello Pod Path)
//
// With just a path based route for www.example.com/test I should get Hello Pod Path for a curl to www.example.com/test
// A curl to www.example.com should fall through to the default handlers.  In the test environment it will fall through
// to a call to 0.0.0.0:8080 which is the master simulator
//
// If a host based route for www.example.com is added into the mix I should then be able to curl www.example.com and get
// Hello Pod and still be able to curl www.example.com/test and get Hello Pod Path
//
// If the path based route is deleted I should still be able to curl both routes successfully using the host based path
func TestRouterPathSpecificity(t *testing.T) {
	fakeMasterAndPod := tr.NewTestHttpService()
	err := fakeMasterAndPod.Start()
	if err != nil {
		t.Fatalf("Unable to start http server: %v", err)
	}
	defer fakeMasterAndPod.Stop()

	validateServer(fakeMasterAndPod, t)

	dockerCli, err := testutil.NewDockerClient()
	if err != nil {
		t.Fatalf("Unable to get docker client: %v", err)
	}

	routerId, err := createAndStartRouterContainer(dockerCli, fakeMasterAndPod.MasterHttpAddr)
	if err != nil {
		t.Fatalf("Error starting container %s : %v", getRouterImage(), err)
	}
	defer cleanUp(dockerCli, routerId)

	httpEndpoint, err := getEndpoint(fakeMasterAndPod.PodHttpAddr)
	if err != nil {
		t.Fatalf("Couldn't get http endpoint: %v", err)
	}

	//create path based route
	endpointEvent := &watch.Event{
		Type: watch.Added,
		Object: &kapi.Endpoints{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "myService",
				Namespace: "default",
			},
			Subsets: []kapi.EndpointSubset{httpEndpoint},
		},
	}
	routeEvent := &watch.Event{
		Type: watch.Added,
		Object: &routeapi.Route{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "path",
				Namespace: "default",
			},
			Host:        "www.example.com",
			Path:        "/test",
			ServiceName: "myService",
		},
	}

	fakeMasterAndPod.EndpointChannel <- eventString(endpointEvent)
	fakeMasterAndPod.RouteChannel <- eventString(routeEvent)
	time.Sleep(time.Second * tcWaitSeconds)
	//ensure you can curl path but not main host
	validateRoute("0.0.0.0/test", "www.example.com", "http", tr.HelloPodPath, t)
	//should fall through to the default backend which is 127.0.0.1:8080 where the test server is simulating a master
	validateRoute("0.0.0.0", "www.example.com", "http", tr.HelloMaster, t)

	//create host based route
	routeEvent = &watch.Event{
		Type: watch.Added,
		Object: &routeapi.Route{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "host",
				Namespace: "default",
			},
			Host:        "www.example.com",
			ServiceName: "myService",
		},
	}
	fakeMasterAndPod.RouteChannel <- eventString(routeEvent)
	time.Sleep(time.Second * tcWaitSeconds)
	//ensure you can curl path and host
	validateRoute("0.0.0.0/test", "www.example.com", "http", tr.HelloPodPath, t)
	validateRoute("0.0.0.0", "www.example.com", "http", tr.HelloPod, t)

	//delete path based route
	routeEvent = &watch.Event{
		Type: watch.Deleted,
		Object: &routeapi.Route{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "path",
				Namespace: "default",
			},
			Host:        "www.example.com",
			Path:        "/test",
			ServiceName: "myService",
		},
	}
	fakeMasterAndPod.RouteChannel <- eventString(routeEvent)
	time.Sleep(time.Second * tcWaitSeconds)
	//ensure you can still curl path and host
	validateRoute("0.0.0.0/test", "www.example.com", "http", tr.HelloPodPath, t)
	validateRoute("0.0.0.0", "www.example.com", "http", tr.HelloPod, t)
}

// TestRouterDuplications ensures that the router implementation is keying correctly and resolving routes that may be
// using the same services with different hosts
func TestRouterDuplications(t *testing.T) {
	fakeMasterAndPod := tr.NewTestHttpService()
	err := fakeMasterAndPod.Start()
	if err != nil {
		t.Fatalf("Unable to start http server: %v", err)
	}
	defer fakeMasterAndPod.Stop()

	validateServer(fakeMasterAndPod, t)

	dockerCli, err := testutil.NewDockerClient()
	if err != nil {
		t.Fatalf("Unable to get docker client: %v", err)
	}

	routerId, err := createAndStartRouterContainer(dockerCli, fakeMasterAndPod.MasterHttpAddr)
	if err != nil {
		t.Fatalf("Error starting container %s : %v", getRouterImage(), err)
	}
	defer cleanUp(dockerCli, routerId)

	httpEndpoint, err := getEndpoint(fakeMasterAndPod.PodHttpAddr)
	if err != nil {
		t.Fatalf("Couldn't get http endpoint: %v", err)
	}

	//create routes
	endpointEvent := &watch.Event{
		Type: watch.Added,
		Object: &kapi.Endpoints{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "myService",
				Namespace: "default",
			},
			Subsets: []kapi.EndpointSubset{httpEndpoint},
		},
	}
	exampleRouteEvent := &watch.Event{
		Type: watch.Added,
		Object: &routeapi.Route{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "example",
				Namespace: "default",
			},
			Host:        "www.example.com",
			ServiceName: "myService",
		},
	}
	example2RouteEvent := &watch.Event{
		Type: watch.Added,
		Object: &routeapi.Route{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "example2",
				Namespace: "default",
			},
			Host:        "www.example2.com",
			ServiceName: "myService",
		},
	}

	fakeMasterAndPod.EndpointChannel <- eventString(endpointEvent)
	fakeMasterAndPod.RouteChannel <- eventString(exampleRouteEvent)
	fakeMasterAndPod.RouteChannel <- eventString(example2RouteEvent)

	var examplePass, example2Pass bool
	var exampleResp, example2Resp string
	for i := 0; i < tcRetries; i++ {
		//ensure you can curl both
		examplePass, exampleResp = isValidRoute("0.0.0.0", "www.example.com", "http", tr.HelloPod)
		example2Pass, example2Resp = isValidRoute("0.0.0.0", "www.example2.com", "http", tr.HelloPod)

		if examplePass && example2Pass {
			break
		}
		//not valid yet, give it some more time before failing
		time.Sleep(time.Second * tcWaitSeconds)
	}

	if !examplePass || !example2Pass {
		t.Errorf("Unable to validate both routes in a duplicate service scenario.  Resp 1: %s, Resp 2: %s", exampleResp, example2Resp)
	}
}

// isValidRoute ensures that the route can be retrieved and matches the expected output
func isValidRoute(url, host, scheme, expected string) (valid bool, response string) {
	resp, err := getRoute(url, host, scheme, expected)
	if err != nil {
		return false, err.Error()
	}
	return resp == expected, resp
}

// validateRoute is a helper that will set the unit test error.  It delegates to isValidRoute which can be used
// if you need to check the response/status manually
func validateRoute(url, host, scheme, expected string, t *testing.T) {
	if valid, response := isValidRoute(url, host, scheme, expected); !valid {
		t.Errorf("Unexepected response, wanted: %s but got: %s", expected, response)
	}
}

func getEndpoint(hostport string) (kapi.EndpointSubset, error) {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		return kapi.EndpointSubset{}, err
	}
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return kapi.EndpointSubset{}, err
	}
	return kapi.EndpointSubset{Addresses: []kapi.EndpointAddress{{IP: host}}, Ports: []kapi.EndpointPort{{Port: portNum}}}, nil
}

// getRoute is a utility function for making the web request to a route.  Protocol is either http or https.  If the
// protocol is https then getRoute will make a secure transport client with InsecureSkipVerify: true.  Http does a plain
// http client request.
func getRoute(routerUrl string, hostName string, protocol string, expectedResponse string) (response string, err error) {
	url := protocol + "://" + routerUrl
	var tlsConfig *tls.Config

	if protocol == "https" || protocol == "wss" {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         hostName,
		}
	}

	switch protocol {
	case "http", "https":
		httpClient := &http.Client{Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		}
		req, err := http.NewRequest("GET", url, nil)

		if err != nil {
			return "", err
		}

		req.Host = hostName
		resp, err := httpClient.Do(req)

		if err != nil {
			return "", err
		}

		var respBody = make([]byte, len([]byte(expectedResponse)))
		resp.Body.Read(respBody)

		return string(respBody), nil
	case "ws", "wss":
		wsConfig, err := websocket.NewConfig(url, "http://localhost/")
		if err != nil {
			return "", err
		}

		wsConfig.Header.Set("Host", hostName)
		wsConfig.TlsConfig = tlsConfig

		ws, err := websocket.DialConfig(wsConfig)
		if err != nil {
			return "", err
		}

		_, err = ws.Write([]byte(expectedResponse))
		if err != nil {
			return "", err
		}

		var msg = make([]byte, len(expectedResponse))
		_, err = ws.Read(msg)
		if err != nil {
			return "", err
		}

		return string(msg), nil
	}

	return "", errors.New("Unrecognized protocol in getRoute")
}

// eventString marshals the event into a string
func eventString(e *watch.Event) string {
	obj, _ := watchjson.Object(v1beta3.Codec, e)
	s, _ := json.Marshal(obj)
	return string(s)
}

// createAndStartRouterContainer is responsible for deploying the router image in docker.  It assumes that all router images
// will use a command line flag that can take --master which points to the master url
func createAndStartRouterContainer(dockerCli *dockerClient.Client, masterIp string) (containerId string, err error) {
	ports := []string{"80", "443"}
	portBindings := make(map[dockerClient.Port][]dockerClient.PortBinding)
	exposedPorts := map[dockerClient.Port]struct{}{}

	for _, p := range ports {
		dockerPort := dockerClient.Port(p + "/tcp")

		portBindings[dockerPort] = []dockerClient.PortBinding{
			{
				HostPort: p,
			},
		}

		exposedPorts[dockerPort] = struct{}{}
	}

	containerOpts := dockerClient.CreateContainerOptions{
		Config: &dockerClient.Config{
			Image:        getRouterImage(),
			Cmd:          []string{"--master=" + masterIp, "--loglevel=4"},
			ExposedPorts: exposedPorts,
		},
	}

	container, err := dockerCli.CreateContainer(containerOpts)

	if err != nil {
		return "", err
	}

	dockerHostCfg := &dockerClient.HostConfig{NetworkMode: "host", PortBindings: portBindings}
	err = dockerCli.StartContainer(container.ID, dockerHostCfg)

	if err != nil {
		return "", err
	}

	running := false

	//wait for it to start
	for i := 0; i < dockerRetries; i++ {
		c, err := dockerCli.InspectContainer(container.ID)

		if err != nil {
			return "", err
		}

		if c.State.Running {
			running = true
			break
		}
		time.Sleep(time.Second * dockerWaitSeconds)
	}

	if !running {
		return "", errors.New("Container did not start after 3 tries!")
	}

	return container.ID, nil
}

// validateServer performs a basic run through by validating each of the configured urls for the simulator to
// ensure they are responding
func validateServer(server *tr.TestHttpService, t *testing.T) {
	_, err := http.Get("http://" + server.MasterHttpAddr)

	if err != nil {
		t.Errorf("Error validating master addr %s : %v", server.MasterHttpAddr, err)
	}

	_, err = http.Get("http://" + server.PodHttpAddr)

	if err != nil {
		t.Errorf("Error validating master addr %s : %v", server.MasterHttpAddr, err)
	}

	secureTransport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	secureClient := &http.Client{Transport: secureTransport}
	_, err = secureClient.Get("https://" + server.PodHttpsAddr)

	if err != nil {
		t.Errorf("Error validating master addr %s : %v", server.MasterHttpAddr, err)
	}
}

// cleanUp stops and removes the deployed router
func cleanUp(dockerCli *dockerClient.Client, routerId string) {
	dockerCli.StopContainer(routerId, 5)

	dockerCli.RemoveContainer(dockerClient.RemoveContainerOptions{
		ID:    routerId,
		Force: true,
	})
}

// getRouterImage is a utility that provides the router image to use by checking to see if OPENSHIFT_ROUTER_IMAGE is set
// or by using the default image
func getRouterImage() string {
	i := os.Getenv("OPENSHIFT_ROUTER_IMAGE")

	if len(i) == 0 {
		i = defaultRouterImage
	}

	return i
}
