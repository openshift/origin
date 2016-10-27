package router

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"

	"github.com/golang/glog"
	"golang.org/x/net/websocket"

	"github.com/openshift/origin/pkg/cmd/util"
)

// GetDefaultLocalAddress returns an address at which the local host can
// be reached, or 0.0.0.0 (which should work for locations from the host
// to itself) if the actual default local address cannot be determined.
func GetDefaultLocalAddress() string {
	addr := "0.0.0.0"
	if a := os.Getenv("OPENSHIFT_ROUTER_SERVER_ADDRESS"); len(a) > 0 {
		return a
	}
	ip, err := util.DefaultLocalIP4()
	if err == nil {
		addr = ip.String()
	}

	return addr
}

// NewTestHttpServer creates a new TestHttpService using default locations for listening address
// as well as default certificates.  New channels will be initialized which can be used by test clients
// to feed events through the server to anything listening.
func NewTestHttpService() *TestHttpService {
	endpointChannel := make(chan string)
	routeChannel := make(chan string)
	nodeChannel := make(chan string)
	svcChannel := make(chan string)

	addr := GetDefaultLocalAddress()

	masterHttpAddr := fmt.Sprintf("%s:8080", addr)
	podHttpAddr := fmt.Sprintf("%s:8888", addr)
	alternatePodHttpAddr := fmt.Sprintf("%s:8889", addr)
	podHttpsAddr := fmt.Sprintf("%s:8443", addr)

	return &TestHttpService{
		MasterHttpAddr:       masterHttpAddr,
		PodHttpAddr:          podHttpAddr,
		AlternatePodHttpAddr: alternatePodHttpAddr,
		PodHttpsAddr:         podHttpsAddr,
		PodTestPath:          "test",
		PodHttpsCert:         []byte(Example2Cert),
		PodHttpsKey:          []byte(Example2Key),
		PodHttpsCaCert:       []byte(ExampleCACert),
		EndpointChannel:      endpointChannel,
		RouteChannel:         routeChannel,
		NodeChannel:          nodeChannel,
		SvcChannel:           svcChannel,
	}
}

// TestHttpService is a service that simulates a master k8s server for the router.  It provides endpoints that
// a router running in docker can attach to for endpoint watches and route watches.  It also simulates a client
// application so that routes can have a destination.
//
// Two channels are provided to simulate watch events: EndpointChannel and RouteChannel.  Use these channels in
// you test cases to feed information to the router that would normally come from client CRUD actions.
//
// List events will return empty data for all calls.
type TestHttpService struct {
	MasterHttpAddr       string
	PodHttpAddr          string
	AlternatePodHttpAddr string
	PodHttpsAddr         string
	PodHttpsCert         []byte
	PodHttpsKey          []byte
	PodHttpsCaCert       []byte
	PodTestPath          string
	EndpointChannel      chan string
	RouteChannel         chan string
	NodeChannel          chan string
	SvcChannel           chan string

	listeners []net.Listener
}

const (
	// HelloPod is the expected response to a call to PodHttpAddr (usually called through a route)
	HelloPod          = "Hello Pod!"
	HelloPodAlternate = "Alternate Hello Pod!"
	// HelloPod is the expected response to a call to PodHttpAddr (usually called through a route)
	HelloPodPath = "Hello Pod Path!"
	// HelloPodSecure is the expected response to a call to PodHttpsAddr (usually called through a route)
	HelloPodSecure = "Hello Pod Secure!"
	// HelloPodPathSecure is the expected response to a call to PodHttpsAddr (usually called through a route)
	HelloPodPathSecure = "Hello Pod Path Secure!"
)

type TestHttpSocketService struct {
	ServeMux         *http.ServeMux
	WebSocketHandler websocket.Handler
}

func (s *TestHttpSocketService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Upgrade") == "websocket" {
		s.WebSocketHandler.ServeHTTP(w, r)
	} else {
		s.ServeMux.ServeHTTP(w, r)
	}
}

