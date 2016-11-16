package openshift

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/util/homedir"

	"github.com/openshift/origin/pkg/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/bootstrap/docker/errors"
	dockerexec "github.com/openshift/origin/pkg/bootstrap/docker/exec"
	"github.com/openshift/origin/pkg/bootstrap/docker/host"
	"github.com/openshift/origin/pkg/bootstrap/docker/run"
	cliconfig "github.com/openshift/origin/pkg/cmd/cli/config"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	_ "github.com/openshift/origin/pkg/cmd/server/api/install"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

const (
	initialStatusCheckWait = 4 * time.Second
	serverUpTimeout        = 35
	serverConfigPath       = "/var/lib/origin/openshift.local.config"
	serverMasterConfig     = serverConfigPath + "/master/master-config.yaml"
	DefaultDNSPort         = 53
	AlternateDNSPort       = 8053
	cmdDetermineNodeHost   = "for name in %s; do ls /var/lib/origin/openshift.local.config/node-$name &> /dev/null && echo $name && break; done"
	OpenShiftContainer     = "origin"
)

var (
	openShiftContainerBinds = []string{
		"/var/log:/var/log:rw",
		"/var/run:/var/run:rw",
		"/sys:/sys:ro",
		"/var/lib/docker:/var/lib/docker",
	}
	BasePorts             = []int{4001, 7001, 8443, 10250}
	RouterPorts           = []int{80, 443}
	DefaultPorts          = append(BasePorts, DefaultDNSPort)
	PortsWithAlternateDNS = append(BasePorts, AlternateDNSPort)
	SocatPidFile          = filepath.Join(homedir.HomeDir(), cliconfig.OpenShiftConfigHomeDir, "socat-8443.pid")
)

// Helper contains methods and utilities to help with OpenShift startup
type Helper struct {
	hostHelper    *host.HostHelper
	dockerHelper  *dockerhelper.Helper
	execHelper    *dockerexec.ExecHelper
	runHelper     *run.RunHelper
	client        *docker.Client
	publicHost    string
	image         string
	containerName string
	routingSuffix string
}

// StartOptions represent the parameters sent to the start command
type StartOptions struct {
	ServerIP           string
	RouterIP           string
	DNSPort            int
	UseSharedVolume    bool
	SetPropagationMode bool
	Images             string
	HostVolumesDir     string
	HostConfigDir      string
	HostDataDir        string
	UseExistingConfig  bool
	Environment        []string
	LogLevel           int
	MetricsHost        string
	LoggingHost        string
	PortForwarding     bool
}

// NewHelper creates a new OpenShift helper
func NewHelper(client *docker.Client, hostHelper *host.HostHelper, image, containerName, publicHostname, routingSuffix string) *Helper {
	return &Helper{
		client:        client,
		dockerHelper:  dockerhelper.NewHelper(client, nil),
		execHelper:    dockerexec.NewExecHelper(client, containerName),
		hostHelper:    hostHelper,
		runHelper:     run.NewRunHelper(client),
		image:         image,
		containerName: containerName,
		publicHost:    publicHostname,
		routingSuffix: routingSuffix,
	}
}

func (h *Helper) TestPorts(ports []int) error {
	portData, _, err := h.runHelper.New().Image(h.image).
		DiscardContainer().
		Privileged().
		HostNetwork().
		HostPid().
		Entrypoint("/bin/bash").
		Command("-c", "cat /proc/net/tcp && ( [ -e /proc/net/tcp6 ] && cat /proc/net/tcp6 || true)").
		CombinedOutput()
	if err != nil {
		return errors.NewError("Cannot get TCP port information from Kubernetes host").WithCause(err)
	}
	return checkPortsInUse(portData, ports)
}

func testIPDial(ip string) error {
	// Attempt to connect to test container
	testHost := fmt.Sprintf("%s:8443", ip)
	glog.V(4).Infof("Attempting to dial %s", testHost)
	if err := cmdutil.WaitForSuccessfulDial(false, "tcp", testHost, 200*time.Millisecond, 1*time.Second, 10); err != nil {
		glog.V(2).Infof("Dial error: %v", err)
		return err
	}
	glog.V(4).Infof("Successfully dialed %s", testHost)
	return nil
}

func (h *Helper) TestIP(ip string) error {

	// Start test server on host
	id, err := h.runHelper.New().Image(h.image).
		Privileged().
		HostNetwork().
		Entrypoint("socat").
		Command("TCP-LISTEN:8443,crlf,reuseaddr,fork", "SYSTEM:\"echo 'hello world'\"").Start()
	if err != nil {
		return errors.NewError("cannnot start simple server on Docker host").WithCause(err)
	}
	defer func() {
		errors.LogError(h.dockerHelper.StopAndRemoveContainer(id))
	}()
	return testIPDial(ip)
}

