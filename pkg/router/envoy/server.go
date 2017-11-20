package envoy

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/cmux"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/duration"
	st "github.com/golang/protobuf/ptypes/struct"
	"google.golang.org/grpc"

	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/openshift/origin/pkg/router/envoy/api"
)

// Serve launches a gRPC endpoint on the provided listener ta accept ADS requests from Envoy.
func (p *Plugin) Serve(listener net.Listener) error {
	grpcServer := grpc.NewServer()

	runtime.SetBlockProfileRate(int(100 * time.Millisecond.Nanoseconds()))
	runtime.SetMutexProfileFraction(int(100 * time.Millisecond.Nanoseconds()))

	s := newServer(p)
	api.RegisterAggregatedDiscoveryServiceServer(grpcServer, s)

	m := cmux.New(listener)
	grpcl := m.Match(cmux.HTTP2HeaderField("content-type", "application/grpc"))
	httpl := m.Match(cmux.HTTP1Fast())

	go func(l net.Listener) {
		if err := grpcServer.Serve(l); err != cmux.ErrListenerClosed {
			panic(err)
		}
	}(grpcl)
	go func(l net.Listener) {
		http.Handle("/metrics", prometheus.Handler())
		if err := http.Serve(l, nil); err != cmux.ErrListenerClosed {
			panic(err)
		}
	}(httpl)

	go s.Run(wait.NeverStop)

	if err := m.Serve(); !strings.Contains(err.Error(), "use of closed network connection") {
		return err
	}
	return nil
}

type handlerFunc func(req *api.DiscoveryRequest) (resources []proto.Message, version string, queue bool, err error)

type server struct {
	plugin   *Plugin
	handlers map[string]handlerFunc

	lock         sync.Mutex
	notifiers    []chan ChangeMap
	lastVersions versions
}

var _ api.AggregatedDiscoveryServiceServer = &server{}

const (
	envoyApiCluster               = "type.googleapis.com/envoy.api.v2.Cluster"
	envoyApiClusterLoadAssignment = "type.googleapis.com/envoy.api.v2.ClusterLoadAssignment"
	envoyApiListener              = "type.googleapis.com/envoy.api.v2.Listener"
	envoyApiRouteConfiguration    = "type.googleapis.com/envoy.api.v2.RouteConfiguration"
)

func newServer(p *Plugin) *server {
	s := &server{
		plugin: p,
	}
	s.handlers = map[string]handlerFunc{
		envoyApiCluster:               s.handleCluster,
		envoyApiClusterLoadAssignment: s.handleClusterLoadAssignment,
		envoyApiListener:              s.handleListener,
		envoyApiRouteConfiguration:    s.handleRouteConfiguration,
	}
	return s
}

func (s *server) StreamAggregatedResources(stream api.AggregatedDiscoveryService_StreamAggregatedResourcesServer) error {
	work := make(chan *api.DiscoveryRequest, 10)
	conn := s.newConn(stream)
	defer conn.Close()

	go func() {
		pending := newPendingRequests()
		for {
			select {
			case changes, ok := <-conn.updates:
				if !ok {
					return
				}
				for _, req := range pending.wake(changes) {
					if err := conn.respond(req, pending); err != nil {
						utilruntime.HandleError(err)
						// TODO: close main loop?
					}
				}
			case req := <-work:
				if err := conn.respond(req, pending); err != nil {
					utilruntime.HandleError(err)
					// TODO: close main loop?
				}
			}
		}
	}()

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			utilruntime.HandleError(err)
			return err
		}

		log.Printf("> req: %s (%v) @ %s", req.TypeUrl, req.ResourceNames, req.VersionInfo)

		if !conn.validateNonce(req) {
			log.Printf("  req: %s (%v) @ %s IGNORED, nonce expired", req.TypeUrl, req.ResourceNames, req.VersionInfo)
			continue
		}

		work <- req
	}
}

func (s *server) Run(stopCh <-chan struct{}) {
	go func() {
		ch := s.plugin.updates.Wait()
		for {
			select {
			case <-stopCh:
				return
			case <-ch:
				// TODO: instead of getting them all at once, we could delay a bit to see if more come in
				// so that we are batching changes
				changes := s.plugin.updates.Changes()
				s.lock.Lock()
				for _, ch := range s.notifiers {
					select {
					case ch <- changes:
					}
				}
				s.lock.Unlock()
			}
		}
	}()
}