// handleHelloPod handles calls to PodHttpAddr (usually called through a route)
func (s *TestHttpService) handleHelloPod(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, HelloPod)
}

// handleHelloPod handles calls to PodHttpAddr (usually called through a route)
func (s *TestHttpService) handleHelloPod2(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, HelloPodAlternate)
}

// handleHelloPodTest handles calls to PodHttpAddr (usually called through a route) with the /test/ path
func (s *TestHttpService) handleHelloPodTest(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, HelloPodPath)
}

// handleHelloPodSecure handles calls to PodHttpsAddr (usually called through a route)
func (s *TestHttpService) handleHelloPodSecure(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, HelloPodSecure)
}

// handleHelloPodTestSecure handles calls to PodHttpsAddr (usually called through a route) with the /test/ path
func (s *TestHttpService) handleHelloPodTestSecure(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, HelloPodPathSecure)
}

// handleSvcList handles calls to /api/v1beta1/services and always returns empty data
func (s *TestHttpService) handleSvcList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, "{}")
}

// handleSvcWatch handles calls to /api/v1beta1/watch/services and uses the svc channel to simulate watch events
func (s *TestHttpService) handleSvcWatch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, <-s.SvcChannel)
}

// handleNodeList handles calls to /api/v1beta1/nodes and always returns empty data
func (s *TestHttpService) handleNodeList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, "{}")
}

// handleNodeWatch handles calls to /api/v1beta1/watch/nodes and uses the node channel to simulate watch events
func (s *TestHttpService) handleNodeWatch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, <-s.NodeChannel)
}

// handleRouteWatch handles calls to /osapi/v1beta1/watch/routes and uses the route channel to simulate watch events
func (s *TestHttpService) handleRouteWatch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, <-s.RouteChannel)
}

// handleRouteList handles calls to /osapi/v1beta1/routes and always returns empty data
func (s *TestHttpService) handleRouteList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, "{}")
}

// handleRouteCalls handles calls to /osapi/v1/routes/* and returns whatever the client sent
func (s *TestHttpService) handleRouteCalls(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, "{}")
}

// handleEndpointWatch handles calls to /api/v1beta1/watch/endpoints and uses the endpoint channel to simulate watch events
func (s *TestHttpService) handleEndpointWatch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, <-s.EndpointChannel)
}

// handleEndpointList handles calls to /api/v1beta1/endpoints and always returns empty data
func (s *TestHttpService) handleEndpointList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, "{}")
}

// handleWebSocket copies whatever is written to the web socket back to the socket
func (s *TestHttpService) handleWebSocket(ws *websocket.Conn) {
	_, err := io.Copy(ws, ws)

	if err != nil {
		panic(err)
	}
}

// Stop stops the service by closing any registered listeners
func (s *TestHttpService) Stop() {
	if s.listeners != nil && len(s.listeners) > 0 {
		for _, l := range s.listeners {
			if l != nil {
				glog.Infof("Stopping listener at %s\n", l.Addr().String())
				l.Close()
			}
		}
	}
}

// Start will start the http service to simulate the master and client urls.  It sets up the appropriate watch
// endpoints and serves the secure and non-secure traffic.
func (s *TestHttpService) Start() error {
	s.listeners = make([]net.Listener, 3)

	if err := s.startMaster(); err != nil {
		return err
	}

	if err := s.startPod(); err != nil {
		return err
	}

	return nil
}

