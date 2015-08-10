package kube

import (
	"encoding/hex"
	"fmt"
	log "github.com/golang/glog"
	"net"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/openshift/openshift-sdn/pkg/netutils"
	netutils_server "github.com/openshift/openshift-sdn/pkg/netutils/server"
)

type FlowController struct {
}

func NewFlowController() *FlowController {
	return &FlowController{}
}

func (c *FlowController) Setup(localSubnet, containerNetwork string) error {
	_, ipnet, err := net.ParseCIDR(localSubnet)
	subnetMaskLength, _ := ipnet.Mask.Size()
	gateway := netutils.GenerateDefaultGateway(ipnet).String()
	out, err := exec.Command("openshift-sdn-kube-subnet-setup.sh", gateway, ipnet.String(), containerNetwork, strconv.Itoa(subnetMaskLength), gateway).CombinedOutput()
	log.Infof("Output of setup script:\n%s", out)
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if ok {
			status := exitErr.ProcessState.Sys().(syscall.WaitStatus)
			if status.Exited() && status.ExitStatus() == 140 {
				// valid, do nothing, its just a benevolent restart
				return nil
			}
		}
		log.Errorf("Error executing setup script. \n\tOutput: %s\n\tError: %v\n", out, err)
		return err
	}
	//go c.manageLocalIpam(ipnet)
	_, err = exec.Command("ovs-ofctl", "-O", "OpenFlow13", "del-flows", "br0").CombinedOutput()
	if err != nil {
		return err
	}
	_, err = exec.Command("ovs-ofctl", "-O", "OpenFlow13", "add-flow", "br0", "cookie=0x0,table=0,priority=50,actions=output:2").CombinedOutput()
	arprule := fmt.Sprintf("cookie=0x0,table=0,priority=100,arp,nw_dst=%s,actions=output:2", gateway)
	iprule := fmt.Sprintf("cookie=0x0,table=0,priority=100,ip,nw_dst=%s,actions=output:2", gateway)
	_, err = exec.Command("ovs-ofctl", "-O", "OpenFlow13", "add-flow", "br0", arprule).CombinedOutput()
	_, err = exec.Command("ovs-ofctl", "-O", "OpenFlow13", "add-flow", "br0", iprule).CombinedOutput()
	return err
}

func (c *FlowController) manageLocalIpam(ipnet *net.IPNet) error {
	ipamHost := "127.0.0.1"
	ipamPort := uint(9080)
	inuse := make([]string, 0)
	ipam, _ := netutils.NewIPAllocator(ipnet.String(), inuse)
	f, err := os.Create("/etc/openshift-sdn/config.env")
	if err != nil {
		return err
	}
	_, err = f.WriteString(fmt.Sprintf("OPENSHIFT_SDN_TAP1_ADDR=%s\nOPENSHIFT_SDN_IPAM_SERVER=http://%s:%s", netutils.GenerateDefaultGateway(ipnet), ipamHost, ipamPort))
	if err != nil {
		return err
	}
	f.Close()
	// listen and serve does not return the control
	netutils_server.ListenAndServeNetutilServer(ipam, net.ParseIP(ipamHost), ipamPort, nil)
	return nil
}

func (c *FlowController) AddOFRules(nodeIP, subnet, localIP string) error {
	cookie := generateCookie(nodeIP)
	if nodeIP == localIP {
		// self, so add the input rules for containers that are not processed through kube-hooks
		// for the input rules to pods, see the kube-hook
		iprule := fmt.Sprintf("table=0,cookie=0x%s,priority=75,ip,nw_dst=%s,actions=output:9", cookie, subnet)
		arprule := fmt.Sprintf("table=0,cookie=0x%s,priority=75,arp,nw_dst=%s,actions=output:9", cookie, subnet)
		o, e := exec.Command("ovs-ofctl", "-O", "OpenFlow13", "add-flow", "br0", iprule).CombinedOutput()
		log.Infof("Output of adding %s: %s (%v)", iprule, o, e)
		o, e = exec.Command("ovs-ofctl", "-O", "OpenFlow13", "add-flow", "br0", arprule).CombinedOutput()
		log.Infof("Output of adding %s: %s (%v)", arprule, o, e)
		return e
	} else {
		iprule := fmt.Sprintf("table=0,cookie=0x%s,priority=100,ip,nw_dst=%s,actions=set_field:%s->tun_dst,output:1", cookie, subnet, nodeIP)
		arprule := fmt.Sprintf("table=0,cookie=0x%s,priority=100,arp,nw_dst=%s,actions=set_field:%s->tun_dst,output:1", cookie, subnet, nodeIP)
		o, e := exec.Command("ovs-ofctl", "-O", "OpenFlow13", "add-flow", "br0", iprule).CombinedOutput()
		log.Infof("Output of adding %s: %s (%v)", iprule, o, e)
		o, e = exec.Command("ovs-ofctl", "-O", "OpenFlow13", "add-flow", "br0", arprule).CombinedOutput()
		log.Infof("Output of adding %s: %s (%v)", arprule, o, e)
		return e
	}
	return nil
}

func (c *FlowController) DelOFRules(node, localIP string) error {
	log.Infof("Calling del rules for %s", node)
	cookie := generateCookie(node)
	if node == localIP {
		iprule := fmt.Sprintf("table=0,cookie=0x%s/0xffffffff,ip,in_port=10", cookie)
		arprule := fmt.Sprintf("table=0,cookie=0x%s/0xffffffff,arp,in_port=10", cookie)
		o, e := exec.Command("ovs-ofctl", "-O", "OpenFlow13", "del-flows", "br0", iprule).CombinedOutput()
		log.Infof("Output of deleting local ip rules %s (%v)", o, e)
		o, e = exec.Command("ovs-ofctl", "-O", "OpenFlow13", "del-flows", "br0", arprule).CombinedOutput()
		log.Infof("Output of deleting local arp rules %s (%v)", o, e)
		return e
	} else {
		iprule := fmt.Sprintf("table=0,cookie=0x%s/0xffffffff,ip", cookie)
		arprule := fmt.Sprintf("table=0,cookie=0x%s/0xffffffff,arp", cookie)
		o, e := exec.Command("ovs-ofctl", "-O", "OpenFlow13", "del-flows", "br0", iprule).CombinedOutput()
		log.Infof("Output of deleting %s: %s (%v)", iprule, o, e)
		o, e = exec.Command("ovs-ofctl", "-O", "OpenFlow13", "del-flows", "br0", arprule).CombinedOutput()
		log.Infof("Output of deleting %s: %s (%v)", arprule, o, e)
		return e
	}
	return nil
}

func generateCookie(ip string) string {
	return hex.EncodeToString(net.ParseIP(ip).To4())
}
