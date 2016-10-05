package util

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kcontainer "k8s.io/kubernetes/pkg/kubelet/container"

	"github.com/openshift/origin/pkg/diagnostics/types"
	sdnplugin "github.com/openshift/origin/pkg/sdn/plugin"
)

type LogInterface struct {
	Result types.DiagnosticResult
	Logdir string
}

func (l *LogInterface) LogNode(kubeClient *kclient.Client) {
	l.LogSystem()
	l.LogServices()

	l.Run("brctl show", "bridges")
	l.Run("docker ps -a", "docker-ps")
	l.Run(fmt.Sprintf("ovs-ofctl -O OpenFlow13 dump-flows %s", sdnplugin.BR), "flows")
	l.Run(fmt.Sprintf("ovs-ofctl -O OpenFlow13 show %s", sdnplugin.BR), "ovs-show")
	l.Run("tc qdisc show", "tc-qdisc")
	l.Run("tc class show", "tc-class")
	l.Run("tc filter show", "tc-filter")
	l.Run("systemctl cat docker.service", "docker-unit-file")
	l.logDockerNetworkFile()
}

func (l *LogInterface) LogMaster() {
	l.LogSystem()
	l.LogServices()

	l.Run("oc get nodes -o yaml", "nodes")
	l.Run("oc get pods --all-namespaces -o yaml", "pods")
	l.Run("oc get services --all-namespaces -o yaml", "services")
	l.Run("oc get endpoints --all-namespaces -o yaml", "endpoints")
	l.Run("oc get routes --all-namespaces -o yaml", "routes")
	l.Run("oc get clusternetwork -o yaml", "clusternetwork")
	l.Run("oc get hostsubnets -o yaml", "hostsubnets")
	l.Run("oc get netnamespaces -o yaml", "netnamespaces")
}

func (l *LogInterface) LogServices() {
	type Service struct {
		name   string
		suffix string
		args   string
	}
	allServices := []Service{
		{suffix: "master", args: "master"},
		{suffix: "master-controllers", args: "master controllers"},
		{suffix: "api", args: "master api"},
		{suffix: "node", args: "node"},
	}
	foundServices := []Service{}

	for _, sysDir := range []string{"/etc/systemd/system", "/usr/lib/systemd/system"} {
		for _, prefix := range []string{"openshift", "origin", "atomic-openshift"} {
			for _, service := range allServices {
				service.name = fmt.Sprintf("%s-%s", prefix, service.suffix)
				servicePath := fmt.Sprintf("%s/%s.service", sysDir, service.name)
				if _, err := os.Stat(servicePath); err == nil {
					foundServices = append(foundServices, service)
				}
			}
		}
	}

	for _, service := range foundServices {
		l.Run(fmt.Sprintf("journalctl -u %s -r -n 5000", service.name), fmt.Sprintf("journal-%s", service.name))
		l.Run(fmt.Sprintf("systemctl show %s", service.name), fmt.Sprintf("systemctl-show-%s", service.name))

		configFile := l.getConfigFileForService(service.name, service.args)
		if len(configFile) > 0 {
			l.Run(fmt.Sprintf("cat %s", configFile), fmt.Sprintf("config-%s", service.name))
		}
	}
}

func (l *LogInterface) LogSystem() {
	l.Run("journalctl --boot -r -n 5000", "journal-boot")
	l.Run("nmcli --nocheck -f all dev show", "nmcli-dev")
	l.Run("nmcli --nocheck -f all con show", "nmcli-con")
	l.Run("ip addr show", "addresses")
	l.Run("ip route show", "routes")
	l.Run("ip neighbor show", "arp")
	l.Run("iptables-save", "iptables")
	l.Run("cat /etc/hosts", "hosts")
	l.Run("cat /etc/resolv.conf", "resolv.conf")
	l.Run("lsmod", "modules")
	l.Run("sysctl -a", "sysctl")
	l.Run("oc version", "version")
	l.Run("docker version", "version")
	l.Run("cat /etc/system-release-cpe", "version")

	l.logNetworkInterfaces()
}

