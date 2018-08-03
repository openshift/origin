package haproxy

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/golang/glog"
)

// BackendServerState indicates the state for a haproxy backend server.
type BackendServerState string

const (
	// BackendServerStateReady indicates a server is ready.
	BackendServerStateReady BackendServerState = "ready"

	// BackendServerStateDrain indicates a server is ready but draining.
	BackendServerStateDrain BackendServerState = "drain"

	// BackendServerStateDown indicates a server is down.
	BackendServerStateDown BackendServerState = "down"

	// BackendServerStateMaint indicates a server is under maintainence.
	BackendServerStateMaint BackendServerState = "maint"

	// ListBackendsCommand is the command to get a list of all backends.
	ListBackendsCommand = "show backend"

	// GetServersStateCommand gets the state of all servers. This can be
	// optionally filtered by backends by passing a backend name.
	GetServersStateCommand = "show servers state"

	// SetServerCommand sets server specific information and state.
	SetServerCommand = "set server"

	// showBackendHeader is the haproxy backend list csv output header.
	showBackendHeader = "name"

	// serverStateHeader is the haproxy server state csv output header.
	serversStateHeader = "be_id be_name srv_id srv_name srv_addr srv_op_state srv_admin_state srv_uweight srv_iweight srv_time_since_last_change srv_check_status srv_check_result srv_check_health srv_check_state srv_agent_state bk_f_forced_id srv_f_forced_id srv_fqdn srv_port"
)

// backendEntry is an entry in the list of backends returned from haproxy.
type backendEntry struct {
	Name string `csv:"name"`
}

// serverStateInfo represents the state of a specific backend server.
type serverStateInfo struct {
	BackendID   string `csv:"be_id"`
	BackendName string `csv:"be_name"`
	ID          string `csv:"srv_id"`
	Name        string `csv:"srv_name"`
	IPAddress   string `csv:"srv_addr"`

	OperationalState    int32 `csv:"srv_op_state"`
	AdministrativeState int32 `csv:"srv_admin_state"`
	UserVisibleWeight   int32 `csv:"srv_uweight"`
	InitialWeight       int32 `csv:"srv_iweight"`

	TimeSinceLastChange   int `csv:"srv_time_since_last_change"`
	LastHealthCheckStatus int `csv:"srv_check_status"`
	LastHealthCheckResult int `csv:"srv_check_result"`
	CheckHealth           int `csv:"srv_check_health"`
	CheckHealthState      int `csv:"srv_check_state"`
	AgentCheckState       int `csv:"srv_agent_state"`

	BackendIDForced int `csv:"bk_f_forced_id"`
	IDForced        int `csv:"srv_f_forced_id"`

	FQDN string `csv:"srv_fqdn"`
	Port int    `csv:"srv_port"`
}

// BackendServerInfo represents a server [endpoint] for a haproxy backend.
type BackendServerInfo struct {
	Name          string
	FQDN          string
	IPAddress     string
	Port          int
	CurrentWeight int32
	InitialWeight int32
	State         BackendServerState
}

// Backend represents a specific haproxy backend.
type Backend struct {
	name    string
	servers map[string]*backendServer

	client *Client
}

// backendServer is internally used for managing a haproxy backend server.
type backendServer struct {
	BackendServerInfo

	updatedIPAddress string
	updatedPort      int
	updatedWeight    string // as it can be a percentage.
	updatedState     BackendServerState
}

// buildHAProxyBackends builds and returns a list of haproxy backends.
func buildHAProxyBackends(c *Client) ([]*Backend, error) {
	entries := []*backendEntry{}
	converter := NewCSVConverter(showBackendHeader, &entries, nil)
	_, err := c.RunCommand(ListBackendsCommand, converter)
	if err != nil {
		return []*Backend{}, err
	}

	backends := make([]*Backend, len(entries))
	for k, v := range entries {
		backends[k] = newBackend(v.Name, c)
	}

	return backends, nil
}

// newBackend returns a new Backend representing a haproxy backend.
func newBackend(name string, c *Client) *Backend {
	return &Backend{
		name:    name,
		servers: make(map[string]*backendServer),
		client:  c,
	}
}

// Name returns the name of this haproxy backend.
func (b *Backend) Name() string {
	return b.name
}

// Reset resets the cached server info in this haproxy backend.
func (b *Backend) Reset() {
	b.servers = make(map[string]*backendServer)
}

