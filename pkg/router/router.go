package router

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sort"
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
	ReadRoutes() (*Routes, error)
	WriteRoutes() (*Routes, error)
	FindFrontend(name string) (v Frontend, ok bool)
	DeleteBackends(name string)
	CreateFrontend(name string, url string) (*Routes, error)
	DeleteFrontend(frontendname string) (*Routes, error)
	AddAlias(alias string, frontendname string) (*Routes, error)
	RemoveAlias(alias string, frontendname string) (*Routes, error)
	AddRoute(frontendname string, fePath string, bePath string, protocols []string, endpoints []Endpoint) (*Routes, error)
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

func (routes *Routes) ReadRoutes() (*Routes, error) {
	file := routes.RouteFile
	fmt.Printf("Reading routes file (%s)\n", file)
	dat, err := ioutil.ReadFile(file)
	if err != nil {
		fmt.Printf("Error while reading file (%s)\n", file)
		routes.GlobalRoutes = make(map[string]Frontend)
		return nil, err
	}
	json.Unmarshal(dat, &routes.GlobalRoutes)
	fmt.Printf("Marshall result %+v\n", routes.GlobalRoutes)
	return routes, nil
}

func (routes *Routes) WriteRoutes() (*Routes, error) {
	dat, err := json.MarshalIndent(routes.GlobalRoutes, "", "  ")
	if err != nil {
		fmt.Println("Failed to marshal routes - %s", err.Error())
		return nil, err
	}
	file := routes.RouteFile
	fmt.Printf("Writing routes tofile (%s)\n", file)
	err = ioutil.WriteFile(file, dat, 0644)
	if err != nil {
		fmt.Println("Failed to write to routes file - %s", err.Error())
		return nil, err
	}
	return routes, nil
}

func (routes *Routes) FindFrontend(name string) (v Frontend, ok bool) {
	v, ok = routes.GlobalRoutes[name]
	return v, ok
}

func (routes *Routes) DeleteBackends(name string) {
	a, ok := routes.GlobalRoutes[name]
	if !ok {
		return
	}
	a.Backends = make(map[string]Backend)
	a.EndpointTable = make(map[string]Endpoint)
	routes.GlobalRoutes[name] = a
}

func (routes *Routes) CreateFrontend(name string, url string) (*Routes, error) {
	a := Frontend{}
	a.Backends = make(map[string]Backend)
	a.EndpointTable = make(map[string]Endpoint)
	a.Name = name
	a.HostAliases = make([]string, 0)
	if url != "" {
		a.HostAliases = append(a.HostAliases, url)
	}
	routes.GlobalRoutes[a.Name] = a
	return routes.WriteRoutes()
}

func (routes *Routes) DeleteFrontend(frontendname string) (*Routes, error) {
	delete(routes.GlobalRoutes, frontendname)
	routes.WriteRoutes()
	return routes, nil
}

func (routes *Routes) AddAlias(alias string, frontendname string) (*Routes, error) {
	a, ok := routes.GlobalRoutes[frontendname]
	if !ok {
		err := fmt.Errorf("Error getting frontend with name: %v, ensure that the frontend has been previously created using the CreateFronted method", frontendname)
		fmt.Printf("%v\n", err.Error())
		return nil, err
	}
	for _, v := range a.HostAliases {
		if v == alias {
			return routes, nil
		}
	}

	a.HostAliases = append(a.HostAliases, alias)
	routes.GlobalRoutes[frontendname] = a
	routes.WriteRoutes()
	return routes, nil
}

func (routes *Routes) RemoveAlias(alias string, frontendname string) (*Routes, error) {
	a, ok := routes.GlobalRoutes[frontendname]
	if !ok {
		err := fmt.Errorf("Error getting frontend with name: %v, ensure that the frontend has been previously created using the CreateFronted method", frontendname)
		fmt.Printf("%v\n", err.Error())
		return nil, err
	}
	newAliases := make([]string, 0)
	for _, v := range a.HostAliases {
		if v == alias || v == "" {
			continue
		}
		newAliases = append(newAliases, v)
	}
	a.HostAliases = newAliases
	routes.GlobalRoutes[frontendname] = a
	routes.WriteRoutes()
	return routes, nil
}

func (routes *Routes) AddRoute(frontendname string, fePath string, bePath string, protocols []string, endpoints []Endpoint) (*Routes, error) {
	var id string
	a, ok := routes.GlobalRoutes[frontendname]
	if !ok {
		err := fmt.Errorf("Error getting frontend with name: %v, ensure that the frontend has been previously created using the CreateFronted method", frontendname)
		fmt.Printf("%v\n", err.Error())
		return nil, err
	}
	a.Name = frontendname

	epIDs := make([]string, 1)
	for newEpId := range endpoints {
		newEndpoint := endpoints[newEpId]
		if newEndpoint.IP == "" || newEndpoint.Port == "" {
			continue
		}
		found := false
		for _, ep := range a.EndpointTable {
			if ep.IP == newEndpoint.IP && ep.Port == newEndpoint.Port {
				epIDs = append(epIDs, ep.ID)
				found = true
				break
			}
		}
		if !found {
			id = makeID()
			ep := Endpoint{id, newEndpoint.IP, newEndpoint.Port}
			fmt.Printf("Frontend  %+v\n", a)
			fmt.Printf("Endpoint %+v\n", ep)
			fmt.Printf("Routes %+v\n", a.EndpointTable[id])
			a.EndpointTable[id] = ep
			fmt.Printf("Routes after %+v\n", a.EndpointTable[id])
			epIDs = append(epIDs, ep.ID)
		}
	}

	// locate a backend that may already exist with this protocol and fe/be path
	found := false
	fmt.Printf("Backends  %+v\n", a.Backends)
	for _, be := range a.Backends {
		sort.Strings(protocols)
		sort.Strings(be.Protocols)
		strProtocols := fmt.Sprintf("%v", protocols)
		strBeProtocols := fmt.Sprintf("%v", be.Protocols)
		if be.FePath == fePath && be.BePath == bePath && strProtocols == strBeProtocols {
			for _, epId := range epIDs {
				be.EndpointIDs = append(be.EndpointIDs, epId)
			}
			a.Backends[be.ID] = be
			found = true
			break
		}
	}
	if !found {
		id = makeID()
		a.Backends[id] = Backend{id, fePath, bePath, protocols, epIDs, TERM_EDGE, nil}
	}
	fmt.Printf("Frontend %+v\n", a)
	routes.WriteRoutes()
	return routes, nil
}
