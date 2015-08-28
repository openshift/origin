// +build integration,docker

package integration

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
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

	routeAddress := getRouteAddress()
	routerTestAddress := fmt.Sprintf("%s/test", routeAddress)
	routerEchoHttpAddress := fmt.Sprintf("%s:80/echo", routeAddress)
	routerEchoHttpsAddress := fmt.Sprintf("%s:443/echo", routeAddress)

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
			routerUrl:         routeAddress,
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
			routerUrl:         routerTestAddress,
		},
		{
			name:              "edge termination",
			serviceName:       "example-edge",
			endpoints:         []kapi.EndpointSubset{httpEndpoint},
			routeAlias:        "www.example.com",
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
			routerUrl: routeAddress,
		},
		{
			name:              "edge termination path",
			serviceName:       "example-edge-path",
			endpoints:         []kapi.EndpointSubset{httpEndpoint},
			routeAlias:        "www.example.com",
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
			routerUrl: routerTestAddress,
		},
		{
			name:              "reencrypt",
			serviceName:       "example-reencrypt",
			endpoints:         []kapi.EndpointSubset{httpsEndpoint},
			routeAlias:        "www.example.com",
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
			routeAlias:        "www.example.com",
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
			routerUrl: routeAddress,
		},
		{
			name:              "websocket unsecure",
			serviceName:       "websocket-unsecure",
			endpoints:         []kapi.EndpointSubset{httpEndpoint},
			routeAlias:        "www.example.com",
			endpointEventType: watch.Added,
			routeEventType:    watch.Added,
			protocol:          "ws",
			expectedResponse:  "hello-websocket-unsecure",
			routerUrl:         routerEchoHttpAddress,
		},
		{
			name:              "ws edge termination",
			serviceName:       "websocket-edge",
			endpoints:         []kapi.EndpointSubset{httpEndpoint},
			routeAlias:        "www.example.com",
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
			routerUrl: routerEchoHttpsAddress,
		},
		{
			name:              "ws passthrough termination",
			serviceName:       "websocket-passthrough",
			endpoints:         []kapi.EndpointSubset{httpsEndpoint},
			routeAlias:        "www.example.com",
			endpointEventType: watch.Added,
			routeEventType:    watch.Added,
			protocol:          "wss",
			expectedResponse:  "hello-websocket-passthrough",
			routeTLS: &routeapi.TLSConfig{
				Termination: routeapi.TLSTerminationPassthrough,
			},
			routerUrl: routerEchoHttpsAddress,
		},
	}

	ns := "rotorouter"
	for _, tc := range testCases {
		// The following is a workaround for the websocket client, which does not
		// allow a "Host" header that is distinct from the address to which the
		// client code attempts to connect—so if we are putting "www.example.com" in
		// the "Host" header, the client will connect to "www.example.com".
		//
		// In the case where we use HAProxy (with the template router), it is
		// possible to use 0.0.0.0, so we can do so as a workaround to get the tests
		// passing with the template router.  In the case of the F5 router though,
		// F5 BIG-IP would reject 0.0.0.0 as an invalid servername, so the only way
		// to make the tests pass with the F5 router is to use a hostname and make
		// that hostname resolve to the F5 BIG-IP host's IP address.
		if getRouterImage() == defaultRouterImage &&
			(tc.protocol == "ws" || tc.protocol == "wss") {
			tc.routeAlias = "0.0.0.0"
		}

		// Simulate the events.
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

		// Give the router some time to finish processing events before we connect.
		time.Sleep(time.Second * 5)

		for i := 0; i < tcRetries; i++ {
			// Wait for router to pick up configs.
			time.Sleep(time.Second * tcWaitSeconds)

			// Now verify the route with an HTTP client.
			resp, err := getRoute(tc.routerUrl, tc.routeAlias, tc.protocol, tc.expectedResponse)

			if err != nil {
				if i != 2 {
					continue
				}
				t.Errorf("Unable to verify response: %v", err)
			}

			if resp != tc.expectedResponse {
				t.Errorf("TC %s failed! Response body %v did not match expected %v", tc.name, resp, tc.expectedResponse)

				// The following is related to the workaround above, q.v.
				if getRouterImage() != defaultRouterImage {
					t.Errorf("You may need to add an entry to /etc/hosts so that the"+
						" hostname of the router (%s) resolves its the IP address, (%s).",
						tc.routeAlias, routeAddress)
				}
			} else {
				//good to go, stop trying
				break
			}
		}

		//clean up
		routeEvent.Type = watch.Deleted
		endpointEvent.Type = watch.Modified
		endpoints := endpointEvent.Object.(*kapi.Endpoints)
		endpoints.Subsets = []kapi.EndpointSubset{}

		fakeMasterAndPod.EndpointChannel <- eventString(endpointEvent)
		fakeMasterAndPod.RouteChannel <- eventString(routeEvent)
	}

	// Give the router some time to finish processing events before we kill it.
	time.Sleep(time.Second * 5)
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

	routeAddress := getRouteAddress()
	routerTestAddress := fmt.Sprintf("%s/test", routeAddress)

	fakeMasterAndPod.EndpointChannel <- eventString(endpointEvent)
	fakeMasterAndPod.RouteChannel <- eventString(routeEvent)
	time.Sleep(time.Second * tcWaitSeconds)
	//ensure you can curl path but not main host
	validateRoute(routerTestAddress, "www.example.com", "http", tr.HelloPodPath, t)

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
	validateRoute(routerTestAddress, "www.example.com", "http", tr.HelloPodPath, t)
	validateRoute(routeAddress, "www.example.com", "http", tr.HelloPod, t)

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
	// Ensure you can still curl path and host.  The host-based route should now
	// handle requests to / as well as requests to /test (or any other path).
	// Note, however, that the host-based route and the host-based route use the
	// same service, and that that service varies its response in accordance with
	// the path, so we still get the tr.HelloPodPath response when we request
	// /test even though we request using routeAddress.
	validateRoute(routerTestAddress, "www.example.com", "http", tr.HelloPodPath, t)
	validateRoute(routeAddress, "www.example.com", "http", tr.HelloPod, t)

	// Clean up the host-based route and endpoint.
	routeEvent = &watch.Event{
		Type: watch.Deleted,
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
	endpointEvent = &watch.Event{
		Type: watch.Modified,
		Object: &kapi.Endpoints{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "myService",
				Namespace: "default",
			},
			Subsets: []kapi.EndpointSubset{},
		},
	}
	fakeMasterAndPod.EndpointChannel <- eventString(endpointEvent)

	time.Sleep(time.Second * 5)
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

	routeAddress := getRouteAddress()

	var examplePass, example2Pass bool
	var exampleResp, example2Resp string
	for i := 0; i < tcRetries; i++ {
		//ensure you can curl both
		examplePass, exampleResp = isValidRoute(routeAddress, "www.example.com", "http", tr.HelloPod)
		example2Pass, example2Resp = isValidRoute(routeAddress, "www.example2.com", "http", tr.HelloPod)

		if examplePass && example2Pass {
			break
		}
		//not valid yet, give it some more time before failing
		time.Sleep(time.Second * tcWaitSeconds)
	}

	if !examplePass || !example2Pass {
		t.Errorf("Unable to validate both routes in a duplicate service scenario.  Resp 1: %s, Resp 2: %s", exampleResp, example2Resp)
	}

	// Clean up the endpoint and routes.
	example2RouteCleanupEvent := &watch.Event{
		Type: watch.Deleted,
		Object: &routeapi.Route{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "example2",
				Namespace: "default",
			},
			Host:        "www.example2.com",
			ServiceName: "myService",
		},
	}
	fakeMasterAndPod.RouteChannel <- eventString(example2RouteCleanupEvent)
	exampleRouteCleanupEvent := &watch.Event{
		Type: watch.Deleted,
		Object: &routeapi.Route{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "example",
				Namespace: "default",
			},
			Host:        "www.example.com",
			ServiceName: "myService",
		},
	}
	fakeMasterAndPod.RouteChannel <- eventString(exampleRouteCleanupEvent)
	endpointCleanupEvent := &watch.Event{
		Type: watch.Modified,
		Object: &kapi.Endpoints{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "myService",
				Namespace: "default",
			},
			Subsets: []kapi.EndpointSubset{},
		},
	}
	fakeMasterAndPod.EndpointChannel <- eventString(endpointCleanupEvent)

	time.Sleep(time.Second * 5)
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

