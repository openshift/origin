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

	"github.com/blang/semver"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/homedir"

	"github.com/openshift/origin/pkg/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/bootstrap/docker/errors"
	dockerexec "github.com/openshift/origin/pkg/bootstrap/docker/exec"
	"github.com/openshift/origin/pkg/bootstrap/docker/host"
	"github.com/openshift/origin/pkg/bootstrap/docker/run"
	defaultsapi "github.com/openshift/origin/pkg/build/admission/defaults/api"
	cliconfig "github.com/openshift/origin/pkg/cmd/cli/config"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	_ "github.com/openshift/origin/pkg/cmd/server/api/install"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
)

const (
	defaultNodeName        = "localhost"
	initialStatusCheckWait = 4 * time.Second
	serverUpTimeout        = 35
	serverConfigPath       = "/var/lib/origin/openshift.local.config"
	serverMasterConfig     = serverConfigPath + "/master/master-config.yaml"
	serverNodeConfig       = serverConfigPath + "/node-" + defaultNodeName + "/node-config.yaml"
	DefaultDNSPort         = 53
	AlternateDNSPort       = 8053
	cmdDetermineNodeHost   = "for name in %s; do ls /var/lib/origin/openshift.local.config/node-$name &> /dev/null && echo $name && break; done"
	OpenShiftContainer     = "origin"
)

var (
	openShiftContainerBinds = []string{
		"/var/log:/var/log:rw",
		"/var/run:/var/run:rw",
		"/sys:/sys:rw",
		"/sys/fs/cgroup:/sys/fs/cgroup:rw",
		"/dev:/dev",
	}
	BasePorts             = []int{4001, 7001, 8443, 10250}
	RouterPorts           = []int{80, 443}
	DefaultPorts          = append(BasePorts, DefaultDNSPort)
	PortsWithAlternateDNS = append(BasePorts, AlternateDNSPort)
	AllPorts              = append(append(RouterPorts, DefaultPorts...), AlternateDNSPort)
	SocatPidFile          = filepath.Join(homedir.HomeDir(), cliconfig.OpenShiftConfigHomeDir, "socat-8443.pid")
	defaultCertHosts      = []string{
		"127.0.0.1",
		"172.30.0.1",
		"localhost",
		"kubernetes",
		"kubernetes.default",
		"kubernetes.default.svc",
		"kubernetes.default.svc.cluster.local",
		"openshift",
		"openshift.default",
		"openshift.default.svc",
		"openshift.default.svc.cluster.local",
	}
	version15 = semver.MustParse("1.5.0")
	version35 = semver.MustParse("3.5.0")
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
	serverIP      string
}

