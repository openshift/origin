package testing

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	haproxyConfigDir = "/var/lib/haproxy/conf"

	serverName = "_dynamic-pod-1"

	OnePodAndOneDynamicServerBackendTemplate = `1
# be_id be_name srv_id srv_name srv_addr srv_op_state srv_admin_state srv_uweight srv_iweight srv_time_since_last_change srv_check_status srv_check_result srv_check_health srv_check_state srv_agent_state bk_f_forced_id srv_f_forced_id srv_fqdn srv_port
9 %s 1 pod:test-1-l8x8w:test-service:172.17.0.3:1234 172.17.0.3 2 4 256 1 8117 6 3 4 6 0 0 0 - 1234
9 %s 2 _dynamic-pod-1 172.4.0.4 2 4 256 1 8117 6 3 4 6 0 0 0 - 1234
`
)

type fakeHAProxyMap map[string]string

type fakeHAProxy struct {
	socketFile  string
	backendName string
	maps        map[string]fakeHAProxyMap
	backends    map[string]string
	lock        sync.Mutex
	shutdown    bool
	commands    []string
}

func startFakeHAProxyServer(prefix string) (*fakeHAProxy, error) {
	f, err := ioutil.TempFile(os.TempDir(), prefix)
	if err != nil {
		return nil, err
	}

	name := f.Name()
	os.Remove(name)
	server := newFakeHAProxy(name, "")
	server.Start()
	return server, nil
}

func StartFakeServerForTest(t *testing.T) *fakeHAProxy {
	name := fmt.Sprintf("fake-haproxy-%s", t.Name())
	server, err := startFakeHAProxyServer(name)
	if err != nil {
		t.Errorf("%s error: %v", t.Name(), err)
	}
	return server
}

func newFakeHAProxy(sockFile, backendName string) *fakeHAProxy {
	if len(backendName) == 0 {
		backendName = "be_edge_http:_hapcm_blueprint_pool:_blueprint-edge-route-1"
	}
	p := &fakeHAProxy{
		socketFile:  sockFile,
		backendName: backendName,
		maps:        make(map[string]fakeHAProxyMap, 0),
		backends:    make(map[string]string, 0),
		shutdown:    false,
		commands:    make([]string, 0),
	}
	p.initialize()
	return p
}

func (p *fakeHAProxy) SocketFile() string {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.socketFile
}

func (p *fakeHAProxy) Reset() {
	p.lock.Lock()
	p.commands = make([]string, 0)
	p.lock.Unlock()
	p.initialize()
}

func (p *fakeHAProxy) Commands() []string {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.commands
}

func (p *fakeHAProxy) Start() {
	started := make(chan bool)
	go func() error {
		listener, err := net.Listen("unix", p.socketFile)
		if err != nil {
			return err
		}

		started <- true
		for {
			p.lock.Lock()
			shutdown := p.shutdown
			p.lock.Unlock()
			if shutdown {
				return nil
			}
			conn, err := listener.Accept()
			if err != nil {
				return err
			}
			go p.process(conn)
		}
	}()

	// wait for server to indicate it started up.
	<-started
}

func (p *fakeHAProxy) Stop() {
	p.lock.Lock()
	p.shutdown = true
	sockFile := p.socketFile
	p.lock.Unlock()
	go func() {
		timeout := time.Duration(10) * time.Second
		net.DialTimeout("unix", sockFile, timeout)
		if len(sockFile) > 0 {
			os.Remove(sockFile)
		}
	}()
}