func (h *Helper) TestForwardedIP(ip string) error {
	// Start test server on host
	id, err := h.runHelper.New().Image(h.image).
		PortForward(8443, 8443).
		Entrypoint("socat").
		Command("TCP-LISTEN:8443,crlf,reuseaddr,fork", "SYSTEM:\"echo 'hello world'\"").Start()
	if err != nil {
		return errors.NewError("cannnot start simple server on Docker host").WithCause(err)
	}
	defer func() {
		errors.LogError(h.dockerHelper.StopAndRemoveContainer(id))
	}()
	return testIPDial(ip)
}

func (h *Helper) DetermineNodeHost(hostConfigDir string, names ...string) (string, error) {
	result, _, _, err := h.runHelper.New().Image(h.image).
		DiscardContainer().
		Privileged().
		HostNetwork().
		Entrypoint("/bin/bash").
		Bind(fmt.Sprintf("%s:/var/lib/origin/openshift.local.config", hostConfigDir)).
		Command("-c", fmt.Sprintf(cmdDetermineNodeHost, strings.Join(names, " "))).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result), nil
}

// ServerIP retrieves the Server ip through the openshift start command
func (h *Helper) ServerIP() (string, error) {
	result, _, _, err := h.runHelper.New().Image(h.image).
		DiscardContainer().
		Privileged().
		HostNetwork().
		Command("start", "--print-ip").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result), nil
}

// OtherIPs tries to find other IPs besides the argument IP for the Docker host
func (h *Helper) OtherIPs(excludeIP string) ([]string, error) {
	result, _, _, err := h.runHelper.New().Image(h.image).
		DiscardContainer().
		Privileged().
		HostNetwork().
		Entrypoint("hostname").
		Command("-I").Output()
	if err != nil {
		return nil, err
	}

	candidates := strings.Split(result, " ")
	resultIPs := []string{}
	for _, ip := range candidates {
		if ip != excludeIP && !strings.Contains(ip, ":") { // for now, ignore IPv6
			resultIPs = append(resultIPs, ip)
		}
	}
	return resultIPs, nil
}

