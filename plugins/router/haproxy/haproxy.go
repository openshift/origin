package haproxy

import (
	"fmt"
	"github.com/golang/glog"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/openshift/origin/pkg/router"
)

const (
	ConfigTemplate   = "/var/lib/haproxy/conf/haproxy_template.conf"
	ConfigFile       = "/var/lib/haproxy/conf/haproxy.config"
	HostMapFile      = "/var/lib/haproxy/conf/host_be.map"
	HostMapSniFile   = "/var/lib/haproxy/conf/host_be_sni.map"
	HostMapResslFile = "/var/lib/haproxy/conf/host_be_ressl.map"
	HostMapWsFile    = "/var/lib/haproxy/conf/host_be_ws.map"
)

type HaproxyRouter struct {
	*router.Routes
}

func NewRouter() *HaproxyRouter {
	r := &HaproxyRouter{&router.Routes{}}
	r.ReadRoutes()
	return r
}

func (hr *HaproxyRouter) writeServer(f *os.File, id string, s *router.Endpoint) {
	f.WriteString(fmt.Sprintf("  server %s %s:%s check inter 5000ms\n", id, s.IP, s.Port))
}

func (hr *HaproxyRouter) WriteConfig() {
	//ReadRoutes()
	hf, herr := os.Create(HostMapFile)
	if herr != nil {
		glog.Fatalf("Error creating host map file - %s", herr.Error())
	}
	dat, terr := ioutil.ReadFile(ConfigTemplate)
	if terr != nil {
		glog.Fatalf("Error reading from template configuration - %s", terr.Error())
	}
	f, err := os.Create(ConfigFile)
	if err != nil {
		glog.Fatalf("Error opening file haproxy.conf - %s", err.Error())
	}
	f.WriteString(string(dat))
	for frontendname, frontend := range hr.GlobalRoutes {
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
			hr.writeServer(f, seid, &se)
		}
		f.WriteString("\n")
	}
	f.Close()
}

func execCmd(cmd *exec.Cmd) (string, bool) {
	out, err := cmd.CombinedOutput()
	var returnStr string
	if err != nil {
		fmt.Sprintf(returnStr, "Error executing command.\n%s", err.Error())
	} else {
		returnStr = string(out)
	}
	return returnStr, err == nil
}

func (hr *HaproxyRouter) ReloadRouter() bool {
	oldPid, oerr := ioutil.ReadFile("/var/lib/haproxy/run/haproxy.pid")
	cmd := exec.Command("/usr/local/sbin/haproxy", "-f", ConfigFile, "-p", "/var/lib/haproxy/run/haproxy.pid")
	if oerr == nil {
		cmd = exec.Command("/usr/local/sbin/haproxy", "-f", ConfigFile, "-p", "/var/lib/haproxy/run/haproxy.pid", "-sf", string(oldPid))
	}
	out, err := execCmd(cmd)
	if err == false {
		glog.Errorf("Error reloading haproxy router - %s", out)
	}
	return err
}