func (s *server) handleCluster(req *api.DiscoveryRequest) (resources []proto.Message, versionInfo string, queue bool, err error) {
	p := s.plugin
	latest := strconv.FormatInt(p.getVersions().route, 10)
	if req.VersionInfo == latest {
		return nil, "", true, nil
	}

	// load initial state, then remember version info?
	log.Printf("  Returning all clusters: %s (%v) @ %s", req.TypeUrl, req.ResourceNames, req.VersionInfo)
	routes, version := p.listRoutes()
	for _, route := range routes {
		resources = append(resources, &api.Cluster{
			Name:           fmt.Sprintf("%s_%s", route.Namespace, route.Name),
			ConnectTimeout: &duration.Duration{Seconds: 30},
			Type:           api.Cluster_EDS,
			ProtocolOptions: &api.Cluster_HttpProtocolOptions{
				HttpProtocolOptions: &api.Http1ProtocolOptions{},
			},
			EdsClusterConfig: &api.Cluster_EdsClusterConfig{
				EdsConfig: &api.ConfigSource{
					ConfigSourceSpecifier: &api.ConfigSource_Ads{
						Ads: &api.AggregatedConfigSource{},
					},
				},
			},
		})
	}
	versionInfo = strconv.FormatInt(version, 10)
	return resources, versionInfo, false, nil
}

func (s *server) handleClusterLoadAssignment(req *api.DiscoveryRequest) (resources []proto.Message, versionInfo string, queue bool, err error) {
	p := s.plugin
	latest := strconv.FormatInt(p.getVersions().combinedEndpoints(), 10)
	if req.VersionInfo == latest {
		return nil, "", true, nil
	}

	// load initial state, then remember version info?
	log.Printf("  Returning cluster load assignments: %s (%v) @ %s", req.TypeUrl, req.ResourceNames, req.VersionInfo)
	endpoints, version := p.listEndpoints(true, req.ResourceNames...)
	for _, endpoint := range endpoints {
		cla := &api.ClusterLoadAssignment{
			ClusterName: fmt.Sprintf("%s_%s", endpoint.Route.Namespace, endpoint.Route.Name),
			Policy:      &api.ClusterLoadAssignment_Policy{},
			Endpoints:   []*api.LocalityLbEndpoints{},
		}
		for _, ept := range endpoint.Endpoints {
			lb := &api.LocalityLbEndpoints{
				Locality:    &api.Locality{},
				LbEndpoints: []*api.LbEndpoint{},
			}
			for _, subset := range ept.Subsets {
				switch p := endpoint.Route.Spec.Port; {
				case p == nil:
					for _, port := range subset.Ports {
						for _, addr := range subset.Addresses {
							lb.LbEndpoints = append(lb.LbEndpoints, &api.LbEndpoint{
								Endpoint: &api.Endpoint{
									Address: &api.Address{
										Address: &api.Address_SocketAddress{
											SocketAddress: &api.SocketAddress{
												Protocol: api.SocketAddress_TCP,
												Address:  addr.IP,
												PortSpecifier: &api.SocketAddress_PortValue{
													PortValue: uint32(port.Port),
												},
											},
										},
									},
								},
							})
						}
					}
				case p.TargetPort.Type == intstr.String:
					var targetPort int32
					for _, port := range subset.Ports {
						if port.Name == p.TargetPort.StrVal {
							targetPort = port.Port
						}
					}
					if targetPort == 0 {
						continue
					}
					for _, addr := range subset.Addresses {
						lb.LbEndpoints = append(lb.LbEndpoints, &api.LbEndpoint{
							Endpoint: &api.Endpoint{
								Address: &api.Address{
									Address: &api.Address_SocketAddress{
										SocketAddress: &api.SocketAddress{
											Protocol: api.SocketAddress_TCP,
											Address:  addr.IP,
											PortSpecifier: &api.SocketAddress_PortValue{
												PortValue: uint32(targetPort),
											},
										},
									},
								},
							},
						})
					}
				case p.TargetPort.Type == intstr.Int:
					for _, addr := range subset.Addresses {
						lb.LbEndpoints = append(lb.LbEndpoints, &api.LbEndpoint{
							Endpoint: &api.Endpoint{
								Address: &api.Address{
									Address: &api.Address_SocketAddress{
										SocketAddress: &api.SocketAddress{
											Protocol: api.SocketAddress_TCP,
											Address:  addr.IP,
											PortSpecifier: &api.SocketAddress_PortValue{
												PortValue: uint32(p.TargetPort.IntVal),
											},
										},
									},
								},
							},
						})
					}
				}
			}
			cla.Endpoints = append(cla.Endpoints, lb)
		}
		resources = append(resources, cla)
	}
	versionInfo = strconv.FormatInt(version, 10)
	return resources, versionInfo, false, nil
}