// Start starts the OpenShift master as a Docker container
// and returns a directory in the local file system where
// the OpenShift configuration has been copied
func (h *Helper) Start(opt *StartOptions, out io.Writer) (string, error) {
	// Ensure that socat is available locally
	if opt.PortForwarding {
		err := CheckSocat()
		if err != nil {
			return "", err
		}
	}

	binds := openShiftContainerBinds
	env := []string{}
	if opt.UseSharedVolume {
		binds = append(binds, fmt.Sprintf("%[1]s:%[1]s:shared", opt.HostVolumesDir))
		env = append(env, "OPENSHIFT_CONTAINERIZED=false")
	} else {
		binds = append(binds, "/:/rootfs:ro")
		propagationMode := ""
		if opt.SetPropagationMode {
			propagationMode = ":rslave"
		}
		binds = append(binds, fmt.Sprintf("%[1]s:%[1]s%[2]s", opt.HostVolumesDir, propagationMode))
	}
	env = append(env, opt.Environment...)
	binds = append(binds, fmt.Sprintf("%s:/var/lib/origin/openshift.local.config:z", opt.HostConfigDir))

	// Check if a configuration exists before creating one if UseExistingConfig
	// was specified
	var configDir string
	cleanupConfig := func() {
		errors.LogError(os.RemoveAll(configDir))
	}
	skipCreateConfig := false
	if opt.UseExistingConfig {
		var err error
		configDir, err = h.copyConfig()
		if err == nil {
			_, err = os.Stat(filepath.Join(configDir, "master", "master-config.yaml"))
			if err == nil {
				skipCreateConfig = true
			}
		}
	}

	// Create configuration if needed
	var nodeHost string
	if !skipCreateConfig {
		fmt.Fprintf(out, "Creating initial OpenShift configuration\n")
		createConfigCmd := []string{
			"start",
			fmt.Sprintf("--images=%s", opt.Images),
			fmt.Sprintf("--volume-dir=%s", opt.HostVolumesDir),
			fmt.Sprintf("--dns=0.0.0.0:%d", opt.DNSPort),
			"--write-config=/var/lib/origin/openshift.local.config",
		}
		if opt.PortForwarding {
			internalIP, err := h.ServerIP()
			if err != nil {
				return "", err
			}
			nodeHost = internalIP
			createConfigCmd = append(createConfigCmd, fmt.Sprintf("--master=%s", internalIP))
			createConfigCmd = append(createConfigCmd, fmt.Sprintf("--public-master=https://%s:8443", opt.ServerIP))
		} else {
			nodeHost = opt.ServerIP
			createConfigCmd = append(createConfigCmd, fmt.Sprintf("--master=%s", opt.ServerIP))
			if len(h.publicHost) > 0 {
				createConfigCmd = append(createConfigCmd, fmt.Sprintf("--public-master=https://%s:8443", h.publicHost))
			}
		}
		createConfigCmd = append(createConfigCmd, fmt.Sprintf("--hostname=%s", nodeHost))
		_, err := h.runHelper.New().Image(h.image).
			Privileged().
			DiscardContainer().
			HostNetwork().
			HostPid().
			Bind(binds...).
			Env(env...).
			Command(createConfigCmd...).Run()
		if err != nil {
			return "", errors.NewError("could not create OpenShift configuration").WithCause(err)
		}
		configDir, err = h.copyConfig()
		if err != nil {
			return "", errors.NewError("could not copy OpenShift configuration").WithCause(err)
		}
		err = h.updateConfig(configDir, opt.RouterIP, opt.MetricsHost, opt.LoggingHost)
		if err != nil {
			cleanupConfig()
			return "", errors.NewError("could not update OpenShift configuration").WithCause(err)
		}
	}
	if nodeHost == "" {
		if opt.PortForwarding {
			var err error
			nodeHost, err = h.ServerIP()
			if err != nil {
				return "", err
			}
		} else {
			var err error
			hostName, err := h.hostHelper.Hostname()
			if err != nil {
				return "", err
			}
			nodeHost, err = h.DetermineNodeHost(opt.HostConfigDir, opt.ServerIP, hostName)
			if err != nil {
				return "", err
			}
		}
	}
	masterConfig, nodeConfig, err := h.getOpenShiftConfigFiles(nodeHost)
	if err != nil {
		cleanupConfig()
		return "", errors.NewError("could not get OpenShift configuration file paths").WithCause(err)
	}

	fmt.Fprintf(out, "Starting OpenShift using container '%s'\n", h.containerName)
	startCmd := []string{
		"start",
		fmt.Sprintf("--master-config=%s", masterConfig),
		fmt.Sprintf("--node-config=%s", nodeConfig),
	}
	if opt.LogLevel > 0 {
		startCmd = append(startCmd, fmt.Sprintf("--loglevel=%d", opt.LogLevel))
	}

	if opt.PortForwarding {
		err = h.startSocatTunnel()
		if err != nil {
			return "", err
		}
	}

	if len(opt.HostDataDir) > 0 {
		binds = append(binds, fmt.Sprintf("%s:/var/lib/origin/openshift.local.etcd:z", opt.HostDataDir))
	}
	_, err = h.runHelper.New().Image(h.image).
		Name(h.containerName).
		Privileged().
		HostNetwork().
		HostPid().
		Bind(binds...).
		Env(env...).
		Command(startCmd...).
		Start()
	if err != nil {
		return "", errors.NewError("cannot start OpenShift daemon").WithCause(err)
	}

	// Wait a minimum amount of time and check whether we're still running. If not, we know the daemon didn't start
	time.Sleep(initialStatusCheckWait)
	_, running, err := h.dockerHelper.GetContainerState(h.containerName)
	if err != nil {
		return "", errors.NewError("cannot get state of OpenShift container %s", h.containerName).WithCause(err)
	}
	if !running {
		return "", ErrOpenShiftFailedToStart(h.containerName).WithDetails(h.OriginLog())
	}

	// Wait until the API server is listening
	fmt.Fprintf(out, "Waiting for API server to start listening\n")
	masterHost := fmt.Sprintf("%s:8443", opt.ServerIP)
	if err = cmdutil.WaitForSuccessfulDial(true, "tcp", masterHost, 200*time.Millisecond, 1*time.Second, serverUpTimeout); err != nil {
		return "", errors.NewError("timed out waiting for OpenShift container %q \nWARNING: %s:8443 may be blocked by firewall rules", h.containerName, opt.ServerIP).WithSolution("Ensure that you can access %s from your machine", masterHost).WithDetails(h.OriginLog())
	}
	// Check for healthz endpoint to be ready
	client, err := masterHTTPClient(configDir)
	if err != nil {
		return "", err
	}
	for {
		resp, ierr := client.Get(h.healthzReadyURL(opt.ServerIP))
		if ierr != nil {
			return "", errors.NewError("cannot access master readiness URL %s", h.healthzReadyURL(opt.ServerIP)).WithCause(err).WithDetails(h.OriginLog())
		}
		if resp.StatusCode == http.StatusOK {
			break
		}
		if resp.StatusCode == http.StatusServiceUnavailable ||
			resp.StatusCode == http.StatusForbidden {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		var responseBody string
		body, rerr := ioutil.ReadAll(resp.Body)
		if rerr == nil {
			responseBody = string(body)
		}
		return "", errors.NewError("server is not ready. Response (%d): %s", resp.StatusCode, responseBody).WithCause(ierr).WithDetails(h.OriginLog())
	}
	fmt.Fprintf(out, "OpenShift server started\n")
	return configDir, nil
}

func (h *Helper) OriginLog() string {
	log := h.dockerHelper.ContainerLog(h.containerName, 10)
	if len(log) > 0 {
		return fmt.Sprintf("Last 10 lines of %q container log:\n%s\n", h.containerName, log)
	}
	return fmt.Sprintf("No log available from %q container\n", h.containerName)
}

func (h *Helper) healthzReadyURL(ip string) string {
	return fmt.Sprintf("%s/healthz/ready", h.Master(ip))
}

func (h *Helper) Master(ip string) string {
	return fmt.Sprintf("https://%s:8443", ip)
}

func masterHTTPClient(localConfig string) (*http.Client, error) {
	caCert := filepath.Join(localConfig, "master", "ca.crt")
	transport, err := cmdutil.TransportFor(caCert, "", "")
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}, nil
}