func (l *LogInterface) Run(cmd, outfile string) {
	if len(cmd) == 0 {
		return
	}

	if _, err := os.Stat(l.Logdir); err != nil {
		if err = os.MkdirAll(l.Logdir, 0700); err != nil {
			l.Result.Error("DLogNet1001", err, fmt.Sprintf("Creating log directory %q failed: %s", l.Logdir, err))
			return
		}
	}
	outPath := filepath.Join(l.Logdir, outfile)
	out, err := os.OpenFile(outPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		l.Result.Error("DLogNet1002", err, fmt.Sprintf("Opening file %q failed: %s", outPath, err))
		return
	}
	defer out.Close()

	newcmd := []string{"sh", "-c", cmd}
	first := newcmd[0]
	rest := newcmd[1:]
	c := exec.Command(first, rest...)
	c.Env = os.Environ()
	c.Stdout = out
	c.Stderr = out
	if err = c.Run(); err != nil {
		// Ignore errors, some commands like nmcli, etc. may not exist on the node
		return
	}
}

func (l *LogInterface) logDockerNetworkFile() {
	out, err := exec.Command("systemctl", "cat", "docker.service").CombinedOutput()
	if err != nil {
		l.Run(fmt.Sprintf("echo %s", string(out)), "docker-network-file")
		return
	}

	re := regexp.MustCompile("EnvironmentFile=-(.*openshift-sdn.*)")
	match := re.FindStringSubmatch(string(out))
	if len(match) > 1 {
		dockerNetworkFile := match[1]
		l.Run(fmt.Sprintf("cat %s", dockerNetworkFile), "docker-network-file")
	}
}

func (l *LogInterface) logNetworkInterfaces() {
	filepath.Walk("/etc/sysconfig/network-scripts/", func(path string, f os.FileInfo, err error) error {
		if !strings.HasPrefix(path, "ifcfg-") {
			return nil
		}
		l.Run(fmt.Sprintf("\necho %s", path), "ifcfg")
		l.Run(fmt.Sprintf("cat %s", path), "ifcfg")
		return nil
	})
}

func (l *LogInterface) logPodInfo(kubeClient *kclient.Client) {
	pods, _, err := GetLocalAndNonLocalDiagnosticPods(kubeClient)
	if err != nil {
		l.Result.Error("DLogNet1003", err, err.Error())
		return
	}

	for _, pod := range pods {
		if len(pod.Status.ContainerStatuses) == 0 {
			continue
		}

		containerID := kcontainer.ParseContainerID(pod.Status.ContainerStatuses[0].ContainerID).ID
		out, err := exec.Command("docker", []string{"inspect", "-f", "'{{.State.Pid}}'", containerID}...).Output()
		if err != nil {
			l.Result.Error("DLogNet1004", err, fmt.Sprintf("Fetching pid for container %q failed: %s", containerID, err))
			continue
		}
		pid := strings.Trim(string(out[:]), "\n")

		p := LogInterface{
			Result: l.Result,
			Logdir: filepath.Join(l.Logdir, NetworkDiagPodLogDirPrefix, pod.Name),
		}
		p.Run(fmt.Sprintf("nsenter -n -t %s -- ip addr show", pid), "addresses")
		p.Run(fmt.Sprintf("nsenter -n -t %s -- ip route show", pid), "routes")
	}
}

func (l *LogInterface) getConfigFileForService(serviceName, serviceArgs string) string {
	out, err := exec.Command("sh", []string{"-c", fmt.Sprintf("echo $(ps wwaux | grep -v grep | sed -ne 's/.*openshift start %s --.*config=\\([^ ]*.yaml\\).*/\\1/p')", serviceArgs)}...).Output()
	if err != nil || len(out) == 0 {
		out, err = exec.Command("sh", []string{"-c", fmt.Sprintf("echo $(systemctl show -p ExecStart %s | sed -ne 's/.*--config=\\([^ ]*\\).*/\\1/p')", serviceName)}...).Output()
		if err != nil || len(out) == 0 {
			out, err = exec.Command("sh", []string{"-c", fmt.Sprintf("cat $(systemctl show %s | grep EnvironmentFile | sed -ne 's/EnvironmentFile=\\([^ ]*\\).*/\\1/p' | sed -ne 's/CONFIG_FILE=//p')", serviceName)}...).Output()
		}
	}

	if err != nil {
		l.Result.Error("DLogNet1005", err, fmt.Sprintf("Failed to fetch service config for %q: %s", serviceName, err))
		return ""
	}
	return string(out[:])
}
