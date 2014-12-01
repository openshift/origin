package router

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"
	"time"
)

const (
	ProtocolHttp  = "http"
	ProtocolHttps = "https"
	ProtocolTls   = "tls"
)

const (
	TERM_EDGE  = "TERM_EDGE"
	TERM_GEAR  = "TERM_GEAR"
	TERM_RESSL = "TERM_RESSL"
)

const (
	RouteFile = "/var/lib/containers/router/routes.json"
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
}

type Router interface {
	ReadRoutes()
	WriteRoutes()
	FindFrontend(name string) (v Frontend, ok bool)
	DeleteBackends(name string)
	CreateFrontend(name, url string)
	DeleteFrontend(name string)
	AddAlias(alias, frontendName string)
	RemoveAlias(alias, frontendName string)
	AddRoute(frontend *Frontend, backend *Backend, endpoints []Endpoint)
	WriteConfig()
	ReloadRouter() bool
}

func makeID() string {
	var s string
	s = strconv.FormatInt(time.Now().UnixNano(), 16)
	return s
}

func (routes *Routes) ReadRoutes() {
	//fmt.Printf("Reading routes file (%s)\n", RouteFile)
	dat, err := ioutil.ReadFile(RouteFile)
	if err != nil {
		routes.GlobalRoutes = make(map[string]Frontend)
		return
	}
	json.Unmarshal(dat, &routes.GlobalRoutes)
}

func (routes *Routes) WriteRoutes() {
	dat, err := json.MarshalIndent(routes.GlobalRoutes, "", "  ")
	if err != nil {
		fmt.Println("Failed to marshal routes - %s", err.Error())
	}
	err = ioutil.WriteFile(RouteFile, dat, 0644)
	if err != nil {
		fmt.Println("Failed to write to routes file - %s", err.Error())
	}
}

func (routes *Routes) FindFrontend(name string) (v Frontend, ok bool) {
	v, ok = routes.GlobalRoutes[name]
	return
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

func (routes *Routes) CreateFrontend(name string, url string) {
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
	routes.WriteRoutes()
}

func (routes *Routes) DeleteFrontend(name string) {
	delete(routes.GlobalRoutes, name)
	routes.WriteRoutes()
}

func (routes *Routes) AddAlias(alias, frontendName string) {
	frontend := routes.GlobalRoutes[frontendName]
	for _, v := range frontend.HostAliases {
		if v == alias {
			return
		}
	}

	frontend.HostAliases = append(frontend.HostAliases, alias)
	routes.GlobalRoutes[frontendName] = frontend
	routes.WriteRoutes()
}

func (routes *Routes) RemoveAlias(alias, frontendName string) {
	frontend := routes.GlobalRoutes[frontendName]
	newAliases := make([]string, 0)
	for _, v := range frontend.HostAliases {
		if v == alias || v == "" {
			continue
		}
		newAliases = append(newAliases, v)
	}
	frontend.HostAliases = newAliases
	routes.GlobalRoutes[frontendName] = frontend
	routes.WriteRoutes()
}

func (routes *Routes) AddRoute(frontend *Frontend, backend *Backend, endpoints []Endpoint) {
	var id string
	existingFrontend := routes.GlobalRoutes[frontend.Name]

	epIDs := make([]string, 1)
	for newEpId := range endpoints {
		newEndpoint := endpoints[newEpId]
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
			for _, epId := range epIDs {
				be.EndpointIDs = append(be.EndpointIDs, epId)
			}
			existingFrontend.Backends[be.ID] = be
			found = true
			break
		}
	}
	if !found {
		id = makeID()
		existingFrontend.Backends[id] = Backend{id, backend.FePath, backend.BePath, backend.Protocols, epIDs, TERM_EDGE, nil}
	}
	routes.GlobalRoutes[existingFrontend.Name] = existingFrontend
	routes.WriteRoutes()
}

func cmpStrSlices(first []string, second []string) bool {
	if len(first) != len(second) {
		return false
	}
	for _, fi := range first {
		found := false
		for _, si := range second {
			if fi == si {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