// copyConfig copies the OpenShift configuration directory from the
// server directory into a local temporary directory.
func (h *Helper) copyConfig() (string, error) {
	tempDir, err := ioutil.TempDir("", "openshift-config")
	if err != nil {
		return "", err
	}
	glog.V(1).Infof("Copying OpenShift config to local directory %s", tempDir)
	if err = h.hostHelper.DownloadDirFromContainer(serverConfigPath, tempDir); err != nil {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			glog.V(2).Infof("Error removing temporary config dir %s: %v", tempDir, removeErr)
		}
		return "", err
	}

	return tempDir, nil
}

func (h *Helper) GetConfigFromLocalDir(configDir string) (*configapi.MasterConfig, string, error) {
	configPath := filepath.Join(configDir, "master", "master-config.yaml")
	glog.V(1).Infof("Reading master config from %s", configPath)
	cfg, err := configapilatest.ReadMasterConfig(configPath)
	if err != nil {
		glog.V(1).Infof("Could not read master config: %v", err)
		return nil, "", err
	}
	return cfg, configPath, nil
}

func GetConfigFromContainer(client *docker.Client) (*configapi.MasterConfig, error) {
	r, err := dockerhelper.StreamFileFromContainer(client, OpenShiftContainer, serverMasterConfig)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	config := &configapi.MasterConfig{}
	err = configapilatest.ReadYAMLInto(data, config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func (h *Helper) updateConfig(configDir, routerIP, metricsHost, loggingHost string) error {
	cfg, configPath, err := h.GetConfigFromLocalDir(configDir)
	if err != nil {
		return err
	}

	if len(h.routingSuffix) > 0 {
		cfg.RoutingConfig.Subdomain = h.routingSuffix
	} else {
		cfg.RoutingConfig.Subdomain = fmt.Sprintf("%s.xip.io", routerIP)
	}

	if len(metricsHost) > 0 && cfg.AssetConfig != nil {
		cfg.AssetConfig.MetricsPublicURL = fmt.Sprintf("https://%s/hawkular/metrics", metricsHost)
	}

	if len(loggingHost) > 0 && cfg.AssetConfig != nil {
		cfg.AssetConfig.LoggingPublicURL = fmt.Sprintf("https://%s", loggingHost)
	}

	cfgBytes, err := configapilatest.WriteYAML(cfg)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(configPath, cfgBytes, 0644)
	if err != nil {
		return err
	}
	return h.hostHelper.UploadFileToContainer(configPath, serverMasterConfig)
}

func (h *Helper) getOpenShiftConfigFiles(hostname string) (string, string, error) {
	return serverMasterConfig,
		fmt.Sprintf("/var/lib/origin/openshift.local.config/node-%s/node-config.yaml", hostname),
		nil
}

func checkPortsInUse(data string, ports []int) error {
	used := getUsedPorts(data)
	conflicts := []int{}
	for _, port := range ports {
		if _, inUse := used[port]; inUse {
			conflicts = append(conflicts, port)
		}
	}
	if len(conflicts) > 0 {
		return ErrPortsNotAvailable(conflicts)
	}
	return nil
}

func getUsedPorts(data string) map[int]struct{} {
	ports := map[int]struct{}{}
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		// discard lines that don't contain connection data
		if !strings.Contains(parts[0], ":") {
			continue
		}
		glog.V(5).Infof("Determining port in use from: %s", line)
		localAddress := strings.Split(parts[1], ":")
		if len(localAddress) < 2 {
			continue
		}
		state := parts[3]
		if state != "0A" { // only look at connections that are listening
			continue
		}
		port, err := strconv.ParseInt(localAddress[1], 16, 0)
		if err == nil {
			ports[int(port)] = struct{}{}
		}
	}
	glog.V(2).Infof("Used ports in container: %#v", ports)
	return ports
}