func (p *fakeHAProxy) initialize() {
	redirectMap := map[string]string{
		`^route\.edge\.test(:[0-9]+)?(/.*)?$`:          `0x559a137bb720 ^route\.edge\.test(:[0-9]+)?(/.*)?$ be_edge_http:ns1:edge-redirect-to-https`,
		`^redirect\.blueprints\.test(:[0-9]+)?(/.*)?$`: `0x559a137bb7e0 ^redirect\.blueprints\.test(:[0-9]+)?(/.*)?$ be_edge_http:blueprints:blueprint-redirect-to-https`,
	}

	passthruMap := map[string]string{
		`^route\.passthrough\.test(:[0-9]+)?(/.*)?$`: `0x559a137bf730 ^route\.passthrough\.test(:[0-9]+)?(/.*)?$ 1`,
	}

	httpMap := map[string]string{
		`^route\.allow-http\.test(:[0-9]+)?(/.*)?$`: `0x559a137b4c10 ^route\.allow-http\.test(:[0-9]+)?(/.*)?$ be_edge_http:default:test-http-allow`,
	}

	tcpMap := map[string]string{
		`^route\.reencrypt\.test(:[0-9]+)?(/.*)?$`:     `0x559a137b4700 ^route\.reencrypt\.test(:[0-9]+)?(/.*)?$ be_secure:default:test-reencrypt`,
		`^reencrypt\.blueprints\.org(:[0-9]+)?(/.*)?$`: `0x559a1400f8a0 ^reencrypt\.blueprints\.org(:[0-9]+)?(/.*)?$ be_secure:blueprints:blueprint-reencrypt`,
		`^route\.passthrough\.test(:[0-9]+)?(/.*)?$`:   `0x559a1400f960 ^route\.passthrough\.test(:[0-9]+)?(/.*)?$ be_tcp:default:test-passthrough`,
	}

	edgeReencryptMap := map[string]string{
		`^www\.example2\.com(:[0-9]+)?(/.*)?$`:         `0x559a140103e0 ^www\.example2\.com(:[0-9]+)?(/.*)?$ be_edge_http:default:example-route`,
		`^something\.edge\.test(:[0-9]+)?(/.*)?$`:      `0x559a14010450 ^something\.edge\.test(:[0-9]+)?(/.*)?$ be_edge_http:default:wildcard-redirect-to-https`,
		`^route\.reencrypt\.test(:[0-9]+)?(/.*)?$`:     `0x559a14010510 ^route\.reencrypt\.test(:[0-9]+)?(/.*)?$ be_secure:default:test-reencrypt`,
		`^reencrypt\.blueprints\.org(:[0-9]+)?(/.*)?$`: `0x559a140105c0 ^reencrypt\.blueprints\.org(:[0-9]+)?(/.*)?$ be_secure:blueprints:blueprint-reencrypt`,
		`^redirect\.blueprints\.org(:[0-9]+)?(/.*)?$`:  `0x559a140109a0 ^route\.edge\.test(:[0-9]+)?(/.*)?$ be_edge_http:default:test-https`,
		`^route\.edge\.test(:[0-9]+)?(/.*)?$`:          `0x559a140109a0 ^route\.edge\.test(:[0-9]+)?(/.*)?$ be_edge_http:default:test-https`,
	}

	mapNames := map[string]fakeHAProxyMap{
		"os_route_http_redirect.map": redirectMap,
		"os_sni_passthrough.map":     passthruMap,
		"os_http_be.map":             httpMap,
		"os_tcp_be.map":              tcpMap,
		"os_edge_reencrypt_be.map":   edgeReencryptMap,
	}

	p.lock.Lock()
	defer p.lock.Unlock()
	for k, v := range mapNames {
		name := path.Join(haproxyConfigDir, k)
		p.maps[name] = v
	}
}

func (p *fakeHAProxy) showInfo() string {
	return `Name: HAProxy
Version: 1.8.1
Release_date: 2017/12/03
Nbproc: 1
Process_num: 1
Pid: 84
Uptime: 0d5h23m33s
Uptime_sec: 19413
Memmax_MB: 0
PoolAlloc_MB: 0
PoolUsed_MB: 0
PoolFailed: 0
Ulimit-n: 40260
Maxsock: 40260
Maxconn: 20000
Hard_maxconn: 20000
CurrConns: 0
CumConns: 3945
CumReq: 3947
MaxSslConns: 0
CurrSslConns: 0
CumSslConns: 7765
Maxpipes: 0
PipesUsed: 0
PipesFree: 0
ConnRate: 0
ConnRateLimit: 0
MaxConnRate: 2
SessRate: 0
SessRateLimit: 0
MaxSessRate: 2
SslRate: 0
SslRateLimit: 0
MaxSslRate: 1
SslFrontendKeyRate: 0
SslFrontendMaxKeyRate: 1
SslFrontendSessionReuse_pct: 0
SslBackendKeyRate: 0
SslBackendMaxKeyRate: 2
SslCacheLookups: 0
SslCacheMisses: 0
CompressBpsIn: 0
CompressBpsOut: 0
CompressBpsRateLim: 0
ZlibMemUsage: 0
MaxZlibMemUsage: 0
Tasks: 278
Run_queue: 0
Idle_pct: 100
node: f27
`
}