// Refresh refreshs our internal state for this haproxy backend.
func (b *Backend) Refresh() error {
	entries := []*serverStateInfo{}
	converter := NewCSVConverter(serversStateHeader, &entries, stripVersionNumber)
	cmd := fmt.Sprintf("%s %s", GetServersStateCommand, b.Name())
	_, err := b.client.RunCommand(cmd, converter)
	if err != nil {
		return err
	}

	b.servers = make(map[string]*backendServer)
	for _, v := range entries {
		info := BackendServerInfo{
			Name:          v.Name,
			IPAddress:     v.IPAddress,
			Port:          v.Port,
			FQDN:          v.FQDN,
			CurrentWeight: v.UserVisibleWeight,
			InitialWeight: v.InitialWeight,
			State:         getManagedServerState(v),
		}

		b.servers[v.Name] = newBackendServer(info)
	}

	return nil
}

// SetRoutingKey sets the cookie routing key for the haproxy backend.
func (b *Backend) SetRoutingKey(k string) error {
	glog.V(4).Infof("Setting routing key for %s", b.name)

	cmd := fmt.Sprintf("set dynamic-cookie-key backend %s %s", b.name, k)
	if err := b.executeCommand(cmd); err != nil {
		return fmt.Errorf("setting routing key for backend %s: %v", b.name, err)
	}

	cmd = fmt.Sprintf("enable dynamic-cookie backend %s", b.name)
	if err := b.executeCommand(cmd); err != nil {
		return fmt.Errorf("enabling routing key for backend %s: %v", b.name, err)
	}

	return nil
}

// executeCommand runs a command using the haproxy dynamic config api client.
func (b *Backend) executeCommand(cmd string) error {
	responseBytes, err := b.client.Execute(cmd)
	if err != nil {
		return err
	}

	response := strings.TrimSpace(string(responseBytes))
	if len(response) > 0 {
		return errors.New(response)
	}

	return nil
}

// Disable stops serving traffic for all servers for a haproxy backend.
func (b *Backend) Disable() error {
	if _, err := b.Servers(); err != nil {
		return err
	}

	for _, s := range b.servers {
		if err := b.DisableServer(s.Name); err != nil {
			return err
		}
	}

	return nil
}

// EnableServer enables serving traffic with a haproxy backend server.
func (b *Backend) EnableServer(name string) error {
	glog.V(4).Infof("Enabling server %s with ready state", name)
	return b.UpdateServerState(name, BackendServerStateReady)
}

// DisableServer stops serving traffic for a haproxy backend server.
func (b *Backend) DisableServer(name string) error {
	glog.V(4).Infof("Disabling server %s with maint state", name)
	return b.UpdateServerState(name, BackendServerStateMaint)
}

// Commit commits all the pending changes made to a haproxy backend.
func (b *Backend) Commit() error {
	for _, s := range b.servers {
		if err := s.ApplyChanges(b.name, b.client); err != nil {
			return err
		}
	}

	b.Reset()
	return nil
}

// Servers returns the servers for this haproxy backend.
func (b *Backend) Servers() ([]BackendServerInfo, error) {
	if len(b.servers) == 0 {
		if err := b.Refresh(); err != nil {
			return []BackendServerInfo{}, err
		}
	}

	serverInfo := make([]BackendServerInfo, len(b.servers))
	i := 0
	for _, s := range b.servers {
		serverInfo[i] = s.BackendServerInfo
		i++
	}

	return serverInfo, nil
}

// UpdateServerInfo updates the information for a haproxy backend server.
func (b *Backend) UpdateServerInfo(id, ipaddr, port string, weight int32, relativeWeight bool) error {
	server, err := b.FindServer(id)
	if err != nil {
		return err
	}

	if len(ipaddr) > 0 {
		server.updatedIPAddress = ipaddr
	}
	if n, err := strconv.Atoi(port); err == nil && n > 0 {
		server.updatedPort = n
	}
	if weight > -1 {
		suffix := ""
		if relativeWeight {
			suffix = "%"
		}
		server.updatedWeight = fmt.Sprintf("%v%s", weight, suffix)
	}

	return nil
}

// UpdateServerState specifies what should be the state of a haproxy backend
// server when all the changes made to the backend committed.
func (b *Backend) UpdateServerState(id string, state BackendServerState) error {
	server, err := b.FindServer(id)
	if err != nil {
		return err
	}

	server.updatedState = state
	return nil
}

