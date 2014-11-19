package bind

import (
	"github.com/golang/glog"
	"os"
	dnsapi "github.com/openshift/origin/pkg/dns"
	"bytes"
	"fmt"
	"io"
	"os/exec"
)

type BindDNS struct {}

func NewBindDNS() *BindDNS{
	return &BindDNS{}
}

const (
	Config string = "/var/named/v3.rhcloud.com.zone"
	ConfigTemplate string = "/var/named/template.zone"
)

func (dns *BindDNS) Run(){
	//TODO: bad idea? Should probably just be running in the pod and we issue a reload?  What about monitoring of this pod?
	cmd := exec.Command("/usr/sbin/named", "-fg", "-unamed")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func (dns *BindDNS) WriteConfig(dnsConfig *dnsapi.DNSConfig) error {
	//copy the template
	templateFile, err := os.Open(ConfigTemplate)

	if err != nil {
		return err
	}

	defer templateFile.Close()

	destFile, err := os.Create(Config)

	if err != nil {
		return err
	}

	defer destFile.Close()

	if _, err := io.Copy(destFile, templateFile); err != nil {
		return err
	}

	//write the config
	for _, shard := range dnsConfig.Shards{
		glog.Infof("Creating config for shard %s", shard.Name)

		buffer := bytes.NewBuffer([]byte(shard.Pattern))

		for _, router := range shard.RouterList {
			routerLine := fmt.Sprintf(" IN A %s \n", router.IP)
			buffer.WriteString(routerLine)
		}

		glog.Infof("Writing config line %s", buffer.String())
		destFile.Write(buffer.Bytes())
	}

	return nil
}

