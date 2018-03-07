// +build linux

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/network/node/cniserver"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/020"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ipam"
	"github.com/containernetworking/plugins/pkg/ns"

	"github.com/vishvananda/netlink"
)

type cniPlugin struct {
	socketPath string
	hostNS     ns.NetNS
}

func NewCNIPlugin(socketPath string, hostNS ns.NetNS) *cniPlugin {
	return &cniPlugin{socketPath: socketPath, hostNS: hostNS}
}

// Create and fill a CNIRequest with this plugin's environment and stdin which
// contain the CNI variables and configuration
func newCNIRequest(args *skel.CmdArgs) *cniserver.CNIRequest {
	envMap := make(map[string]string)
	for _, item := range os.Environ() {
		idx := strings.Index(item, "=")
		if idx > 0 {
			envMap[strings.TrimSpace(item[:idx])] = item[idx+1:]
		}
	}

	return &cniserver.CNIRequest{
		Env:    envMap,
		Config: args.StdinData,
	}
}

// Send a CNI request to the CNI server via JSON + HTTP over a root-owned unix socket,
// and return the result
func (p *cniPlugin) doCNI(url string, req *cniserver.CNIRequest) ([]byte, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal CNI request %v: %v", req, err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			Dial: func(proto, addr string) (net.Conn, error) {
				return net.Dial("unix", p.socketPath)
			},
		},
	}

	var resp *http.Response
	err = p.hostNS.Do(func(ns.NetNS) error {
		resp, err = client.Post(url, "application/json", bytes.NewReader(data))
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send CNI request: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read CNI result: %v", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("CNI request failed with status %v: '%s'", resp.StatusCode, string(body))
	}

	return body, nil
}

// Send the ADD command environment and config to the CNI server, returning
// the IPAM result to the caller
func (p *cniPlugin) doCNIServerAdd(req *cniserver.CNIRequest, hostVeth string) (types.Result, error) {
	req.HostVeth = hostVeth
	body, err := p.doCNI("http://dummy/", req)
	if err != nil {
		return nil, err
	}

	// We currently expect CNI version 0.2.0 results, because that's the
	// CNIVersion we pass in our config JSON
	result, err := types020.NewResult(body)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response '%s': %v", string(body), err)
	}

	return result, nil
}

func (p *cniPlugin) testCmdAdd(args *skel.CmdArgs) (types.Result, error) {
	return p.doCNIServerAdd(newCNIRequest(args), "dummy0")
}

func (p *cniPlugin) CmdAdd(args *skel.CmdArgs) error {
	req := newCNIRequest(args)
	config, err := cniserver.ReadConfig(cniserver.CNIServerConfigFilePath)
	if err != nil {
		return err
	}

	var hostVeth, contVeth net.Interface
	err = ns.WithNetNSPath(args.Netns, func(hostNS ns.NetNS) error {
		hostVeth, contVeth, err = ip.SetupVeth(args.IfName, int(config.MTU), hostNS)
		if err != nil {
			return fmt.Errorf("failed to create container veth: %v", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	result, err := p.doCNIServerAdd(req, hostVeth.Name)
	if err != nil {
		return err
	}

	// current.NewResultFromResult and ipam.ConfigureIface both think that
	// a route with no gateway specified means to pass the default gateway
	// as the next hop to ip.AddRoute, but that's not what we want; we want
	// to pass nil as the next hop. So we need to clear the default gateway.
	result020, err := types020.GetResult(result)
	if err != nil {
		return fmt.Errorf("failed to convert IPAM result: %v", err)
	}
	result020.IP4.Gateway = nil

	result030, err := current.NewResultFromResult(result020)
	if err != nil || len(result030.IPs) != 1 || result030.IPs[0].Version != "4" {
		return fmt.Errorf("failed to convert IPAM result: %v", err)
	}

	// Add a sandbox interface record which ConfigureInterface expects.
	// The only interface we report is the pod interface.
	result030.Interfaces = []*current.Interface{
		{
			Name:    args.IfName,
			Mac:     contVeth.HardwareAddr.String(),
			Sandbox: args.Netns,
		},
	}
	result030.IPs[0].Interface = current.Int(0)

	err = ns.WithNetNSPath(args.Netns, func(ns.NetNS) error {
		// Set up eth0
		if err := ip.SetHWAddrByIP(args.IfName, result030.IPs[0].Address.IP, nil); err != nil {
			return fmt.Errorf("failed to set pod interface MAC address: %v", err)
		}
		if err := ipam.ConfigureIface(args.IfName, result030); err != nil {
			return fmt.Errorf("failed to configure container IPAM: %v", err)
		}

		// Set up lo
		link, err := netlink.LinkByName("lo")
		if err == nil {
			err = netlink.LinkSetUp(link)
		}
		if err != nil {
			return fmt.Errorf("failed to configure container loopback: %v", err)
		}

		// Set up macvlan0 (if it exists)
		link, err = netlink.LinkByName("macvlan0")
		if err == nil {
			err = netlink.LinkSetUp(link)
			if err != nil {
				return fmt.Errorf("failed to configure macvlan device: %v", err)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	return result.Print()
}

func (p *cniPlugin) CmdDel(args *skel.CmdArgs) error {
	_, err := p.doCNI("http://dummy/", newCNIRequest(args))
	return err
}

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	hostNS, err := ns.GetCurrentNS()
	if err != nil {
		panic(fmt.Sprintf("could not get current kernel netns: %v", err))
	}
	defer hostNS.Close()
	p := NewCNIPlugin(cniserver.CNIServerSocketPath, hostNS)
	skel.PluginMain(p.CmdAdd, p.CmdDel, version.Legacy)
}