func (s *server) handleRouteConfiguration(req *api.DiscoveryRequest) (resources []proto.Message, versionInfo string, queue bool, err error) {
	p := s.plugin
	latest := strconv.FormatInt(p.getVersions().route, 10)
	if req.VersionInfo == latest {
		return nil, "", true, nil
	}

	log.Printf("  Returning all route configuration: %s (%v) @ %s", req.TypeUrl, req.ResourceNames, req.VersionInfo)
	routes, version := p.listRoutes()
	config := &api.RouteConfiguration{
		Name:         "openshift_http",
		VirtualHosts: []*api.VirtualHost{},
	}
	for _, route := range routes {
		name := fmt.Sprintf("%s_%s", route.Namespace, route.Name)
		config.VirtualHosts = append(config.VirtualHosts, &api.VirtualHost{
			Name:    name,
			Domains: []string{route.Spec.Host},
			Routes: []*api.Route{
				{
					Match: &api.RouteMatch{
						PathSpecifier: &api.RouteMatch_Prefix{},
					},
					Action: &api.Route_Route{
						Route: &api.RouteAction{
							ClusterSpecifier: &api.RouteAction_Cluster{
								Cluster: name,
							},
						},
					},
				},
			},
		})
	}
	resources = append(resources, config)
	versionInfo = strconv.FormatInt(version, 10)
	return resources, versionInfo, false, nil
}

func (s *server) handleListener(req *api.DiscoveryRequest) (resources []proto.Message, versionInfo string, queue bool, err error) {
	latest := "1"
	if req.VersionInfo == latest {
		return nil, "", true, nil
	}

	log.Printf("Returning all listeners: %s (%v) @ %s", req.TypeUrl, req.ResourceNames, req.VersionInfo)
	resources = append(resources,
		&api.Listener{
			Name:     "http",
			Metadata: &api.Metadata{},
			FilterChains: []*api.FilterChain{
				{
					FilterChainMatch: &api.FilterChainMatch{},
					Filters: []*api.Filter{
						{
							Name: "envoy.http_connection_manager",
							Config: &st.Struct{Fields: pm{
								"codec_type":  pstring("AUTO"),
								"stat_prefix": pstring("openshift_http"),
								"rds": pstruct(pm{
									"route_config_name": pstring("openshift_http"),
									"config_source": pstruct(pm{
										"ads": pstruct(pm{}),
									}),
								}),
								"http_filters": plist(pa{
									pstruct(pm{
										"name":   pstring("envoy.router"),
										"config": pstruct(pm{}),
									}),
								}),
							}},
						},
					},
				},
			},
			Address: &api.Address{
				Address: &api.Address_SocketAddress{
					&api.SocketAddress{
						Protocol: api.SocketAddress_TCP,
						Address:  "0.0.0.0",
						PortSpecifier: &api.SocketAddress_PortValue{
							PortValue: 80,
						},
					},
				},
			},
		},
	)
	versionInfo = latest
	return resources, versionInfo, false, nil
}

type conn struct {
	stream   api.AggregatedDiscoveryService_StreamAggregatedResourcesServer
	handlers map[string]handlerFunc
	updates  <-chan ChangeMap
	closeFn  func()

	lock   sync.Mutex
	nonces map[string]int
}

func (s *server) newConn(stream api.AggregatedDiscoveryService_StreamAggregatedResourcesServer) *conn {
	ch := make(chan ChangeMap, 2)
	closeFn := func() {
		s.lock.Lock()
		defer s.lock.Unlock()
		for i, existing := range s.notifiers {
			if existing == ch {
				updated := make([]chan ChangeMap, 0, len(s.notifiers)-1)
				copy(updated, s.notifiers[:i])
				copy(updated[i:], s.notifiers[i+1:])
				s.notifiers = updated
				go func() {
					// drain the channel
					for range existing {
					}
				}()
				break
			}
		}
		close(ch)
	}

	s.lock.Lock()
	defer s.lock.Unlock()
	s.notifiers = append(s.notifiers, ch)
	return &conn{
		stream:   stream,
		handlers: s.handlers,
		updates:  ch,
		closeFn:  closeFn,
		nonces:   make(map[string]int),
	}
}

func (c *conn) Close() {
	c.closeFn()
}

func (c *conn) validateNonce(req *api.DiscoveryRequest) bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	nonce, ok := c.nonces[req.TypeUrl]
	if !ok {
		return true
	}
	if len(req.ResponseNonce) > 0 && req.ResponseNonce != strconv.Itoa(nonce) {
		return false
	}
	return true
}

func (c *conn) nextNonce(req *api.DiscoveryRequest) string {
	c.lock.Lock()
	defer c.lock.Unlock()
	nonce := c.nonces[req.TypeUrl]
	nonce++
	c.nonces[req.TypeUrl] = nonce
	return strconv.Itoa(nonce)
}

