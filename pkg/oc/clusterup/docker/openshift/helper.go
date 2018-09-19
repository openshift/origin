package openshift

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/errors"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/run"
)

const (
	defaultNodeName      = "localhost"
	DefaultDNSPort       = 8053
	DefaultSvcCIDR       = "172.30.0.0/16"
	cmdDetermineNodeHost = "for name in %s; do ls /var/lib/origin/openshift.local.config/node-$name &> /dev/null && echo $name && break; done"

	// TODO: Figure out why cluster up relies on this name
	ContainerName = "origin"
	Namespace     = "openshift"
)

var (
	BasePorts    = []int{4001, 7001, 8443, 10250, DefaultDNSPort}
	RouterPorts  = []int{80, 443}
	AllPorts     = append(RouterPorts, BasePorts...)
	SocatPidFile = filepath.Join(homedir.HomeDir(), kclientcmd.RecommendedHomeDir, "socat-8443.pid")
)

// Helper contains methods and utilities to help with OpenShift startup
type Helper struct {
	dockerHelper  *dockerhelper.Helper
	runHelper     *run.RunHelper
	image         string
	containerName string
	serverIP      string
}

// NewHelper creates a new OpenShift helper
func NewHelper(dockerHelper *dockerhelper.Helper, image, containerName string) *Helper {
	return &Helper{
		dockerHelper:  dockerHelper,
		runHelper:     run.NewRunHelper(dockerHelper),
		image:         image,
		containerName: containerName,
	}
}

func (h *Helper) TestPorts(ports []int) error {
	_, portData, _, _, err := h.runHelper.New().Image(h.image).
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

func (h *Helper) DetermineNodeHost(hostConfigDir string, names ...string) (string, error) {
	_, result, _, _, err := h.runHelper.New().Image(h.image).
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
	_, result, _, _, err := h.runHelper.New().Image(h.image).
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
	_, result, _, _, err := h.runHelper.New().Image(h.image).
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
		if len(strings.TrimSpace(ip)) == 0 {
			continue
		}
		if ip != excludeIP && !strings.Contains(ip, ":") { // for now, ignore IPv6
			resultIPs = append(resultIPs, ip)
		}
	}
	return resultIPs, nil
}

func (h *Helper) Master(ip string) string {
	return fmt.Sprintf("https://%s:8443", ip)
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