func (p *fakeHAProxy) listMaps() string {
	return `# id (file) description
1 (/var/lib/haproxy/conf/os_route_http_redirect.map) pattern loaded from file '/var/lib/haproxy/conf/os_route_http_redirect.map' used by map at file '/var/lib/haproxy/conf/haproxy.config' line 68
5 (/var/lib/haproxy/conf/os_sni_passthrough.map) pattern loaded from file '/var/lib/haproxy/conf/os_sni_passthrough.map' used by map at file '/var/lib/haproxy/conf/haproxy.config' line 87
-1 (/var/lib/haproxy/conf/os_http_be.map) pattern loaded from file '/var/lib/haproxy/conf/os_http_be.map' used by map at file '/var/lib/haproxy/conf/haproxy.config' line 71
-1 (/var/lib/haproxy/conf/os_tcp_be.map) pattern loaded from file '/var/lib/haproxy/conf/os_tcp_be.map' used by map at file '/var/lib/haproxy/conf/haproxy.config' line 88
-1 (/var/lib/haproxy/conf/os_edge_reencrypt_be.map) pattern loaded from file '/var/lib/haproxy/conf/os_edge_reencrypt_be.map' used by map at file '/var/lib/haproxy/conf/haproxy.config' line 127, by map at file '/var/lib/haproxy/conf/haproxy.config' line 163

`
}

