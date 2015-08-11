package multitenant

import (
	"encoding/hex"
	"fmt"
	log "github.com/golang/glog"
	"net"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/openshift/openshift-sdn/pkg/netutils"
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
	out, err := exec.Command("openshift-sdn-multitenant-setup.sh", gateway, ipnet.String(), containerNetwork, strconv.Itoa(subnetMaskLength), gateway).CombinedOutput()
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
	}
	return err
}

func (c *FlowController) AddOFRules(nodeIP, subnet, localIP string) error {
	if nodeIP == localIP {
		return nil
	}

	cookie := generateCookie(nodeIP)
	iprule := fmt.Sprintf("table=6,cookie=0x%s,priority=100,ip,nw_dst=%s,actions=move:NXM_NX_REG0[]->NXM_NX_TUN_ID[0..31],set_field:%s->tun_dst,output:1", cookie, subnet, nodeIP)
	arprule := fmt.Sprintf("table=7,cookie=0x%s,priority=100,arp,nw_dst=%s,actions=move:NXM_NX_REG0[]->NXM_NX_TUN_ID[0..31],set_field:%s->tun_dst,output:1", cookie, subnet, nodeIP)
	o, e := exec.Command("ovs-ofctl", "-O", "OpenFlow13", "add-flow", "br0", iprule).CombinedOutput()
	log.Infof("Output of adding %s: %s (%v)", iprule, o, e)
	o, e = exec.Command("ovs-ofctl", "-O", "OpenFlow13", "add-flow", "br0", arprule).CombinedOutput()
	log.Infof("Output of adding %s: %s (%v)", arprule, o, e)
	return e
}

func (c *FlowController) DelOFRules(node, localIP string) error {
	if node == localIP {
		return nil
	}

	log.Infof("Calling del rules for %s", node)
	cookie := generateCookie(node)
	iprule := fmt.Sprintf("table=6,cookie=0x%s/0xffffffff", cookie)
	arprule := fmt.Sprintf("table=7,cookie=0x%s/0xffffffff", cookie)
	o, e := exec.Command("ovs-ofctl", "-O", "OpenFlow13", "del-flows", "br0", iprule).CombinedOutput()
	log.Infof("Output of deleting local ip rules %s (%v)", o, e)
	o, e = exec.Command("ovs-ofctl", "-O", "OpenFlow13", "del-flows", "br0", arprule).CombinedOutput()
	log.Infof("Output of deleting local arp rules %s (%v)", o, e)
	return e
}

func generateCookie(ip string) string {
	return hex.EncodeToString(net.ParseIP(ip).To4())
}
