// Copyright 2013 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package testing provides a fake implementation of the Docker API, useful for
// testing purpose.
package testing

import (
	"archive/tar"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	mathrand "math/rand"
	"net"
	"net/http"
	libpath "path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/pkg/stdcopy"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/gorilla/mux"
)

var nameRegexp = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]+$`)

// DockerServer represents a programmable, concurrent (not much), HTTP server
// implementing a fake version of the Docker remote API.
//
// It can used in standalone mode, listening for connections or as an arbitrary
// HTTP handler.
//
// For more details on the remote API, check http://goo.gl/G3plxW.
type DockerServer struct {
	containers     map[string]*docker.Container
	contNameToID   map[string]string
	uploadedFiles  map[string]string
	execs          []*docker.ExecInspect
	execMut        sync.RWMutex
	cMut           sync.RWMutex
	images         map[string]docker.Image
	iMut           sync.RWMutex
	imgIDs         map[string]string
	networks       []*docker.Network
	netMut         sync.RWMutex
	listener       net.Listener
	mux            *mux.Router
	hook           func(*http.Request)
	failures       map[string]string
	multiFailures  []map[string]string
	execCallbacks  map[string]func()
	statsCallbacks map[string]func(string) docker.Stats
	customHandlers map[string]http.Handler
	handlerMutex   sync.RWMutex
	cChan          chan<- *docker.Container
	volStore       map[string]*volumeCounter
	volMut         sync.RWMutex
	swarmMut       sync.RWMutex
	swarm          *swarm.Swarm
	swarmServer    *swarmServer
	nodes          []swarm.Node
	nodeID         string
	tasks          []*swarm.Task
	services       []*swarm.Service
	nodeRR         int
	servicePorts   int
}

type volumeCounter struct {
	volume docker.Volume
	count  int
}

func baseDockerServer() DockerServer {
	return DockerServer{
		containers:     make(map[string]*docker.Container),
		contNameToID:   make(map[string]string),
		imgIDs:         make(map[string]string),
		images:         make(map[string]docker.Image),
		failures:       make(map[string]string),
		execCallbacks:  make(map[string]func()),
		statsCallbacks: make(map[string]func(string) docker.Stats),
		customHandlers: make(map[string]http.Handler),
		uploadedFiles:  make(map[string]string),
	}
}

func buildDockerServer(listener net.Listener, containerChan chan<- *docker.Container, hook func(*http.Request)) *DockerServer {
	server := baseDockerServer()
	server.listener = listener
	server.hook = hook
	server.cChan = containerChan
	server.buildMuxer()
	return &server
}

// NewServer returns a new instance of the fake server, in standalone mode. Use
// the method URL to get the URL of the server.
//
// It receives the bind address (use 127.0.0.1:0 for getting an available port
// on the host), a channel of containers and a hook function, that will be
// called on every request.
//
// The fake server will send containers in the channel whenever the container
// changes its state, via the HTTP API (i.e.: create, start and stop). This
// channel may be nil, which means that the server won't notify on state
// changes.
func NewServer(bind string, containerChan chan<- *docker.Container, hook func(*http.Request)) (*DockerServer, error) {
	listener, err := net.Listen("tcp", bind)
	if err != nil {
		return nil, err
	}
	server := buildDockerServer(listener, containerChan, hook)
	go http.Serve(listener, server)
	return server, nil
}

// TLSConfig is the set of options to start the TLS-enabled testing server.
type TLSConfig struct {
	CertPath    string
	CertKeyPath string
	RootCAPath  string
}

// NewTLSServer creates and starts a TLS-enabled testing server.
func NewTLSServer(bind string, containerChan chan<- *docker.Container, hook func(*http.Request), tlsConfig TLSConfig) (*DockerServer, error) {
	listener, err := net.Listen("tcp", bind)
	if err != nil {
		return nil, err
	}
	defaultCertificate, err := tls.LoadX509KeyPair(tlsConfig.CertPath, tlsConfig.CertKeyPath)
	if err != nil {
		return nil, err
	}
	tlsServerConfig := new(tls.Config)
	tlsServerConfig.Certificates = []tls.Certificate{defaultCertificate}
	if tlsConfig.RootCAPath != "" {
		rootCertPEM, err := ioutil.ReadFile(tlsConfig.RootCAPath)
		if err != nil {
			return nil, err
		}
		certsPool := x509.NewCertPool()
		certsPool.AppendCertsFromPEM(rootCertPEM)
		tlsServerConfig.RootCAs = certsPool
	}
	tlsListener := tls.NewListener(listener, tlsServerConfig)
	server := buildDockerServer(tlsListener, containerChan, hook)
	go http.Serve(tlsListener, server)
	return server, nil
}

func (s *DockerServer) notify(container *docker.Container) {
	if s.cChan != nil {
		s.cChan <- container
	}
}

func (s *DockerServer) buildMuxer() {
	s.mux = mux.NewRouter()
	s.mux.Path("/commit").Methods(http.MethodPost).HandlerFunc(s.handlerWrapper(s.commitContainer))
	s.mux.Path("/containers/json").Methods(http.MethodGet).HandlerFunc(s.handlerWrapper(s.listContainers))
	s.mux.Path("/containers/create").Methods(http.MethodPost).HandlerFunc(s.handlerWrapper(s.createContainer))
	s.mux.Path("/containers/{id:.*}/json").Methods(http.MethodGet).HandlerFunc(s.handlerWrapper(s.inspectContainer))
	s.mux.Path("/containers/{id:.*}/rename").Methods(http.MethodPost).HandlerFunc(s.handlerWrapper(s.renameContainer))
	s.mux.Path("/containers/{id:.*}/top").Methods(http.MethodGet).HandlerFunc(s.handlerWrapper(s.topContainer))
	s.mux.Path("/containers/{id:.*}/start").Methods(http.MethodPost).HandlerFunc(s.handlerWrapper(s.startContainer))
	s.mux.Path("/containers/{id:.*}/kill").Methods(http.MethodPost).HandlerFunc(s.handlerWrapper(s.stopContainer))
	s.mux.Path("/containers/{id:.*}/stop").Methods(http.MethodPost).HandlerFunc(s.handlerWrapper(s.stopContainer))
	s.mux.Path("/containers/{id:.*}/pause").Methods(http.MethodPost).HandlerFunc(s.handlerWrapper(s.pauseContainer))
	s.mux.Path("/containers/{id:.*}/unpause").Methods(http.MethodPost).HandlerFunc(s.handlerWrapper(s.unpauseContainer))
	s.mux.Path("/containers/{id:.*}/wait").Methods(http.MethodPost).HandlerFunc(s.handlerWrapper(s.waitContainer))
	s.mux.Path("/containers/{id:.*}/attach").Methods(http.MethodPost).HandlerFunc(s.handlerWrapper(s.attachContainer))
	s.mux.Path("/containers/{id:.*}").Methods(http.MethodDelete).HandlerFunc(s.handlerWrapper(s.removeContainer))
	s.mux.Path("/containers/{id:.*}/exec").Methods(http.MethodPost).HandlerFunc(s.handlerWrapper(s.createExecContainer))
	s.mux.Path("/containers/{id:.*}/stats").Methods(http.MethodGet).HandlerFunc(s.handlerWrapper(s.statsContainer))
	s.mux.Path("/containers/{id:.*}/archive").Methods(http.MethodPut).HandlerFunc(s.handlerWrapper(s.uploadToContainer))
	s.mux.Path("/containers/{id:.*}/archive").Methods(http.MethodGet).HandlerFunc(s.handlerWrapper(s.downloadFromContainer))
	s.mux.Path("/containers/{id:.*}/logs").Methods(http.MethodGet).HandlerFunc(s.handlerWrapper(s.logContainer))
	s.mux.Path("/exec/{id:.*}/resize").Methods(http.MethodPost).HandlerFunc(s.handlerWrapper(s.resizeExecContainer))
	s.mux.Path("/exec/{id:.*}/start").Methods(http.MethodPost).HandlerFunc(s.handlerWrapper(s.startExecContainer))
	s.mux.Path("/exec/{id:.*}/json").Methods(http.MethodGet).HandlerFunc(s.handlerWrapper(s.inspectExecContainer))
	s.mux.Path("/images/create").Methods(http.MethodPost).HandlerFunc(s.handlerWrapper(s.pullImage))
	s.mux.Path("/build").Methods(http.MethodPost).HandlerFunc(s.handlerWrapper(s.buildImage))
	s.mux.Path("/images/json").Methods(http.MethodGet).HandlerFunc(s.handlerWrapper(s.listImages))
	s.mux.Path("/images/{id:.*}").Methods(http.MethodDelete).HandlerFunc(s.handlerWrapper(s.removeImage))
	s.mux.Path("/images/{name:.*}/json").Methods(http.MethodGet).HandlerFunc(s.handlerWrapper(s.inspectImage))
	s.mux.Path("/images/{name:.*}/push").Methods(http.MethodPost).HandlerFunc(s.handlerWrapper(s.pushImage))
	s.mux.Path("/images/{name:.*}/tag").Methods(http.MethodPost).HandlerFunc(s.handlerWrapper(s.tagImage))
	s.mux.Path("/events").Methods(http.MethodGet).HandlerFunc(s.listEvents)
	s.mux.Path("/_ping").Methods(http.MethodGet).HandlerFunc(s.handlerWrapper(s.pingDocker))
	s.mux.Path("/images/load").Methods(http.MethodPost).HandlerFunc(s.handlerWrapper(s.loadImage))
	s.mux.Path("/images/{id:.*}/get").Methods(http.MethodGet).HandlerFunc(s.handlerWrapper(s.getImage))
	s.mux.Path("/networks").Methods(http.MethodGet).HandlerFunc(s.handlerWrapper(s.listNetworks))
	s.mux.Path("/networks/{id:.*}").Methods(http.MethodGet).HandlerFunc(s.handlerWrapper(s.networkInfo))
	s.mux.Path("/networks/{id:.*}").Methods(http.MethodDelete).HandlerFunc(s.handlerWrapper(s.removeNetwork))
	s.mux.Path("/networks/create").Methods(http.MethodPost).HandlerFunc(s.handlerWrapper(s.createNetwork))
	s.mux.Path("/networks/{id:.*}/connect").Methods(http.MethodPost).HandlerFunc(s.handlerWrapper(s.networksConnect))
	s.mux.Path("/volumes").Methods(http.MethodGet).HandlerFunc(s.handlerWrapper(s.listVolumes))
	s.mux.Path("/volumes/create").Methods(http.MethodPost).HandlerFunc(s.handlerWrapper(s.createVolume))
	s.mux.Path("/volumes/{name:.*}").Methods(http.MethodGet).HandlerFunc(s.handlerWrapper(s.inspectVolume))
	s.mux.Path("/volumes/{name:.*}").Methods(http.MethodDelete).HandlerFunc(s.handlerWrapper(s.removeVolume))
	s.mux.Path("/info").Methods(http.MethodGet).HandlerFunc(s.handlerWrapper(s.infoDocker))
	s.mux.Path("/version").Methods(http.MethodGet).HandlerFunc(s.handlerWrapper(s.versionDocker))
	s.mux.Path("/swarm/init").Methods(http.MethodPost).HandlerFunc(s.handlerWrapper(s.swarmInit))
	s.mux.Path("/swarm").Methods(http.MethodGet).HandlerFunc(s.handlerWrapper(s.swarmInspect))
	s.mux.Path("/swarm/join").Methods(http.MethodPost).HandlerFunc(s.handlerWrapper(s.swarmJoin))
	s.mux.Path("/swarm/leave").Methods(http.MethodPost).HandlerFunc(s.handlerWrapper(s.swarmLeave))
	s.mux.Path("/nodes/{id:.+}/update").Methods(http.MethodPost).HandlerFunc(s.handlerWrapper(s.nodeUpdate))
	s.mux.Path("/nodes/{id:.+}").Methods(http.MethodGet).HandlerFunc(s.handlerWrapper(s.nodeInspect))
	s.mux.Path("/nodes/{id:.+}").Methods(http.MethodDelete).HandlerFunc(s.handlerWrapper(s.nodeDelete))
	s.mux.Path("/nodes").Methods(http.MethodGet).HandlerFunc(s.handlerWrapper(s.nodeList))
	s.mux.Path("/services/create").Methods(http.MethodPost).HandlerFunc(s.handlerWrapper(s.serviceCreate))
	s.mux.Path("/services/{id:.+}").Methods(http.MethodGet).HandlerFunc(s.handlerWrapper(s.serviceInspect))
	s.mux.Path("/services").Methods(http.MethodGet).HandlerFunc(s.handlerWrapper(s.serviceList))
	s.mux.Path("/services/{id:.+}").Methods(http.MethodDelete).HandlerFunc(s.handlerWrapper(s.serviceDelete))
	s.mux.Path("/services/{id:.+}/update").Methods(http.MethodPost).HandlerFunc(s.handlerWrapper(s.serviceUpdate))
	s.mux.Path("/tasks").Methods(http.MethodGet).HandlerFunc(s.handlerWrapper(s.taskList))
	s.mux.Path("/tasks/{id:.+}").Methods(http.MethodGet).HandlerFunc(s.handlerWrapper(s.taskInspect))
}

// SetHook changes the hook function used by the server.
//
// The hook function is a function called on every request.
func (s *DockerServer) SetHook(hook func(*http.Request)) {
	s.hook = hook
}

// PrepareExec adds a callback to a container exec in the fake server.
//
// This function will be called whenever the given exec id is started, and the
// given exec id will remain in the "Running" start while the function is
// running, so it's useful for emulating an exec that runs for two seconds, for
// example:
//
//    opts := docker.CreateExecOptions{
//        AttachStdin:  true,
//        AttachStdout: true,
//        AttachStderr: true,
//        Tty:          true,
//        Cmd:          []string{"/bin/bash", "-l"},
//    }
//    // Client points to a fake server.
//    exec, err := client.CreateExec(opts)
//    // handle error
//    server.PrepareExec(exec.ID, func() {time.Sleep(2 * time.Second)})
//    err = client.StartExec(exec.ID, docker.StartExecOptions{Tty: true}) // will block for 2 seconds
//    // handle error
func (s *DockerServer) PrepareExec(id string, callback func()) {
	s.execCallbacks[id] = callback
}

// PrepareStats adds a callback that will be called for each container stats
// call.
//
// This callback function will be called multiple times if stream is set to
// true when stats is called.
func (s *DockerServer) PrepareStats(id string, callback func(string) docker.Stats) {
	s.statsCallbacks[id] = callback
}

// PrepareFailure adds a new expected failure based on a URL regexp it receives
// an id for the failure.
func (s *DockerServer) PrepareFailure(id string, urlRegexp string) {
	s.failures[id] = urlRegexp
}

// PrepareMultiFailures enqueues a new expected failure based on a URL regexp
// it receives an id for the failure.
func (s *DockerServer) PrepareMultiFailures(id string, urlRegexp string) {
	s.multiFailures = append(s.multiFailures, map[string]string{"error": id, "url": urlRegexp})
}

// ResetFailure removes an expected failure identified by the given id.
func (s *DockerServer) ResetFailure(id string) {
	delete(s.failures, id)
}

// ResetMultiFailures removes all enqueued failures.
func (s *DockerServer) ResetMultiFailures() {
	s.multiFailures = []map[string]string{}
}

// CustomHandler registers a custom handler for a specific path.
//
// For example:
//
//     server.CustomHandler("/containers/json", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//         http.Error(w, "Something wrong is not right", http.StatusInternalServerError)
//     }))
func (s *DockerServer) CustomHandler(path string, handler http.Handler) {
	s.handlerMutex.Lock()
	s.customHandlers[path] = handler
	s.handlerMutex.Unlock()
}

// MutateContainer changes the state of a container, returning an error if the
// given id does not match to any container "running" in the server.
func (s *DockerServer) MutateContainer(id string, state docker.State) error {
	s.cMut.Lock()
	defer s.cMut.Unlock()
	if container, ok := s.containers[id]; ok {
		container.State = state
		return nil
	}
	return errors.New("container not found")
}

// Stop stops the server.
func (s *DockerServer) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}
	if s.swarmServer != nil {
		s.swarmServer.listener.Close()
	}
}

// URL returns the HTTP URL of the server.
func (s *DockerServer) URL() string {
	if s.listener == nil {
		return ""
	}
	return "http://" + s.listener.Addr().String() + "/"
}

// ServeHTTP handles HTTP requests sent to the server.
func (s *DockerServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handlerMutex.RLock()
	defer s.handlerMutex.RUnlock()
	for re, handler := range s.customHandlers {
		if m, _ := regexp.MatchString(re, r.URL.Path); m {
			handler.ServeHTTP(w, r)
			return
		}
	}
	s.mux.ServeHTTP(w, r)
	if s.hook != nil {
		s.hook(r)
	}
}

// DefaultHandler returns default http.Handler mux, it allows customHandlers to
// call the default behavior if wanted.
func (s *DockerServer) DefaultHandler() http.Handler {
	return s.mux
}

func (s *DockerServer) handlerWrapper(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for errorID, urlRegexp := range s.failures {
			matched, err := regexp.MatchString(urlRegexp, r.URL.Path)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if !matched {
				continue
			}
			http.Error(w, errorID, http.StatusBadRequest)
			return
		}
		for i, failure := range s.multiFailures {
			matched, err := regexp.MatchString(failure["url"], r.URL.Path)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if !matched {
				continue
			}
			http.Error(w, failure["error"], http.StatusBadRequest)
			s.multiFailures = append(s.multiFailures[:i], s.multiFailures[i+1:]...)
			return
		}
		f(w, r)
	}
}

func (s *DockerServer) listContainers(w http.ResponseWriter, r *http.Request) {
	all := r.URL.Query().Get("all")
	filtersRaw := r.FormValue("filters")
	filters := make(map[string][]string)
	json.Unmarshal([]byte(filtersRaw), &filters)
	labelFilters := make(map[string]*string)
	for _, f := range filters["label"] {
		parts := strings.Split(f, "=")
		if len(parts) == 2 {
			labelFilters[parts[0]] = &parts[1]
			continue
		}
		labelFilters[parts[0]] = nil
	}
	s.cMut.RLock()
	result := make([]docker.APIContainers, 0, len(s.containers))
loop:
	for _, container := range s.containers {
		if all == "1" || container.State.Running {
			var ports []docker.APIPort
			if container.NetworkSettings != nil {
				ports = container.NetworkSettings.PortMappingAPI()
			}
			for l, fv := range labelFilters {
				lv, ok := container.Config.Labels[l]
				if !ok {
					continue loop
				}
				if fv != nil && lv != *fv {
					continue loop
				}
			}
			result = append(result, docker.APIContainers{
				ID:      container.ID,
				Image:   container.Image,
				Command: fmt.Sprintf("%s %s", container.Path, strings.Join(container.Args, " ")),
				Created: container.Created.Unix(),
				Status:  container.State.String(),
				State:   container.State.StateString(),
				Ports:   ports,
				Names:   []string{fmt.Sprintf("/%s", container.Name)},
			})
		}
	}
	s.cMut.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (s *DockerServer) listImages(w http.ResponseWriter, r *http.Request) {
	s.cMut.RLock()
	result := make([]docker.APIImages, len(s.images))
	i := 0
	for _, image := range s.images {
		result[i] = docker.APIImages{
			ID:      image.ID,
			Created: image.Created.Unix(),
		}
		for tag, id := range s.imgIDs {
			if id == image.ID {
				result[i].RepoTags = append(result[i].RepoTags, tag)
			}
		}
		i++
	}
	s.cMut.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (s *DockerServer) findImage(id string) (string, error) {
	s.iMut.RLock()
	defer s.iMut.RUnlock()
	image, ok := s.imgIDs[id]
	if ok {
		return image, nil
	}
	if _, ok := s.images[id]; ok {
		return id, nil
	}
	return "", errors.New("no such image")
}

func (s *DockerServer) createContainer(w http.ResponseWriter, r *http.Request) {
	var config struct {
		*docker.Config
		HostConfig *docker.HostConfig
	}
	defer r.Body.Close()
	err := json.NewDecoder(r.Body).Decode(&config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := r.URL.Query().Get("name")
	if name != "" && !nameRegexp.MatchString(name) {
		http.Error(w, "Invalid container name", http.StatusInternalServerError)
		return
	}
	imageID, err := s.findImage(config.Image)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	ports := map[docker.Port][]docker.PortBinding{}
	for port := range config.ExposedPorts {
		ports[port] = []docker.PortBinding{{
			HostIP:   "0.0.0.0",
			HostPort: strconv.Itoa(mathrand.Int() % 0xffff),
		}}
	}

	// the container may not have cmd when using a Dockerfile
	var path string
	var args []string
	if len(config.Cmd) == 1 {
		path = config.Cmd[0]
	} else if len(config.Cmd) > 1 {
		path = config.Cmd[0]
		args = config.Cmd[1:]
	}

	generatedID := s.generateID()
	config.Config.Hostname = generatedID[:12]
	container := docker.Container{
		Name:       name,
		ID:         generatedID,
		Created:    time.Now(),
		Path:       path,
		Args:       args,
		Config:     config.Config,
		HostConfig: config.HostConfig,
		State: docker.State{
			Running:  false,
			Pid:      mathrand.Int() % 50000,
			ExitCode: 0,
		},
		Image: config.Image,
		NetworkSettings: &docker.NetworkSettings{
			IPAddress:   fmt.Sprintf("172.16.42.%d", mathrand.Int()%250+2),
			IPPrefixLen: 24,
			Gateway:     "172.16.42.1",
			Bridge:      "docker0",
			Ports:       ports,
		},
	}
	s.cMut.Lock()
	if val, ok := s.uploadedFiles[imageID]; ok {
		s.uploadedFiles[container.ID] = val
	}
	if container.Name != "" {
		_, err = s.findContainerWithLock(container.Name, false)
		if err == nil {
			defer s.cMut.Unlock()
			http.Error(w, "there's already a container with this name", http.StatusConflict)
			return
		}
	}
	s.addContainer(&container)
	s.cMut.Unlock()
	w.WriteHeader(http.StatusCreated)
	s.notify(&container)

	json.NewEncoder(w).Encode(container)
}

func (s *DockerServer) addContainer(container *docker.Container) {
	s.containers[container.ID] = container
	if container.Name != "" {
		s.contNameToID[container.Name] = container.ID
	}
}

func (s *DockerServer) generateID() string {
	var buf [16]byte
	rand.Read(buf[:])
	return fmt.Sprintf("%x", buf)
}

func (s *DockerServer) renameContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	s.cMut.Lock()
	defer s.cMut.Unlock()
	container, err := s.findContainerWithLock(id, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	delete(s.contNameToID, container.Name)
	container.Name = r.URL.Query().Get("name")
	s.contNameToID[container.Name] = container.ID
	w.WriteHeader(http.StatusNoContent)
}

func (s *DockerServer) inspectContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	container, err := s.findContainer(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	s.cMut.RLock()
	defer s.cMut.RUnlock()
	json.NewEncoder(w).Encode(container)
}

func (s *DockerServer) statsContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	_, err := s.findContainer(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	stream, _ := strconv.ParseBool(r.URL.Query().Get("stream"))
	callback := s.statsCallbacks[id]
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(w)
	for {
		var stats docker.Stats
		if callback != nil {
			stats = callback(id)
		}
		encoder.Encode(stats)
		if !stream {
			break
		}
	}
}

func (s *DockerServer) uploadToContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	_, err := s.findContainer(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	path := r.URL.Query().Get("path")
	if r.Body != nil {
		tr := tar.NewReader(r.Body)
		if hdr, _ := tr.Next(); hdr != nil {
			path = libpath.Join(path, hdr.Name)
		}
	}
	s.cMut.Lock()
	s.uploadedFiles[id] = path
	s.cMut.Unlock()
	w.WriteHeader(http.StatusOK)
}

func (s *DockerServer) downloadFromContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	_, err := s.findContainer(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	path := r.URL.Query().Get("path")
	s.cMut.RLock()
	val, ok := s.uploadedFiles[id]
	s.cMut.RUnlock()
	if !ok || val != path {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Path %s not found", path)
		return
	}
	w.Header().Set("Content-Type", "application/x-tar")
	w.WriteHeader(http.StatusOK)
}

func (s *DockerServer) topContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	container, err := s.findContainer(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	s.cMut.RLock()
	defer s.cMut.RUnlock()
	if !container.State.Running {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Container %s is not running", id)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	result := docker.TopResult{
		Titles: []string{"UID", "PID", "PPID", "C", "STIME", "TTY", "TIME", "CMD"},
		Processes: [][]string{
			{"root", "7535", "7516", "0", "03:20", "?", "00:00:00", container.Path + " " + strings.Join(container.Args, " ")},
		},
	}
	json.NewEncoder(w).Encode(result)
}

func (s *DockerServer) startContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	container, err := s.findContainer(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	s.cMut.Lock()
	defer s.cMut.Unlock()
	defer r.Body.Close()
	if container.State.Running {
		http.Error(w, "", http.StatusNotModified)
		return
	}
	var hostConfig *docker.HostConfig
	err = json.NewDecoder(r.Body).Decode(&hostConfig)
	if err != nil && err != io.EOF {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if hostConfig == nil {
		hostConfig = container.HostConfig
	} else {
		container.HostConfig = hostConfig
	}
	if hostConfig != nil && len(hostConfig.PortBindings) > 0 {
		ports := map[docker.Port][]docker.PortBinding{}
		for key, items := range hostConfig.PortBindings {
			bindings := make([]docker.PortBinding, len(items))
			for i := range items {
				binding := docker.PortBinding{
					HostIP:   items[i].HostIP,
					HostPort: items[i].HostPort,
				}
				if binding.HostIP == "" {
					binding.HostIP = "0.0.0.0"
				}
				if binding.HostPort == "" {
					binding.HostPort = strconv.Itoa(mathrand.Int() % 0xffff)
				}
				bindings[i] = binding
			}
			ports[key] = bindings
		}
		container.NetworkSettings.Ports = ports
	}
	container.State.Running = true
	container.State.StartedAt = time.Now()
	s.notify(container)
}

func (s *DockerServer) stopContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	container, err := s.findContainer(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	s.cMut.Lock()
	defer s.cMut.Unlock()
	if !container.State.Running {
		http.Error(w, "Container not running", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	container.State.Running = false
	s.notify(container)
}

func (s *DockerServer) pauseContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	container, err := s.findContainer(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	s.cMut.Lock()
	defer s.cMut.Unlock()
	if container.State.Paused {
		http.Error(w, "Container already paused", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	container.State.Paused = true
}

func (s *DockerServer) unpauseContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	container, err := s.findContainer(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	s.cMut.Lock()
	defer s.cMut.Unlock()
	if !container.State.Paused {
		http.Error(w, "Container not paused", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	container.State.Paused = false
}

func (s *DockerServer) attachContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	container, err := s.findContainer(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "cannot hijack connection", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.docker.raw-stream")
	w.WriteHeader(http.StatusOK)
	conn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	wg := sync.WaitGroup{}
	if r.URL.Query().Get("stdin") == "1" {
		wg.Add(1)
		go func() {
			ioutil.ReadAll(conn)
			wg.Done()
		}()
	}
	outStream := stdcopy.NewStdWriter(conn, stdcopy.Stdout)
	s.cMut.RLock()
	if container.State.Running {
		fmt.Fprintf(outStream, "Container is running\n")
	} else {
		fmt.Fprintf(outStream, "Container is not running\n")
	}
	s.cMut.RUnlock()
	fmt.Fprintln(outStream, "What happened?")
	fmt.Fprintln(outStream, "Something happened")
	wg.Wait()
	if r.URL.Query().Get("stream") == "1" {
		for {
			time.Sleep(1e6)
			s.cMut.RLock()
			if !container.State.StartedAt.IsZero() && !container.State.Running {
				s.cMut.RUnlock()
				break
			}
			s.cMut.RUnlock()
		}
	}
	conn.Close()
}

func (s *DockerServer) waitContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	container, err := s.findContainer(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	var exitCode int
	for {
		time.Sleep(1e6)
		s.cMut.RLock()
		if !container.State.Running {
			exitCode = container.State.ExitCode
			s.cMut.RUnlock()
			break
		}
		s.cMut.RUnlock()
	}
	result := map[string]int{"StatusCode": exitCode}
	json.NewEncoder(w).Encode(result)
}

func (s *DockerServer) removeContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	force := r.URL.Query().Get("force")
	s.cMut.Lock()
	defer s.cMut.Unlock()
	container, err := s.findContainerWithLock(id, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if container.State.Running && force != "1" {
		msg := "Error: API error (406): Impossible to remove a running container, please stop it first"
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	delete(s.containers, container.ID)
	delete(s.contNameToID, container.Name)
}

func (s *DockerServer) commitContainer(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("container")
	container, err := s.findContainer(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	config := new(docker.Config)
	runConfig := r.URL.Query().Get("run")
	if runConfig != "" {
		err = json.Unmarshal([]byte(runConfig), config)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	image := docker.Image{
		ID:        "img-" + container.ID,
		Parent:    container.Image,
		Container: container.ID,
		Comment:   r.URL.Query().Get("m"),
		Author:    r.URL.Query().Get("author"),
		Config:    config,
	}
	repository := r.URL.Query().Get("repo")
	tag := r.URL.Query().Get("tag")
	s.iMut.Lock()
	s.images[image.ID] = image
	if repository != "" {
		if tag != "" {
			repository += ":" + tag
		}
		s.imgIDs[repository] = image.ID
	}
	s.iMut.Unlock()
	s.cMut.Lock()
	if val, ok := s.uploadedFiles[container.ID]; ok {
		s.uploadedFiles[image.ID] = val
	}
	s.cMut.Unlock()
	fmt.Fprintf(w, `{"ID":%q}`, image.ID)
}

func (s *DockerServer) findContainer(idOrName string) (*docker.Container, error) {
	return s.findContainerWithLock(idOrName, true)
}

func (s *DockerServer) findContainerWithLock(idOrName string, shouldLock bool) (*docker.Container, error) {
	if shouldLock {
		s.cMut.RLock()
		defer s.cMut.RUnlock()
	}
	if contID, ok := s.contNameToID[idOrName]; ok {
		idOrName = contID
	}
	if cont, ok := s.containers[idOrName]; ok {
		return cont, nil
	}
	return nil, errors.New("no such container")
}

func (s *DockerServer) logContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	container, err := s.findContainer(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.docker.raw-stream")
	w.WriteHeader(http.StatusOK)
	s.cMut.RLock()
	if container.State.Running {
		fmt.Fprintf(w, "Container is running\n")
	} else {
		fmt.Fprintf(w, "Container is not running\n")
	}
	s.cMut.RUnlock()
	fmt.Fprintln(w, "What happened?")
	fmt.Fprintln(w, "Something happened")
	if r.URL.Query().Get("follow") == "1" {
		for {
			time.Sleep(1e6)
			s.cMut.RLock()
			if !container.State.StartedAt.IsZero() && !container.State.Running {
				s.cMut.RUnlock()
				break
			}
			s.cMut.RUnlock()
		}
	}
}

func (s *DockerServer) buildImage(w http.ResponseWriter, r *http.Request) {
	if ct := r.Header.Get("Content-Type"); ct == "application/tar" {
		gotDockerFile := false
		tr := tar.NewReader(r.Body)
		for {
			header, err := tr.Next()
			if err != nil {
				break
			}
			if header.Name == "Dockerfile" {
				gotDockerFile = true
			}
		}
		if !gotDockerFile {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("miss Dockerfile"))
			return
		}
	}
	// we did not use that Dockerfile to build image cause we are a fake Docker daemon
	image := docker.Image{
		ID:      s.generateID(),
		Created: time.Now(),
	}

	query := r.URL.Query()
	repository := image.ID
	if t := query.Get("t"); t != "" {
		repository = t
	}
	s.iMut.Lock()
	s.images[image.ID] = image
	s.imgIDs[repository] = image.ID
	s.iMut.Unlock()
	w.Write([]byte(fmt.Sprintf("Successfully built %s", image.ID)))
}

func (s *DockerServer) pullImage(w http.ResponseWriter, r *http.Request) {
	fromImageName := r.URL.Query().Get("fromImage")
	tag := r.URL.Query().Get("tag")
	if fromImageName != "" {
		if tag != "" {
			separator := ":"
			if strings.HasPrefix(tag, "sha256") {
				separator = "@"
			}
			fromImageName = fmt.Sprintf("%s%s%s", fromImageName, separator, tag)
		}
	}
	image := docker.Image{
		ID:     s.generateID(),
		Config: &docker.Config{},
	}
	s.iMut.Lock()
	if _, exists := s.imgIDs[fromImageName]; fromImageName == "" || !exists {
		s.images[image.ID] = image
		if fromImageName != "" {
			s.imgIDs[fromImageName] = image.ID
		}
	}
	s.iMut.Unlock()
}

func (s *DockerServer) pushImage(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	tag := r.URL.Query().Get("tag")
	if tag != "" {
		name += ":" + tag
	}
	s.iMut.RLock()
	if _, ok := s.imgIDs[name]; !ok {
		s.iMut.RUnlock()
		http.Error(w, "No such image", http.StatusNotFound)
		return
	}
	s.iMut.RUnlock()
	fmt.Fprintln(w, "Pushing...")
	fmt.Fprintln(w, "Pushed")
}

func (s *DockerServer) tagImage(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	id, err := s.findImage(name)
	if err != nil {
		http.Error(w, "No such image", http.StatusNotFound)
		return
	}
	s.iMut.Lock()
	defer s.iMut.Unlock()
	newRepo := r.URL.Query().Get("repo")
	newTag := r.URL.Query().Get("tag")
	if newTag != "" {
		newRepo += ":" + newTag
	}
	s.imgIDs[newRepo] = id
	w.WriteHeader(http.StatusCreated)
}

func (s *DockerServer) removeImage(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	s.iMut.Lock()
	defer s.iMut.Unlock()
	var tag string
	if img, ok := s.imgIDs[id]; ok {
		id, tag = img, id
	}
	var tags []string
	for tag, taggedID := range s.imgIDs {
		if taggedID == id {
			tags = append(tags, tag)
		}
	}
	_, ok := s.images[id]
	if !ok {
		http.Error(w, "No such image", http.StatusNotFound)
		return
	}
	if tag == "" && len(tags) > 1 {
		http.Error(w, "image is referenced in multiple repositories", http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	if tag == "" {
		// delete called with image ID
		for _, t := range tags {
			delete(s.imgIDs, t)
		}
		delete(s.images, id)
	} else {
		// delete called with image repository name
		delete(s.imgIDs, tag)
		if len(tags) == 1 {
			delete(s.images, id)
		}
	}
}

func (s *DockerServer) inspectImage(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	s.iMut.RLock()
	defer s.iMut.RUnlock()
	if id, ok := s.imgIDs[name]; ok {
		name = id
	}
	img, ok := s.images[name]
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(img)
}

func (s *DockerServer) listEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var events [][]byte
	count := mathrand.Intn(20)
	for i := 0; i < count; i++ {
		data, err := json.Marshal(s.generateEvent())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		events = append(events, data)
	}
	w.WriteHeader(http.StatusOK)
	for _, d := range events {
		fmt.Fprintln(w, d)
		time.Sleep(time.Duration(mathrand.Intn(200)) * time.Millisecond)
	}
}

func (s *DockerServer) pingDocker(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *DockerServer) generateEvent() *docker.APIEvents {
	var eventType string
	switch mathrand.Intn(4) {
	case 0:
		eventType = "create"
	case 1:
		eventType = "start"
	case 2:
		eventType = "stop"
	case 3:
		eventType = "destroy"
	}
	return &docker.APIEvents{
		ID:     s.generateID(),
		Status: eventType,
		From:   "mybase:latest",
		Time:   time.Now().Unix(),
	}
}

func (s *DockerServer) loadImage(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *DockerServer) getImage(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/tar")
}

func (s *DockerServer) createExecContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	container, err := s.findContainer(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	execID := s.generateID()
	s.cMut.Lock()
	container.ExecIDs = append(container.ExecIDs, execID)
	s.cMut.Unlock()

	exec := docker.ExecInspect{
		ID:          execID,
		ContainerID: container.ID,
	}

	var params docker.CreateExecOptions
	err = json.NewDecoder(r.Body).Decode(&params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(params.Cmd) > 0 {
		exec.ProcessConfig.EntryPoint = params.Cmd[0]
		if len(params.Cmd) > 1 {
			exec.ProcessConfig.Arguments = params.Cmd[1:]
		}
	}

	exec.ProcessConfig.User = params.User
	exec.ProcessConfig.Tty = params.Tty

	s.execMut.Lock()
	s.execs = append(s.execs, &exec)
	s.execMut.Unlock()
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"Id": exec.ID})
}

func (s *DockerServer) startExecContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if exec, err := s.getExec(id, false); err == nil {
		s.execMut.Lock()
		exec.Running = true
		s.execMut.Unlock()
		if callback, ok := s.execCallbacks[id]; ok {
			callback()
			delete(s.execCallbacks, id)
		} else if callback, ok := s.execCallbacks["*"]; ok {
			callback()
			delete(s.execCallbacks, "*")
		}
		s.execMut.Lock()
		exec.Running = false
		s.execMut.Unlock()
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

func (s *DockerServer) resizeExecContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if _, err := s.getExec(id, false); err == nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

func (s *DockerServer) inspectExecContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if exec, err := s.getExec(id, true); err == nil {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(exec)
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

func (s *DockerServer) getExec(id string, copy bool) (*docker.ExecInspect, error) {
	s.execMut.RLock()
	defer s.execMut.RUnlock()
	for _, exec := range s.execs {
		if exec.ID == id {
			if copy {
				cp := *exec
				exec = &cp
			}
			return exec, nil
		}
	}
	return nil, errors.New("exec not found")
}

func (s *DockerServer) findNetwork(idOrName string) (*docker.Network, int, error) {
	s.netMut.RLock()
	defer s.netMut.RUnlock()
	for i, network := range s.networks {
		if network.ID == idOrName || network.Name == idOrName {
			return network, i, nil
		}
	}
	return nil, -1, errors.New("no such network")
}

func (s *DockerServer) listNetworks(w http.ResponseWriter, r *http.Request) {
	s.netMut.RLock()
	result := make([]docker.Network, 0, len(s.networks))
	for _, network := range s.networks {
		result = append(result, *network)
	}
	s.netMut.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (s *DockerServer) networkInfo(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	network, _, err := s.findNetwork(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(network)
}

// isValidName validates configuration objects supported by libnetwork
func isValidName(name string) bool {
	if name == "" || strings.Contains(name, ".") {
		return false
	}
	return true
}

func (s *DockerServer) createNetwork(w http.ResponseWriter, r *http.Request) {
	var config *docker.CreateNetworkOptions
	defer r.Body.Close()
	err := json.NewDecoder(r.Body).Decode(&config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !isValidName(config.Name) {
		http.Error(w, "Invalid network name", http.StatusBadRequest)
		return
	}
	if n, _, _ := s.findNetwork(config.Name); n != nil {
		http.Error(w, "network already exists", http.StatusForbidden)
		return
	}

	generatedID := s.generateID()
	network := docker.Network{
		Name:       config.Name,
		ID:         generatedID,
		Driver:     config.Driver,
		Containers: map[string]docker.Endpoint{},
	}
	s.netMut.Lock()
	s.networks = append(s.networks, &network)
	s.netMut.Unlock()
	w.WriteHeader(http.StatusCreated)
	c := struct{ ID string }{ID: network.ID}
	json.NewEncoder(w).Encode(c)
}

func (s *DockerServer) removeNetwork(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	_, index, err := s.findNetwork(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	s.netMut.Lock()
	defer s.netMut.Unlock()
	s.networks[index] = s.networks[len(s.networks)-1]
	s.networks = s.networks[:len(s.networks)-1]
	w.WriteHeader(http.StatusNoContent)
}

func (s *DockerServer) networksConnect(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var config *docker.NetworkConnectionOptions
	defer r.Body.Close()
	err := json.NewDecoder(r.Body).Decode(&config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	network, index, _ := s.findNetwork(id)
	container, _ := s.findContainer(config.Container)
	if network == nil || container == nil {
		http.Error(w, "network or container not found", http.StatusNotFound)
		return
	}

	if _, found := network.Containers[container.ID]; found {
		http.Error(w, "endpoint already exists in network", http.StatusBadRequest)
		return
	}

	s.netMut.Lock()
	s.networks[index].Containers[config.Container] = docker.Endpoint{}
	s.netMut.Unlock()

	w.WriteHeader(http.StatusOK)
}

func (s *DockerServer) listVolumes(w http.ResponseWriter, r *http.Request) {
	s.volMut.RLock()
	result := make([]docker.Volume, 0, len(s.volStore))
	for _, volumeCounter := range s.volStore {
		result = append(result, volumeCounter.volume)
	}
	s.volMut.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string][]docker.Volume{"Volumes": result})
}

func (s *DockerServer) createVolume(w http.ResponseWriter, r *http.Request) {
	var data struct {
		*docker.CreateVolumeOptions
	}
	defer r.Body.Close()
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	volume := &docker.Volume{
		Name:   data.CreateVolumeOptions.Name,
		Driver: data.CreateVolumeOptions.Driver,
	}
	// If the name is not specified, generate one.  Just using generateID for now
	if len(volume.Name) == 0 {
		volume.Name = s.generateID()
	}
	// If driver is not specified, use local
	if len(volume.Driver) == 0 {
		volume.Driver = "local"
	}
	// Mount point is a default one with name
	volume.Mountpoint = "/var/lib/docker/volumes/" + volume.Name

	// If the volume already exists, don't re-add it.
	exists := false
	s.volMut.Lock()
	if s.volStore != nil {
		_, exists = s.volStore[volume.Name]
	} else {
		// No volumes, create volStore
		s.volStore = make(map[string]*volumeCounter)
	}
	if !exists {
		s.volStore[volume.Name] = &volumeCounter{
			volume: *volume,
			count:  0,
		}
	}
	s.volMut.Unlock()
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(volume)
}

func (s *DockerServer) inspectVolume(w http.ResponseWriter, r *http.Request) {
	s.volMut.RLock()
	defer s.volMut.RUnlock()
	name := mux.Vars(r)["name"]
	vol, err := s.findVolume(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(vol.volume)
}

func (s *DockerServer) findVolume(name string) (*volumeCounter, error) {
	vol, ok := s.volStore[name]
	if !ok {
		return nil, errors.New("no such volume")
	}
	return vol, nil
}

func (s *DockerServer) removeVolume(w http.ResponseWriter, r *http.Request) {
	s.volMut.Lock()
	defer s.volMut.Unlock()
	name := mux.Vars(r)["name"]
	vol, err := s.findVolume(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if vol.count != 0 {
		http.Error(w, "volume in use and cannot be removed", http.StatusConflict)
		return
	}
	delete(s.volStore, vol.volume.Name)
	w.WriteHeader(http.StatusNoContent)
}

func (s *DockerServer) infoDocker(w http.ResponseWriter, r *http.Request) {
	s.cMut.RLock()
	defer s.cMut.RUnlock()
	s.iMut.RLock()
	defer s.iMut.RUnlock()
	var running, stopped, paused int
	for _, c := range s.containers {
		if c.State.Running {
			running++
		} else {
			stopped++
		}
		if c.State.Paused {
			paused++
		}
	}
	var swarmInfo *swarm.Info
	if s.swarm != nil {
		swarmInfo = &swarm.Info{
			NodeID: s.nodeID,
		}
		for _, n := range s.nodes {
			swarmInfo.RemoteManagers = append(swarmInfo.RemoteManagers, swarm.Peer{
				NodeID: n.ID,
				Addr:   n.ManagerStatus.Addr,
			})
		}
	}
	envs := map[string]interface{}{
		"ID":                "AAAA:XXXX:0000:BBBB:AAAA:XXXX:0000:BBBB:AAAA:XXXX:0000:BBBB",
		"Containers":        len(s.containers),
		"ContainersRunning": running,
		"ContainersPaused":  paused,
		"ContainersStopped": stopped,
		"Images":            len(s.images),
		"Driver":            "aufs",
		"DriverStatus":      [][]string{},
		"SystemStatus":      nil,
		"Plugins": map[string]interface{}{
			"Volume": []string{
				"local",
			},
			"Network": []string{
				"bridge",
				"null",
				"host",
			},
			"Authorization": nil,
		},
		"MemoryLimit":        true,
		"SwapLimit":          false,
		"CpuCfsPeriod":       true,
		"CpuCfsQuota":        true,
		"CPUShares":          true,
		"CPUSet":             true,
		"IPv4Forwarding":     true,
		"BridgeNfIptables":   true,
		"BridgeNfIp6tables":  true,
		"Debug":              false,
		"NFd":                79,
		"OomKillDisable":     true,
		"NGoroutines":        101,
		"SystemTime":         "2016-02-25T18:13:10.25870078Z",
		"ExecutionDriver":    "native-0.2",
		"LoggingDriver":      "json-file",
		"NEventsListener":    0,
		"KernelVersion":      "3.13.0-77-generic",
		"OperatingSystem":    "Ubuntu 14.04.3 LTS",
		"OSType":             "linux",
		"Architecture":       "x86_64",
		"IndexServerAddress": "https://index.docker.io/v1/",
		"RegistryConfig": map[string]interface{}{
			"InsecureRegistryCIDRs": []string{},
			"IndexConfigs":          map[string]interface{}{},
			"Mirrors":               nil,
		},
		"InitSha1":          "e2042dbb0fcf49bb9da199186d9a5063cda92a01",
		"InitPath":          "/usr/lib/docker/dockerinit",
		"NCPU":              1,
		"MemTotal":          2099204096,
		"DockerRootDir":     "/var/lib/docker",
		"HttpProxy":         "",
		"HttpsProxy":        "",
		"NoProxy":           "",
		"Name":              "vagrant-ubuntu-trusty-64",
		"Labels":            nil,
		"ExperimentalBuild": false,
		"ServerVersion":     "1.10.1",
		"ClusterStore":      "",
		"ClusterAdvertise":  "",
		"Swarm":             swarmInfo,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(envs)
}

func (s *DockerServer) versionDocker(w http.ResponseWriter, r *http.Request) {
	envs := map[string]interface{}{
		"Version":       "1.10.1",
		"Os":            "linux",
		"KernelVersion": "3.13.0-77-generic",
		"GoVersion":     "go1.4.2",
		"GitCommit":     "9e83765",
		"Arch":          "amd64",
		"ApiVersion":    "1.22",
		"BuildTime":     "2015-12-01T07:09:13.444803460+00:00",
		"Experimental":  false,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(envs)
}

// SwarmAddress returns the address if there's a fake swarm server enabled.
func (s *DockerServer) SwarmAddress() string {
	if s.swarmServer == nil {
		return ""
	}
	return s.swarmServer.listener.Addr().String()
}

func (s *DockerServer) initSwarmNode(listenAddr, advertiseAddr string) (swarm.Node, error) {
	_, portPart, _ := net.SplitHostPort(listenAddr)
	if portPart == "" {
		portPart = "0"
	}
	var err error
	s.swarmServer, err = newSwarmServer(s, fmt.Sprintf("127.0.0.1:%s", portPart))
	if err != nil {
		return swarm.Node{}, err
	}
	if advertiseAddr == "" {
		advertiseAddr = s.SwarmAddress()
	}
	hostPart, portPart, err := net.SplitHostPort(advertiseAddr)
	if err != nil {
		hostPart = advertiseAddr
	}
	if portPart == "" || portPart == "0" {
		_, portPart, _ = net.SplitHostPort(s.SwarmAddress())
	}
	s.nodeID = s.generateID()
	return swarm.Node{
		ID: s.nodeID,
		Status: swarm.NodeStatus{
			Addr:  hostPart,
			State: swarm.NodeStateReady,
		},
		ManagerStatus: &swarm.ManagerStatus{
			Addr: fmt.Sprintf("%s:%s", hostPart, portPart),
		},
	}, nil
}