func (c *conn) respond(req *api.DiscoveryRequest, pending *pendingRequests) error {
	fn, ok := c.handlers[req.TypeUrl]
	if !ok {
		log.Printf("  req: %s (%v) @ %s IGNORED", req.TypeUrl, req.ResourceNames, req.VersionInfo)
		return nil
	}

	resources, versionInfo, queue, err := fn(req)
	if err != nil {
		return fmt.Errorf("unable to handle discovery request: %v", err)
	}
	if queue {
		pending.wait(req)
		return nil
	}

	nonce := c.nextNonce(req)
	res := &api.DiscoveryResponse{
		VersionInfo: versionInfo,
		TypeUrl:     req.TypeUrl,
		Nonce:       nonce,
	}

	for _, resource := range resources {
		data, err := proto.Marshal(resource)
		if err != nil {
			return err
		}
		res.Resources = append(res.Resources, &any.Any{
			TypeUrl: req.TypeUrl,
			Value:   data,
		})
	}

	if err := c.stream.Send(res); err != nil {
		return err
	}
	log.Printf("< req: %s (%v) @ %s N=%s", req.TypeUrl, req.ResourceNames, req.VersionInfo, res.Nonce)
	return nil
}

type pm map[string]*st.Value

func pstruct(m map[string]*st.Value) *st.Value {
	return &st.Value{Kind: &st.Value_StructValue{StructValue: &st.Struct{Fields: m}}}
}

type pa []*st.Value

func plist(m []*st.Value) *st.Value {
	return &st.Value{Kind: &st.Value_ListValue{ListValue: &st.ListValue{Values: m}}}
}

func pstring(m string) *st.Value {
	return &st.Value{Kind: &st.Value_StringValue{StringValue: m}}
}

type ChangeMap map[string]map[string]struct{}

type updates struct {
	lock    sync.Mutex
	ch      chan struct{}
	updates ChangeMap
}

func newUpdates() *updates {
	return &updates{
		ch:      make(chan struct{}, 1),
		updates: make(ChangeMap),
	}
}

func (u *updates) Close() {
}

func (u *updates) Wait() <-chan struct{} {
	return u.ch
}

func (u *updates) Changes() ChangeMap {
	u.lock.Lock()
	defer u.lock.Unlock()
	m := u.updates
	u.updates = make(ChangeMap)
	return m
}

func (u *updates) Notify(typeUrl string, resources ...string) {
	u.lock.Lock()
	defer func() {
		u.lock.Unlock()
		select {
		case u.ch <- struct{}{}:
		default:
		}
	}()
	m := u.updates[typeUrl]
	if m == nil {
		m = make(map[string]struct{})
		u.updates[typeUrl] = m
	}
	if len(resources) == 0 {
		m[""] = struct{}{}
		return
	}
	for _, resource := range resources {
		m[resource] = struct{}{}
	}
}

type pendingRequests struct {
	waiting map[string]map[string]*api.DiscoveryRequest
}

func newPendingRequests() *pendingRequests {
	return &pendingRequests{
		waiting: make(map[string]map[string]*api.DiscoveryRequest),
	}
}

func (r *pendingRequests) wake(changes ChangeMap) []*api.DiscoveryRequest {
	var reqs []*api.DiscoveryRequest

	for typeUrl, names := range changes {
		m := r.waiting[typeUrl]
		if req, ok := m[""]; ok {
			reqs = append(reqs, req)
			delete(m, "")
		} else {
			var bulk *api.DiscoveryRequest
			for name := range names {
				if req, ok := m[name]; ok {
					if bulk == nil {
						bulk = req
						bulk.ResourceNames = make([]string, 0, len(names))
						reqs = append(reqs, bulk)
					}
					bulk.ResourceNames = append(bulk.ResourceNames, name)
					delete(m, name)
				}
			}
		}
	}
	if len(reqs) > 0 {
		log.Printf("WAKE: %s triggered %d", changes, len(reqs))
	}
	return reqs
}

func (r *pendingRequests) wait(req *api.DiscoveryRequest) {
	if len(req.ResourceNames) == 0 {
		log.Printf("  req: %s (%v) @ %s QUEUED", req.TypeUrl, req.ResourceNames, req.VersionInfo)
		r.waiting[req.TypeUrl] = map[string]*api.DiscoveryRequest{"": req}
		return
	}
	existing := r.waiting[req.TypeUrl]
	if _, ok := existing[""]; ok {
		log.Printf("  req: %s (%v) @ %s IGNORED: already queued, ", req.TypeUrl, req.ResourceNames, req.VersionInfo)
		return
	}
	log.Printf("  req: %s (%v) @ %s QUEUED", req.TypeUrl, req.ResourceNames, req.VersionInfo)
	if existing == nil {
		existing = make(map[string]*api.DiscoveryRequest)
		r.waiting[req.TypeUrl] = existing
	}
	for _, name := range req.ResourceNames {
		existing[name] = req
	}
}
