package router

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
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
	BeTable       map[string]Backend
	EndpointTable map[string]Endpoint
}

type Backend struct {
	Id           string
	FePath       string
	BePath       string
	Protocols    []string
	EndpointIds  []string
	SslTerm      string
	Certificates []Certificate
}

type Certificate struct {
	Id                 string
	Contents           []byte
	PrivateKey         []byte
	PrivateKeyPassword string
}

type Endpoint struct {
	Id   string
	IP   string
	Port string
}

var GlobalRoutes map[string]Frontend

func makeId() string {
	var s string
	s = strconv.FormatInt(time.Now().UnixNano(), 16)
	return s
}

func PrintRoutes() string {
	dat, err := ioutil.ReadFile(RouteFile)
	var s string
	if err != nil {
		s = fmt.Sprintf("Error reading routes file : %s", err.Error())
	} else {
		s = fmt.Sprintf("%s", string(dat))
	}
	return s
}

func PrintFrontendRoutes(frontendname string) string {
	dat, err := json.MarshalIndent(GlobalRoutes[frontendname], "", "  ")
	if err != nil {
		fmt.Println("Failed to marshal routes - %s", err.Error())
	}
	return string(dat)
}

func ReadRoutes() {
	//fmt.Printf("Reading routes file (%s)\n", RouteFile)
	dat, err := ioutil.ReadFile(RouteFile)
	if err != nil {
		GlobalRoutes = make(map[string]Frontend)
		return
	}
	json.Unmarshal(dat, &GlobalRoutes)
}

func WriteRoutes() {
	dat, err := json.MarshalIndent(GlobalRoutes, "", "  ")
	if err != nil {
		fmt.Println("Failed to marshal routes - %s", err.Error())
	}
	err = ioutil.WriteFile(RouteFile, dat, 0644)
	if err != nil {
		fmt.Println("Failed to write to routes file - %s", err.Error())
	}
}

func (a *Frontend) Init() {
	// a.HostAliases = make([]string)
	// a.Certificates = make([]Certificate)
	if a.BeTable == nil {
		a.BeTable = make(map[string]Backend)
	}
	if a.EndpointTable == nil {
		a.EndpointTable = make(map[string]Endpoint)
	}
}

func FindFrontend(name string) (v Frontend, ok bool) {
	v, ok = GlobalRoutes[name]
	return
}

func DeleteBackends(name string) {
	a, ok := GlobalRoutes[name]
	if !ok {
		return
	}
	a.EndpointTable = nil
	a.BeTable = nil
	GlobalRoutes[name] = a
}

func CreateFrontend(name string, url string) {
	a := Frontend{}
	a.Init()
	a.Name = name
	a.HostAliases = make([]string, 1)
	if url != "" {
		a.HostAliases[0] = url
	}
	if GlobalRoutes == nil {
		GlobalRoutes = make(map[string]Frontend)
	}
	GlobalRoutes[a.Name] = a
	WriteRoutes()
}

func DeleteFrontend(frontendname string) {
	delete(GlobalRoutes, frontendname)
	WriteRoutes()
}

func AddAlias(alias string, frontendname string) {
	a := GlobalRoutes[frontendname]
	for _, v := range a.HostAliases {
		if v == alias {
			return
		}
	}

	a.HostAliases = append(a.HostAliases, alias)
	GlobalRoutes[frontendname] = a
	WriteRoutes()
}

func AddRoute(frontendname string, fe_path string, be_path string, protocols []string, endpoints []Endpoint) {
	var id string
	a := GlobalRoutes[frontendname]
	a.Init()

	ep_ids := make([]string, 1)
	for new_ep_id := range endpoints {
		new_endpoint := endpoints[new_ep_id]
		if new_endpoint.IP == "" || new_endpoint.Port == "" {
			continue
		}
		found := false
		for _, ep := range a.EndpointTable {
			if ep.IP == new_endpoint.IP && ep.Port == new_endpoint.Port {
				ep_ids = append(ep_ids, ep.Id)
				found = true
				break
			}
		}
		if !found {
			id = makeId()
			ep := Endpoint{id, new_endpoint.IP, new_endpoint.Port}
			a.EndpointTable[id] = ep
			ep_ids = append(ep_ids, ep.Id)
		}
	}
	// locate a backend that may already exist with this protocol and fe/be path
	found := false
	for _, be := range a.BeTable {
		if be.FePath == fe_path && be.BePath == be_path && cmpStrSlices(protocols, be.Protocols) {
			for _, ep_id := range ep_ids {
				be.EndpointIds = append(be.EndpointIds, ep_id)
			}
			a.BeTable[be.Id] = be
			found = true
			break
		}
	}
	if !found {
		id = makeId()
		a.BeTable[id] = Backend{id, fe_path, be_path, protocols, ep_ids, TERM_EDGE, nil}
	}
	GlobalRoutes[a.Name] = a
	WriteRoutes()
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

func (a Frontend) PrintOut() {
	fmt.Println(GlobalRoutes)
}

func BumpRouter() {
	return
	out, err := exec.Command("docker", "kill", "-s", "SIGUSR2", "openshift-router").CombinedOutput()
	if err != nil {
		fmt.Println(string(out))
		fmt.Printf("Failed to reload the router - %s\n", err.Error())
		fmt.Println("Router plugin not installed?")
	}
}

func init() {
	ReadRoutes()
}