// FindServer returns a specific haproxy backend server if found.
func (b *Backend) FindServer(id string) (*backendServer, error) {
	if _, err := b.Servers(); err != nil {
		return nil, err
	}

	if s, ok := b.servers[id]; ok {
		return s, nil
	}

	return nil, fmt.Errorf("no server found for id: %s", id)
}

// newBackendServer returns a BackendServer representing a haproxy backend server.
func newBackendServer(info BackendServerInfo) *backendServer {
	return &backendServer{
		BackendServerInfo: info,

		updatedIPAddress: info.IPAddress,
		updatedPort:      info.Port,
		updatedWeight:    strconv.Itoa(int(info.CurrentWeight)),
		updatedState:     info.State,
	}
}

// ApplyChanges applies all the local backend server changes.
func (s *backendServer) ApplyChanges(backendName string, client *Client) error {
	// Build the haproxy dynamic config API commands.
	commands := []string{}

	cmdPrefix := fmt.Sprintf("%s %s/%s", SetServerCommand, backendName, s.Name)

	if s.updatedIPAddress != s.IPAddress || s.updatedPort != s.Port {
		cmd := fmt.Sprintf("%s addr %s", cmdPrefix, s.updatedIPAddress)
		if s.updatedPort != s.Port {
			cmd = fmt.Sprintf("%s port %v", cmd, s.updatedPort)
		}
		commands = append(commands, cmd)
	}

	if s.updatedWeight != strconv.Itoa(int(s.CurrentWeight)) {
		// Build and execute the haproxy dynamic config API command.
		cmd := fmt.Sprintf("%s weight %s", cmdPrefix, s.updatedWeight)
		commands = append(commands, cmd)
	}

	state := string(s.updatedState)
	if s.updatedState == BackendServerStateDown {
		// BackendServerStateDown for a server can't be set!
		state = ""
	}

	if len(state) > 0 && s.updatedState != s.State {
		cmd := fmt.Sprintf("%s state %s", cmdPrefix, state)
		commands = append(commands, cmd)
	}

	// Execute all the commands.
	for _, cmd := range commands {
		if err := s.executeCommand(cmd, client); err != nil {
			return err
		}
	}

	return nil
}

// executeCommand runs a server change command and handles the response.
func (s *backendServer) executeCommand(cmd string, client *Client) error {
	responseBytes, err := client.Execute(cmd)
	if err != nil {
		return err
	}

	response := strings.TrimSpace(string(responseBytes))
	if len(response) == 0 {
		return nil
	}

	okPrefixes := []string{"IP changed from", "no need to change"}
	for _, prefix := range okPrefixes {
		if strings.HasPrefix(response, prefix) {
			return nil
		}
	}

	return fmt.Errorf("setting server info with %s : %s", cmd, response)
}

// stripVersionNumber strips off the first line if it is a version number.
func stripVersionNumber(data []byte) ([]byte, error) {
	// The first line contains the version number, so we need to strip
	// that off in order to use the CSV converter.
	//  Example:
	//  > show servers state be_sni
	//  1
	//  # be_id be_name srv_id srv_name ... srv_fqdn srv_port
	//  4 be_sni 1 fe_sni 127.0.0.1 2 0 1 1 46518 1 0 2 0 0 0 0 - 10444
	//
	idx := bytes.Index(data, []byte("\n"))
	if idx > -1 {
		version := string(data[:idx])
		if _, err := strconv.ParseInt(version, 10, 0); err == nil {
			if idx+1 < len(data) {
				return data[idx+1:], nil
			}
		}
	}

	return data, nil
}

// getManagedServerState returns the "managed" state for a backend server.
func getManagedServerState(s *serverStateInfo) BackendServerState {
	if (s.AdministrativeState & 0x01) == 0x01 {
		return BackendServerStateMaint
	}
	if (s.AdministrativeState & 0x08) == 0x08 {
		return BackendServerStateDrain
	}

	if s.OperationalState == 0 {
		maintainenceMasks := []int32{0x01, 0x02, 0x04, 0x20}
		for _, m := range maintainenceMasks {
			if (s.AdministrativeState & m) == m {
				return BackendServerStateMaint
			}
		}

		drainingMasks := []int32{0x08, 0x10}
		for _, m := range drainingMasks {
			if (s.AdministrativeState & m) == m {
				return BackendServerStateDrain
			}
		}

		return BackendServerStateDown
	}

	return BackendServerStateReady
}