// getRoute is a utility function for making the web request to a route.
// Protocol is one of http, https, ws, or wss.  If the protocol is https or wss,
// then getRoute will make a secure transport client with InsecureSkipVerify:
// true.  If the protocol is http or ws, then getRoute does an unencrypted HTTP
// client request.  If the protocol is ws or wss, then getRoute will upgrade the
// connection to websockets and then send expectedResponse *to* the route, with
// the expectation that the route will echo back what it receives.  Note that
// getRoute returns only the first len(expectedResponse) bytes of the actual
// response.
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
		origin := fmt.Sprintf("http://%s/", tr.GetDefaultLocalAddress())
		wsConfig, err := websocket.NewConfig(url, origin)
		if err != nil {
			return "", err
		}

		port := 80
		if protocol == "wss" {
			port = 443
		}
		wsConfig.Location.Host = fmt.Sprintf("%s:%d", hostName, port)
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

	copyEnv := []string{
		"ROUTER_EXTERNAL_HOST_HOSTNAME",
		"ROUTER_EXTERNAL_HOST_USERNAME",
		"ROUTER_EXTERNAL_HOST_PASSWORD",
		"ROUTER_EXTERNAL_HOST_HTTP_VSERVER",
		"ROUTER_EXTERNAL_HOST_HTTPS_VSERVER",
		"ROUTER_EXTERNAL_HOST_INSECURE",
		"ROUTER_EXTERNAL_HOST_PRIVKEY",
	}

	env := []string{}

	for _, name := range copyEnv {
		val := os.Getenv(name)
		if len(val) > 0 {
			env = append(env, name+"="+val)
		}
	}

	vols := ""
	hostVols := []string{}

	privkeyFilename := os.Getenv("ROUTER_EXTERNAL_HOST_PRIVKEY")
	if len(privkeyFilename) != 0 {
		vols = privkeyFilename
		privkeyBindmount := fmt.Sprintf("%[1]s:%[1]s", privkeyFilename)
		hostVols = append(hostVols, privkeyBindmount)
	}

	containerOpts := dockerClient.CreateContainerOptions{
		Config: &dockerClient.Config{
			Image:        getRouterImage(),
			Cmd:          []string{"--master=" + masterIp, "--loglevel=4"},
			Env:          env,
			ExposedPorts: exposedPorts,
			VolumesFrom:  vols,
		},
		HostConfig: &dockerClient.HostConfig{
			Binds: hostVols,
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
		time.Sleep(time.Second * dockerWaitSeconds)

		c, err := dockerCli.InspectContainer(container.ID)

		if err != nil {
			return "", err
		}

		if c.State.Running {
			running = true
			break
		}
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

// getRouteAddress checks for the OPENSHIFT_ROUTE_ADDRESS environment
// variable and returns it if it set and non-empty; otherwise it returns
// "0.0.0.0".
func getRouteAddress() string {
	addr := os.Getenv("OPENSHIFT_ROUTE_ADDRESS")

	if len(addr) == 0 {
		addr = "0.0.0.0"
	}

	return addr
}
