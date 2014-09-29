package router

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
)

const (
	ConfigTemplate   = "/var/lib/haproxy/conf/haproxy_template.conf"
	ConfigFile       = "/var/lib/haproxy/conf/haproxy.config"
	HostMapFile      = "/var/lib/haproxy/conf/host_be.map"
	HostMapSniFile   = "/var/lib/haproxy/conf/host_be_sni.map"
	HostMapResslFile = "/var/lib/haproxy/conf/host_be_ressl.map"
	HostMapWsFile    = "/var/lib/haproxy/conf/host_be_ws.map"
)

func writeServer(f *os.File, id string, s *Endpoint) {
	f.WriteString(fmt.Sprintf("  server %s %s:%s check inter 5000ms\n", id, s.IP, s.Port))
}

func WriteConfig() {
	//ReadRoutes()
	hf, herr := os.Create(HostMapFile)
	if herr != nil {
		fmt.Println("Error creating host map file - %s", herr.Error())
		os.Exit(1)
	}
	dat, terr := ioutil.ReadFile(ConfigTemplate)
	if terr != nil {
		fmt.Println("Error reading from template configuration - %s", terr.Error())
		os.Exit(1)
	}
	f, err := os.Create(ConfigFile)
	if err != nil {
		fmt.Println("Error opening file haproxy.conf - %s", err.Error())
		os.Exit(1)
	}
	f.WriteString(string(dat))
	for frontendname, frontend := range GlobalRoutes {
		if len(frontend.HostAliases) == 0 || len(frontend.EndpointTable) == 0 {
			continue
		}
		for host := range frontend.HostAliases {
			if frontend.HostAliases[host] != "" {
				hf.WriteString(fmt.Sprintf("%s %s\n", frontend.HostAliases[host], frontendname))
			}
		}

		f.WriteString(fmt.Sprintf("backend be_%s\n  mode http\n  balance leastconn\n  timeout check 5000ms\n", frontendname))
		for seid, se := range frontend.EndpointTable {
			writeServer(f, seid, &se)
		}
		f.WriteString("\n")
	}
	f.Close()
}

func execCmd(cmd *exec.Cmd) (string, bool) {
	out, err := cmd.CombinedOutput()
	var return_str string
	if err != nil {
		fmt.Sprintf(return_str, "Error executing command.\n%s", err.Error())
	} else {
		return_str = string(out)
	}
	return return_str, err == nil
}

func ReloadRouter() {
	old_pid, oerr := ioutil.ReadFile("/var/lib/haproxy/run/haproxy.pid")
	cmd := exec.Command("/usr/local/sbin/haproxy", "-f", "/var/lib/haproxy/conf/haproxy.config", "-p", "/var/lib/haproxy/run/haproxy.pid")
	if oerr == nil {
		cmd = exec.Command("/usr/local/sbin/haproxy", "-f", "/var/lib/haproxy/conf/haproxy.config", "-p", "/var/lib/haproxy/run/haproxy.pid", "-sf", string(old_pid))
	}
	out, _ := execCmd(cmd)
	fmt.Println(out)
}