// StartOptions represent the parameters sent to the start command
type StartOptions struct {
	ServerIP                 string
	AdditionalIPs            []string
	RoutingSuffix            string
	DNSPort                  int
	UseSharedVolume          bool
	Images                   string
	HostVolumesDir           string
	HostConfigDir            string
	HostDataDir              string
	HostPersistentVolumesDir string
	UseExistingConfig        bool
	Environment              []string
	LogLevel                 int
	MetricsHost              string
	LoggingHost              string
	PortForwarding           bool
	HTTPProxy                string
	HTTPSProxy               string
	NoProxy                  []string
	KubeconfigContents       string
	DockerRoot               string
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
	portData, _, _, err := h.runHelper.New().Image(h.image).
		DiscardContainer().
		Privileged().
		HostNetwork().
		HostPid().
		Entrypoint("/bin/bash").
		Command("-c", "cat /proc/net/tcp && ( [ -e /proc/net/tcp6 ] && cat /proc/net/tcp6 || true)").
		Output()
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
		return errors.NewError("cannot start simple server on Docker host").WithCause(err)
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
		return errors.NewError("cannot start simple server on Docker host").WithCause(err)
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
	if len(h.serverIP) > 0 {
		return h.serverIP, nil
	}
	result, _, _, err := h.runHelper.New().Image(h.image).
		DiscardContainer().
		Privileged().
		HostNetwork().
		Command("start", "--print-ip").Output()
	if err != nil {
		return "", err
	}
	h.serverIP = strings.TrimSpace(result)
	return h.serverIP, nil
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
	if len(opt.HTTPProxy) > 0 {
		env = append(env, fmt.Sprintf("HTTP_PROXY=%s", opt.HTTPProxy))
	}
	if len(opt.HTTPSProxy) > 0 {
		env = append(env, fmt.Sprintf("HTTPS_PROXY=%s", opt.HTTPSProxy))
	}
	if len(opt.NoProxy) > 0 {
		env = append(env, fmt.Sprintf("NO_PROXY=%s", strings.Join(opt.NoProxy, ",")))
	}
	if opt.UseSharedVolume {
		binds = append(binds, fmt.Sprintf("%[1]s:%[1]s:shared", opt.HostVolumesDir))
		env = append(env, "OPENSHIFT_CONTAINERIZED=false")
	} else {
		binds = append(binds, "/:/rootfs:ro")
		binds = append(binds, fmt.Sprintf("%[1]s:%[1]s:rslave", opt.HostVolumesDir))
	}
	env = append(env, opt.Environment...)
	binds = append(binds, fmt.Sprintf("%[1]s:%[1]s", opt.DockerRoot))
	binds = append(binds, fmt.Sprintf("%s:/var/lib/origin/openshift.local.config:z", opt.HostConfigDir))

	// Kubelet needs to be able to write to
	// /sys/devices/virtual/net/vethXXX/brport/hairpin_mode, so make this rw, not ro.
	binds = append(binds, "/sys/devices/virtual/net:/sys/devices/virtual/net:rw")

	// Check if a configuration exists before creating one if UseExistingConfig
	// was specified
	var configDir string
	cleanupConfig := func() {
		errors.LogError(os.RemoveAll(configDir))
	}
	skipCreateConfig := false
	if opt.UseExistingConfig {
		masterConfigExists := false
		nodeConfigExists := false
		var err error
		configDir, err = h.copyConfig()
		if err == nil {
			_, err = os.Stat(filepath.Join(configDir, "master", "master-config.yaml"))
			if err == nil {
				masterConfigExists = true
			}
			_, err = os.Stat(filepath.Join(configDir, "node-"+defaultNodeName, "node-config.yaml"))
			if err == nil {
				nodeConfigExists = true
			}
		}
		if masterConfigExists && nodeConfigExists {
			skipCreateConfig = true
		}
	}

	// Create configuration if needed
	if !skipCreateConfig {
		fmt.Fprintf(out, "Creating initial OpenShift configuration\n")
		publicHost := h.publicHost
		if len(publicHost) == 0 {
			publicHost = opt.ServerIP
		}
		createConfigCmd := []string{
			"start",
			fmt.Sprintf("--images=%s", opt.Images),
			fmt.Sprintf("--volume-dir=%s", opt.HostVolumesDir),
			fmt.Sprintf("--dns=0.0.0.0:%d", opt.DNSPort),
			"--write-config=/var/lib/origin/openshift.local.config",
			"--master=127.0.0.1",
			fmt.Sprintf("--hostname=%s", defaultNodeName),
			fmt.Sprintf("--public-master=https://%s:8443", publicHost),
		}
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
		if err := h.updateConfig(configDir, opt); err != nil {
			cleanupConfig()
			return "", errors.NewError("could not update OpenShift configuration").WithCause(err)
		}
	}
	fmt.Fprintf(out, "Starting OpenShift using container '%s'\n", h.containerName)
	startCmd := []string{
		"start",
		fmt.Sprintf("--master-config=%s", serverMasterConfig),
		fmt.Sprintf("--node-config=%s", serverNodeConfig),
	}
	if opt.LogLevel > 0 {
		startCmd = append(startCmd, fmt.Sprintf("--loglevel=%d", opt.LogLevel))
	}

	if opt.PortForwarding {
		if err := h.startSocatTunnel(opt.ServerIP); err != nil {
			return "", err
		}
	}

	if len(opt.HostDataDir) > 0 {
		binds = append(binds, fmt.Sprintf("%s:/var/lib/origin/openshift.local.etcd:z", opt.HostDataDir))
	}
	if len(opt.HostPersistentVolumesDir) > 0 {
		binds = append(binds, fmt.Sprintf("%[1]s:%[1]s", opt.HostPersistentVolumesDir))
		env = append(env, fmt.Sprintf("OPENSHIFT_PV_DIR=%s", opt.HostPersistentVolumesDir))
	}
	_, err := h.runHelper.New().Image(h.image).
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
			return "", errors.NewError("cannot access master readiness URL %s", h.healthzReadyURL(opt.ServerIP)).WithCause(ierr).WithDetails(h.OriginLog())
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

// CheckNodes determines if there is more than one node that corresponds to the
// current machine and removes the one that doesn't match the default node name
func (h *Helper) CheckNodes(kclient kclientset.Interface) error {
	nodes, err := kclient.Core().Nodes().List(kapi.ListOptions{})
	if err != nil {
		return errors.NewError("cannot retrieve nodes").WithCause(err)
	}
	if len(nodes.Items) > 1 {
		glog.V(1).Infof("Found more than one node, will attempt to remove duplicate nodes")
		nodesToRemove := []string{}

		// First, find default node
		defaultNodeMachineId := ""
		for i := 0; i < len(nodes.Items); i++ {
			if nodes.Items[i].Name == defaultNodeName {
				defaultNodeMachineId = nodes.Items[i].Status.NodeInfo.MachineID
				glog.V(5).Infof("machine id for default node is: %s", defaultNodeMachineId)
				break
			}
		}

		for i := 0; i < len(nodes.Items); i++ {
			if nodes.Items[i].Name != defaultNodeName &&
				nodes.Items[i].Status.NodeInfo.MachineID == defaultNodeMachineId {
				glog.V(5).Infof("Found non-default node with duplicate machine id: %s", nodes.Items[i].Name)
				nodesToRemove = append(nodesToRemove, nodes.Items[i].Name)
			}
		}

		for i := 0; i < len(nodesToRemove); i++ {
			glog.V(1).Infof("Deleting extra node %s", nodesToRemove[i])
			err = kclient.Core().Nodes().Delete(nodesToRemove[i], nil)
			if err != nil {
				return errors.NewError("cannot delete duplicate node %s", nodesToRemove[i]).WithCause(err)
			}
		}
	}
	return nil
}

// StartNode starts the OpenShift node as a Docker container
// and returns a directory in the local file system where
// the OpenShift configuration has been copied
func (h *Helper) StartNode(opt *StartOptions, out io.Writer) error {
	binds := openShiftContainerBinds
	env := []string{}
	if opt.UseSharedVolume {
		binds = append(binds, fmt.Sprintf("%[1]s:%[1]s:shared", opt.HostVolumesDir))
		env = append(env, "OPENSHIFT_CONTAINERIZED=false")
	} else {
		binds = append(binds, "/:/rootfs:ro")
		binds = append(binds, fmt.Sprintf("%[1]s:%[1]s:rslave", opt.HostVolumesDir))
	}
	env = append(env, opt.Environment...)

	kubeconfig := "/var/lib/origin/openshift.local.config/node/node-bootstrap.kubeconfig"

	fmt.Fprintf(out, "Starting OpenShift Node using container '%s'\n", h.containerName)
	startCmd := []string{
		"start", "node", "--bootstrap",
		fmt.Sprintf("--kubeconfig=%s", kubeconfig),
	}
	if opt.LogLevel > 0 {
		startCmd = append(startCmd, fmt.Sprintf("--loglevel=%d", opt.LogLevel))
	}

	_, err := h.runHelper.New().Image(h.image).
		Name(h.containerName).
		Privileged().
		HostNetwork().
		HostPid().
		Bind(binds...).
		Env(env...).
		Command(startCmd...).
		Copy(map[string][]byte{
			kubeconfig: []byte(opt.KubeconfigContents),
		}).
		Start()
	if err != nil {
		return errors.NewError("cannot start OpenShift Node daemon").WithCause(err)
	}

	// Wait a minimum amount of time and check whether we're still running. If not, we know the daemon didn't start
	time.Sleep(initialStatusCheckWait)
	_, running, err := h.dockerHelper.GetContainerState(h.containerName)
	if err != nil {
		return errors.NewError("cannot get state of OpenShift container %s", h.containerName).WithCause(err)
	}
	if !running {
		return ErrOpenShiftFailedToStart(h.containerName).WithDetails(h.OriginLog())
	}

	// Wait until the API server is listening
	fmt.Fprintf(out, "Waiting for server to start listening\n")
	masterHost := fmt.Sprintf("%s:10250", opt.ServerIP)
	if err = cmdutil.WaitForSuccessfulDial(true, "tcp", masterHost, 200*time.Millisecond, 1*time.Second, serverUpTimeout); err != nil {
		return ErrTimedOutWaitingForStart(h.containerName).WithDetails(h.OriginLog())
	}

	fmt.Fprintf(out, "OpenShift server started\n")
	return nil
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

func (h *Helper) GetNodeConfigFromLocalDir(configDir string) (*configapi.NodeConfig, string, error) {
	configPath := filepath.Join(configDir, fmt.Sprintf("node-%s", defaultNodeName), "node-config.yaml")
	glog.V(1).Infof("Reading node config from %s", configPath)
	cfg, err := configapilatest.ReadNodeConfig(configPath)
	if err != nil {
		glog.V(1).Infof("Could not read node config: %v", err)
		return nil, "", err
	}
	return cfg, configPath, nil
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

func (h *Helper) serverVersion() (semver.Version, error) {
	versionText, _, _, err := h.runHelper.New().Image(h.image).
		Command("version").
		Output()
	if err != nil {
		return semver.Version{}, err
	}
	lines := strings.Split(versionText, "\n")
	versionStr := ""
	for _, line := range lines {
		if strings.HasPrefix(line, "openshift") {
			parts := strings.SplitN(line, " ", 2)
			versionStr = strings.TrimLeft(parts[1], "v")
			break
		}
	}
	return semver.Parse(versionStr)
}

func useDNSIP(version semver.Version) bool {
	// Ignore pre-release portion
	version.Pre = []semver.PRVersion{}

	if version.Major == 1 {
		return version.GTE(version15)
	}
	return version.GTE(version35)
}

func (h *Helper) updateConfig(configDir string, opt *StartOptions) error {
	cfg, configPath, err := h.GetConfigFromLocalDir(configDir)
	if err != nil {
		return err
	}

	if len(opt.RoutingSuffix) > 0 {
		cfg.RoutingConfig.Subdomain = opt.RoutingSuffix
	} else {
		cfg.RoutingConfig.Subdomain = fmt.Sprintf("%s.nip.io", opt.ServerIP)
	}

	if len(opt.MetricsHost) > 0 && cfg.AssetConfig != nil {
		cfg.AssetConfig.MetricsPublicURL = fmt.Sprintf("https://%s/hawkular/metrics", opt.MetricsHost)
	}

	if len(opt.LoggingHost) > 0 && cfg.AssetConfig != nil {
		cfg.AssetConfig.LoggingPublicURL = fmt.Sprintf("https://%s", opt.LoggingHost)
	}

	if len(opt.HTTPProxy) > 0 || len(opt.HTTPSProxy) > 0 || len(opt.NoProxy) > 0 {
		if cfg.AdmissionConfig.PluginConfig == nil {
			cfg.AdmissionConfig.PluginConfig = map[string]configapi.AdmissionPluginConfig{}
		}

		var buildDefaults *defaultsapi.BuildDefaultsConfig
		buildDefaultsConfig, ok := cfg.AdmissionConfig.PluginConfig[defaultsapi.BuildDefaultsPlugin]
		if !ok {
			buildDefaultsConfig = configapi.AdmissionPluginConfig{}
		}
		if buildDefaultsConfig.Configuration != nil {
			buildDefaults = buildDefaultsConfig.Configuration.(*defaultsapi.BuildDefaultsConfig)
		}
		if buildDefaults == nil {
			buildDefaults = &defaultsapi.BuildDefaultsConfig{}
			buildDefaultsConfig.Configuration = buildDefaults
		}
		buildDefaults.GitHTTPProxy = opt.HTTPProxy
		buildDefaults.GitHTTPSProxy = opt.HTTPSProxy
		buildDefaults.GitNoProxy = strings.Join(opt.NoProxy, ",")
		varsToSet := map[string]string{
			"HTTP_PROXY":  opt.HTTPProxy,
			"http_proxy":  opt.HTTPProxy,
			"HTTPS_PROXY": opt.HTTPSProxy,
			"https_proxy": opt.HTTPSProxy,
			"NO_PROXY":    strings.Join(opt.NoProxy, ","),
			"no_proxy":    strings.Join(opt.NoProxy, ","),
		}
		for k, v := range varsToSet {
			buildDefaults.Env = append(buildDefaults.Env, kapi.EnvVar{
				Name:  k,
				Value: v,
			})
		}
		cfg.AdmissionConfig.PluginConfig[defaultsapi.BuildDefaultsPlugin] = buildDefaultsConfig
	}

	cfg.JenkinsPipelineConfig.TemplateName = "jenkins-persistent"

	cfgBytes, err := configapilatest.WriteYAML(cfg)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(configPath, cfgBytes, 0644)
	if err != nil {
		return err
	}
	err = h.hostHelper.UploadFileToContainer(configPath, serverMasterConfig)
	if err != nil {
		return err
	}
	nodeCfg, nodeConfigPath, err := h.GetNodeConfigFromLocalDir(configDir)
	if err != nil {
		return err
	}
	version, err := h.serverVersion()
	if err != nil {
		return err
	}
	if useDNSIP(version) {
		nodeCfg.DNSIP = "172.30.0.1"
	} else {
		nodeCfg.DNSIP = ""
	}
	nodeCfg.DNSBindAddress = ""
	cfgBytes, err = configapilatest.WriteYAML(nodeCfg)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(nodeConfigPath, cfgBytes, 0644)
	if err != nil {
		return err
	}
	return h.hostHelper.UploadFileToContainer(nodeConfigPath, serverNodeConfig)
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