func (s *TestHttpService) startMaster() error {
	masterServer := http.NewServeMux()
	apis := []string{"v1"}

	for _, version := range apis {
		masterServer.HandleFunc(fmt.Sprintf("/api/%s/endpoints", version), s.handleEndpointList)
		masterServer.HandleFunc(fmt.Sprintf("/api/%s/watch/endpoints", version), s.handleEndpointWatch)
		masterServer.HandleFunc(fmt.Sprintf("/oapi/%s/routes", version), s.handleRouteList)
		masterServer.HandleFunc(fmt.Sprintf("/oapi/%s/namespaces/", version), s.handleRouteCalls)
		masterServer.HandleFunc(fmt.Sprintf("/oapi/%s/watch/routes", version), s.handleRouteWatch)
		masterServer.HandleFunc(fmt.Sprintf("/api/%s/nodes", version), s.handleNodeList)
		masterServer.HandleFunc(fmt.Sprintf("/api/%s/watch/nodes", version), s.handleNodeWatch)
		masterServer.HandleFunc(fmt.Sprintf("/api/%s/services", version), s.handleSvcList)
		masterServer.HandleFunc(fmt.Sprintf("/api/%s/watch/services", version), s.handleSvcWatch)
	}

	if err := s.startServing(s.MasterHttpAddr, http.Handler(masterServer)); err != nil {
		return err
	}

	return nil
}

func (s *TestHttpService) startPod() error {
	unsecurePodServeMux := http.NewServeMux()
	unsecurePodServeMux.HandleFunc("/", s.handleHelloPod)
	unsecurePodServeMux.HandleFunc("/"+s.PodTestPath, s.handleHelloPodTest)
	unsecurePodServer := &TestHttpSocketService{
		ServeMux:         unsecurePodServeMux,
		WebSocketHandler: websocket.Handler(s.handleWebSocket),
	}

	if err := s.startServing(s.PodHttpAddr, http.Handler(unsecurePodServer)); err != nil {
		return err
	}

	alternatePodServeMux := http.NewServeMux()
	alternatePodServeMux.HandleFunc("/", s.handleHelloPod2)
	alternatePodServeMux.HandleFunc("/"+s.PodTestPath, s.handleHelloPod2)
	alternatePodServer := &TestHttpSocketService{
		ServeMux:         alternatePodServeMux,
		WebSocketHandler: websocket.Handler(s.handleWebSocket),
	}

	if err := s.startServing(s.AlternatePodHttpAddr, http.Handler(alternatePodServer)); err != nil {
		return err
	}

	securePodServeMux := http.NewServeMux()
	securePodServeMux.HandleFunc("/", s.handleHelloPodSecure)
	securePodServeMux.HandleFunc("/"+s.PodTestPath, s.handleHelloPodTestSecure)
	securePodServer := &TestHttpSocketService{
		ServeMux:         securePodServeMux,
		WebSocketHandler: websocket.Handler(s.handleWebSocket),
	}
	if err := s.startServingTLS(s.PodHttpsAddr, s.PodHttpsCert, s.PodHttpsKey, s.PodHttpsCaCert, http.Handler(securePodServer)); err != nil {
		return err
	}

	return nil
}

// startServing creates and registers a non-secure listener and begins serving traffic
func (s *TestHttpService) startServing(addr string, handler http.Handler) error {
	listener, err := net.Listen("tcp", addr)

	if err != nil {
		return err
	}

	s.listeners = append(s.listeners, listener)

	glog.Infof("Started, serving at %s\n", listener.Addr().String())

	go func() {
		err := http.Serve(listener, handler)

		if err != nil {
			glog.Errorf("HTTP server failed: %v", err)
		}
	}()

	return nil
}

// startServingTLS creates and registers a secure listener and begins serving traffic.
func (s *TestHttpService) startServingTLS(addr string, cert []byte, key []byte, caCert []byte, handler http.Handler) error {
	tlsCert, err := tls.X509KeyPair(append(cert, caCert...), key)

	if err != nil {
		return err
	}

	cfg := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
	}

	listener, err := tls.Listen("tcp", addr, cfg)

	if err != nil {
		return err
	}

	s.listeners = append(s.listeners, listener)
	glog.Infof("Started, serving TLS at %s\n", listener.Addr().String())

	go func() {
		if err := http.Serve(listener, handler); err != nil {
			glog.Errorf("HTTPS server failed: %v", err)
		}
	}()

	return nil
}