func (p *fakeHAProxy) showMap(name string) string {
	lines := []string{}
	p.lock.Lock()
	defer p.lock.Unlock()
	if m, ok := p.maps[name]; ok {
		for _, v := range m {
			lines = append(lines, v)
		}
	} else {
		lines = append(lines, "Unknown map identifier. Please use #<id> or <file>.")
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func (p *fakeHAProxy) addMap(name, k, v string) string {
	lines := []string{}
	p.lock.Lock()
	defer p.lock.Unlock()
	if m, ok := p.maps[name]; !ok {
		lines = append(lines, "Unknown map identifier. Please use #<id> or <file>.")
		lines = append(lines, "")
	} else {
		m[k] = v
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func (p *fakeHAProxy) delMap(name, id string) string {
	id = strings.Trim(id, "#")
	p.lock.Lock()
	defer p.lock.Unlock()
	if m, ok := p.maps[name]; ok {
		matchingKeys := []string{}
		for k, v := range m {
			if strings.HasPrefix(v, id) {
				matchingKeys = append(matchingKeys, k)
			}
		}

		for _, v := range matchingKeys {
			delete(m, v)
		}
	}

	return fmt.Sprintf("del map %s\n", name)
}

func (p *fakeHAProxy) listBackends() string {
	return `# name
be_sni
be_no_sni
openshift_default
be_edge_http:_hapcm_blueprint_pool:_blueprint-edge-route-1
be_edge_http:_hapcm_blueprint_pool:_blueprint-edge-route-2
be_edge_http:_hapcm_blueprint_pool:_blueprint-edge-route-3
be_http:_hapcm_blueprint_pool:_blueprint-http-route-1
be_http:_hapcm_blueprint_pool:_blueprint-http-route-2
be_http:_hapcm_blueprint_pool:_blueprint-http-route-3
be_tcp:_hapcm_blueprint_pool:_blueprint-passthrough-route-1
be_tcp:_hapcm_blueprint_pool:_blueprint-passthrough-route-2
be_tcp:_hapcm_blueprint_pool:_blueprint-passthrough-route-3
be_edge_http:blueprints:blueprint-redirect-to-https
be_secure:blueprints:blueprint-reencrypt
be_edge_http:default:example-route
be_edge_http:default:test-http-allow
be_edge_http:default:test-https
be_edge_http:default:test-https-only
be_tcp:default:test-passthrough
be_secure:default:test-reencrypt
be_edge_http:default:wildcard-redirect-to-https
`
}

func (p *fakeHAProxy) showServers(name string) string {
	p.lock.Lock()
	defer p.lock.Unlock()

	onePodAndOneDynamicServerBackends := map[string]string{
		"be_edge_http:_hapcm_blueprint_pool:_blueprint-edge-route-1": "",
		"be_edge_http:_hapcm_blueprint_pool:_blueprint-edge-route-2": "",
		"be_edge_http:_hapcm_blueprint_pool:_blueprint-edge-route-3": "",

		"be_http:_hapcm_blueprint_pool:_blueprint-http-route-1": "",
		"be_http:_hapcm_blueprint_pool:_blueprint-http-route-2": "",
		"be_http:_hapcm_blueprint_pool:_blueprint-http-route-3": "",

		"be_tcp:_hapcm_blueprint_pool:_blueprint-passthrough-route-1": "",
		"be_tcp:_hapcm_blueprint_pool:_blueprint-passthrough-route-2": "",
		"be_tcp:_hapcm_blueprint_pool:_blueprint-passthrough-route-3": "",

		"be_edge_http:blueprints:blueprint-redirect-to-https": "",
		"be_secure:blueprints:blueprint-reencrypt":            "",
		"be_edge_http:default:example-route":                  "",

		"be_edge_http:default:test-http-allow": "",
		"be_edge_http:default:test-https":      "",
		"be_edge_http:default:test-https-only": "",

		"be_tcp:default:test-passthrough":  "",
		"be_secure:default:test-reencrypt": "",

		"be_edge_http:default:wildcard-redirect-to-https": "",
	}

	if name != p.backendName {
		if _, ok := onePodAndOneDynamicServerBackends[name]; ok {
			return fmt.Sprintf(OnePodAndOneDynamicServerBackendTemplate, name, name)
		}
		if len(name) > 0 {
			return fmt.Sprintf("Can't find backend.\n")
		}
	}

	return `1
# be_id be_name srv_id srv_name srv_addr srv_op_state srv_admin_state srv_uweight srv_iweight srv_time_since_last_change srv_check_status srv_check_result srv_check_health srv_check_state srv_agent_state bk_f_forced_id srv_f_forced_id srv_fqdn srv_port
9 be_edge_http:_hapcm_blueprint_pool:_blueprint-edge-route-1 1 _dynamic-pod-1 172.17.0.3 2 4 256 1 8117 6 3 4 6 0 0 0 - 8080
9 be_edge_http:_hapcm_blueprint_pool:_blueprint-edge-route-1 2 _dynamic-pod-2 172.17.0.3 2 5 256 1 8117 6 3 0 14 0 0 0 - 8080
9 be_edge_http:_hapcm_blueprint_pool:_blueprint-edge-route-1 3 _dynamic-pod-3 172.4.0.4 0 5 1 1 8206 1 0 0 14 0 0 0 - 8765
9 be_edge_http:_hapcm_blueprint_pool:_blueprint-edge-route-1 4 _dynamic-pod-4 172.4.0.4 0 5 1 1 8206 1 0 0 14 0 0 0 - 8765
9 be_edge_http:_hapcm_blueprint_pool:_blueprint-edge-route-1 5 _dynamic-pod-5 172.17.0.2 2 4 256 1 8206 6 3 4 6 0 0 0 - 8080
`
}

func (p *fakeHAProxy) setServer(name string, options []string) string {
	if len(name) == 0 {
		return fmt.Sprintf("Require 'backend/server'.\n")
	}

	p.lock.Lock()
	defer p.lock.Unlock()
	existingServer := fmt.Sprintf("%s/%s", p.backendName, serverName)
	if name != existingServer {
		return fmt.Sprintf("No such server.\n")
	}

	return fmt.Sprintf("\n")
}

func (p *fakeHAProxy) process(conn net.Conn) error {
	readBuffer := make([]byte, 1024)
	nread, err := conn.Read(readBuffer)
	if err != nil {
		response := fmt.Sprintf("error: %v", err)
		conn.Write([]byte(response))
		return err
	}

	response := ""
	cmd := string(bytes.Trim(readBuffer[0:nread], " "))
	cmd = strings.Trim(cmd, "\n")
	p.lock.Lock()
	p.commands = append(p.commands, cmd)
	p.lock.Unlock()

	if strings.HasPrefix(cmd, "show info") {
		response = p.showInfo()
	} else if strings.HasPrefix(cmd, "show map") {
		name := strings.Trim(cmd[len("show map"):], " ")
		if len(name) == 0 {
			response = p.listMaps()
		} else {
			response = p.showMap(name)
		}
	} else if strings.HasPrefix(cmd, "show backend") {
		response = p.listBackends()
	} else if strings.HasPrefix(cmd, "add map") {
		params := strings.Trim(cmd[len("add map"):], " ")
		vals := strings.Split(params, " ")
		if len(vals) < 3 {
			response = fmt.Sprintf("'add map' expects three parameters: map identifier, key and value.\n")
		} else {
			response = p.addMap(vals[0], vals[1], vals[2])
		}
	} else if strings.HasPrefix(cmd, "del map") {
		params := strings.Trim(cmd[len("del map"):], " ")
		vals := strings.Split(params, " ")
		if len(vals) < 2 {
			response = fmt.Sprintf("This command expects two parameters: map identifier and key.\n")
		} else {
			response = p.delMap(vals[0], vals[1])
		}
	} else if strings.HasPrefix(cmd, "show servers state") {
		name := strings.Trim(cmd[len("show servers state"):], " ")
		response = p.showServers(name)
	} else if strings.HasPrefix(cmd, "set server") {
		params := strings.Trim(cmd[len("set server"):], " ")
		name := ""
		vals := strings.Split(params, " ")
		if len(vals) > 0 {
			name = vals[0]
		}
		response = p.setServer(name, vals[1:])
	} else {
		response = fmt.Sprintf("Unknown command. Please enter one of the following commands only :\nhelp\n...\n")
	}

	if _, err := conn.Write([]byte(response)); err != nil {
		return err
	}
	return conn.Close()
}
