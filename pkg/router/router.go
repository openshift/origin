package router

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sort"
	"strconv"
	"time"

	"github.com/golang/glog"
)

const (
	ProtocolHTTP  = "http"
	ProtocolHTTPS = "https"
	ProtocolTLS   = "tls"
)

const (
	TermEdge  = "TERM_EDGE"
	TermGear  = "TERM_GEAR"
	TermRessl = "TERM_RESSL"
)

const (
	DefaultRouteFile = "/var/lib/containers/router/routes.json"
)

type Frontend struct {
	Name          string
	HostAliases   []string
	Backends      map[string]Backend
	EndpointTable map[string]Endpoint
}

type Backend struct {
	ID           string
	FePath       string
	BePath       string
	Protocols    []string
	EndpointIDs  []string
	SslTerm      string
	Certificates []Certificate
}

type Certificate struct {
	ID                 string
	Contents           []byte
	PrivateKey         []byte
	PrivateKeyPassword string
}

type Endpoint struct {
	ID   string
	IP   string
	Port string
}

type Routes struct {
	GlobalRoutes map[string]Frontend
	RouteFile    string
}

type Router interface {
	ReadRoutes() error
	WriteRoutes() error
	FindFrontend(name string) (v Frontend, ok bool)
	DeleteBackends(name string)
	CreateFrontend(name string, url string) (*Frontend, error)
	DeleteFrontend(frontendName string) error
	AddAlias(alias string, frontendName string) error
	RemoveAlias(alias string, frontendName string) error
	AddRoute(frontend *Frontend, backend *Backend, endpoints []Endpoint) error
	WriteConfig()
	ReloadRouter() bool
}

func NewRoutes(filename ...string) *Routes {
	file := DefaultRouteFile
	if filename != nil && len(filename) > 0 {
		file = filename[0]
	}
	return &Routes{make(map[string]Frontend), file}
}

func makeID() string {
	var s string
	s = strconv.FormatInt(time.Now().UnixNano(), 16)
	return s
}

func (routes *Routes) ReadRoutes() error {
	file := routes.RouteFile
	glog.V(4).Infof("Reading routes file (%s)\n", file)
	dat, err := ioutil.ReadFile(file)
	if err != nil {
		glog.Errorf("Error while reading file %v (%s)", file, err)
		routes.GlobalRoutes = make(map[string]Frontend)
		return err
	}
	json.Unmarshal(dat, &routes.GlobalRoutes)
	glog.V(4).Infof("Marshall result %+v", routes.GlobalRoutes)
	return nil
}

func (routes *Routes) WriteRoutes() error {
	dat, err := json.MarshalIndent(routes.GlobalRoutes, "", "  ")
	if err != nil {
		glog.Errorf("Failed to marshal routes - %s", err.Error())
		return err
	}
	file := routes.RouteFile
	glog.V(4).Infof("Writing routes tofile (%s)", file)
	err = ioutil.WriteFile(file, dat, 0644)
	if err != nil {
		glog.Errorf("Failed to write to routes file - %s", err.Error())
	}
	return err
}

func (routes *Routes) FindFrontend(name string) (v Frontend, ok bool) {
	v, ok = routes.GlobalRoutes[name]
	return v, ok
}

func (routes *Routes) DeleteBackends(name string) {
	frontend, ok := routes.GlobalRoutes[name]
	if !ok {
		return
	}
	frontend.Backends = make(map[string]Backend)
	frontend.EndpointTable = make(map[string]Endpoint)

	routes.GlobalRoutes[name] = frontend
}

func (routes *Routes) CreateFrontend(name string, url string) (*Frontend, error) {
	frontend := Frontend{
		Name:          name,
		Backends:      make(map[string]Backend),
		EndpointTable: make(map[string]Endpoint),
		HostAliases:   make([]string, 0),
	}

	if url != "" {
		frontend.HostAliases = append(frontend.HostAliases, url)
	}
	routes.GlobalRoutes[frontend.Name] = frontend
	return &frontend, nil
}

func (routes *Routes) DeleteFrontend(frontendName string) error {
	delete(routes.GlobalRoutes, frontendName)
	return routes.WriteRoutes()
}

func (routes *Routes) AddAlias(alias string, frontendName string) error {
	frontend, ok := routes.GlobalRoutes[frontendName]
	if !ok {
		err := fmt.Errorf("Error getting frontend with name: %v, ensure that the frontend has backenden previously created using the CreateFronted method", frontendName)
		glog.Errorf("%s", err.Error())
		return err
	}
	for _, v := range frontend.HostAliases {
		if v == alias {
			return nil
		}
	}
	frontend.HostAliases = append(frontend.HostAliases, alias)
	routes.GlobalRoutes[frontendName] = frontend
	return routes.WriteRoutes()
}

func (routes *Routes) RemoveAlias(alias string, frontendName string) error {
	frontend, ok := routes.GlobalRoutes[frontendName]
	if !ok {
		err := fmt.Errorf("Error getting frontend with name: %v, ensure that the frontend has backenden previously created using the CreateFronted method", frontendName)
		glog.Errorf("%s\n", err.Error())
		return err
	}
	newAliases := []string{}
	for _, v := range frontend.HostAliases {
		if v == alias || v == "" {
			continue
		}
		newAliases = append(newAliases, v)
	}
	frontend.HostAliases = newAliases
	routes.GlobalRoutes[frontendName] = frontend
	routes.WriteRoutes()
	return nil
}

func (routes *Routes) AddRoute(frontend *Frontend, backend *Backend, endpoints []Endpoint) error {
	var id string
	existingFrontend, ok := routes.GlobalRoutes[frontend.Name]
	if !ok {
		err := fmt.Errorf("Error getting frontend with name: %v, ensure that the frontend has backenden previously created using the CreateFronted method", frontend.Name)
		glog.Errorf("%s\n", err.Error())
		return err
	}

	epIDs := make([]string, 1)
	for newEpID := range endpoints {
		newEndpoint := endpoints[newEpID]
		if newEndpoint.IP == "" || newEndpoint.Port == "" {
			continue
		}
		found := false
		for _, ep := range existingFrontend.EndpointTable {
			if ep.IP == newEndpoint.IP && ep.Port == newEndpoint.Port {
				epIDs = append(epIDs, ep.ID)
				found = true
				break
			}
		}
		if !found {
			id = makeID()
			ep := Endpoint{id, newEndpoint.IP, newEndpoint.Port}
			existingFrontend.EndpointTable[id] = ep
			epIDs = append(epIDs, ep.ID)
		}
	}
	// locate a backend that may already exist with this protocol and fe/be path
	found := false
	for _, be := range existingFrontend.Backends {
		if be.FePath == backend.FePath && be.BePath == backend.BePath && cmpStrSlices(backend.Protocols, be.Protocols) {
			for _, epID := range epIDs {
				be.EndpointIDs = append(be.EndpointIDs, epID)
			}
			existingFrontend.Backends[be.ID] = be
			found = true
			break
		}
	}
	if !found {
		id = makeID()
		existingFrontend.Backends[id] = Backend{id, backend.FePath, backend.BePath, backend.Protocols, epIDs, TermEdge, nil}
	}
	routes.GlobalRoutes[existingFrontend.Name] = existingFrontend
	routes.WriteRoutes()

	return nil
}

func cmpStrSlices(first []string, second []string) bool {
	sort.Strings(first)
	sort.Strings(second)
	strFirst := fmt.Sprintf("%v", first)
	strSecond := fmt.Sprintf("%v", second)
	return strFirst == strSecond
}
