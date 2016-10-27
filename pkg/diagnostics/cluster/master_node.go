package cluster

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	osclient "github.com/openshift/origin/pkg/client"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/diagnostics/types"

	sdnapi "github.com/openshift/origin/pkg/sdn/api"
)

const masterNotRunningAsANode = `Unable to find a node matching the cluster server IP.
This may indicate the master is not also running a node, and is unable
to proxy to pods over the Open vSwitch SDN.
`

// MasterNode is a Diagnostic for checking that the OpenShift master is also running as node.
// This is currently required to have the master on the Open vSwitch SDN and able to communicate
// with other nodes.
type MasterNode struct {
	KubeClient       *kclient.Client
	OsClient         *osclient.Client
	ServerUrl        string
	MasterConfigFile string // may often be empty if not being run on the host
}

const MasterNodeName = "MasterNode"

func (d *MasterNode) Name() string {
	return MasterNodeName
}

func (d *MasterNode) Description() string {
	return "Check if master is also running node (for Open vSwitch)"
}

func (d *MasterNode) CanRun() (bool, error) {
	if d.KubeClient == nil || d.OsClient == nil {
		return false, errors.New("must have kube and os client")
	}
	if d.ServerUrl == "" {
		return false, errors.New("must have a server URL")
	}

	// If there is a master config file available, we'll perform an additional
	// check to see if an OVS network plugin is in use. If no master config,
	// we assume this is the case for now and let the check run anyhow.
	if len(d.MasterConfigFile) > 0 {
		// Parse the master config and check the network plugin name:
		masterCfg, masterErr := configapilatest.ReadAndResolveMasterConfig(d.MasterConfigFile)
		if masterErr != nil {
			return false, types.DiagnosticError{ID: "DClu3008",
				LogMessage: fmt.Sprintf("Master config provided but unable to parse: %s", masterErr), Cause: masterErr}
		}
		if !sdnapi.IsOpenShiftNetworkPlugin(masterCfg.NetworkConfig.NetworkPluginName) {
			return false, errors.New(fmt.Sprintf("Network plugin does not require master to also run node: %s", masterCfg.NetworkConfig.NetworkPluginName))
		}
	}

	can, err := userCan(d.OsClient, authorizationapi.Action{
		Verb:     "list",
		Group:    kapi.GroupName,
		Resource: "nodes",
	})
	if err != nil {
		return false, types.DiagnosticError{ID: "DClu3000", LogMessage: fmt.Sprintf(clientErrorGettingNodes, err), Cause: err}
	} else if !can {
		return false, types.DiagnosticError{ID: "DClu3001", LogMessage: "Client does not have access to see node status", Cause: err}
	}
	return true, nil
}

func (d *MasterNode) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult(MasterNodeName)

	nodes, err := d.KubeClient.Nodes().List(kapi.ListOptions{})
	if err != nil {
		r.Error("DClu3002", err, fmt.Sprintf(clientErrorGettingNodes, err))
		return r
	}

	// Provide the actual net.LookupHost as the DNS resolver:
	serverIps, err := resolveServerIP(d.ServerUrl, net.LookupHost)
	if err != nil {
		r.Error("DClu3007", err, "Error resolving servers IP")
		return r
	}

	return searchNodesForIP(nodes.Items, serverIps)
}

// Define a resolve callback function type, use to swap in a dummy implementation
// in tests and avoid actual DNS calls.
type dnsResolver func(string) ([]string, error)

// resolveServerIP extracts the hostname portion of the API server URL passed in,
// and attempts dns resolution. It also attempts to catch server URL's that already
// contain both IPv4 and IPv6 addresses.
func resolveServerIP(serverUrl string, fn dnsResolver) ([]string, error) {
	// Extract the hostname from the API server URL:
	u, err := url.Parse(serverUrl)
	if err != nil || u.Host == "" {
		return nil, errors.New(fmt.Sprintf("Unable to parse hostname from URL: %s", serverUrl))
	}

	// Trim the port, if one exists, and watchout for IPv6 URLs.
	if strings.Count(u.Host, ":") > 1 {
		// Check if this is an IPv6 address as is to avoid problems with splitting
		// off the port:
		ipv6 := net.ParseIP(u.Host)
		if ipv6 != nil {
			return []string{ipv6.String()}, nil
		}
	}
	hostname, _, err := net.SplitHostPort(u.Host)
	if err != nil && hostname == "" {
		// Likely didn't have a port, carry on:
		hostname = u.Host
	}

	// Check if the hostname already looks like an IPv4 or IPv6 address:
	goIp := net.ParseIP(hostname)
	if goIp != nil {
		return []string{goIp.String()}, nil
	}

	// If not, attempt a DNS lookup. We may get multiple addresses for the hostname,
	// we'll return them all and search for any match in Kube nodes:
	ips, err := fn(hostname)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to perform DNS lookup for: %s", hostname))
	}
	return ips, nil
}

func searchNodesForIP(nodes []kapi.Node, ips []string) types.DiagnosticResult {
	r := types.NewDiagnosticResult(MasterNodeName)
	r.Debug("DClu3005", fmt.Sprintf("Seaching for a node with master IP: %s", ips))

	// Loops = # of nodes * number of IPs per node (2 commonly) * # of IPs the
	// server hostname resolves to. (should usually be 1)
	for _, node := range nodes {
		for _, address := range node.Status.Addresses {
			for _, ipAddress := range ips {
				r.Debug("DClu3006", fmt.Sprintf("Checking node %s address %s",
					node.ObjectMeta.Name, address.Address))
				if address.Address == ipAddress {
					r.Info("DClu3003", fmt.Sprintf("Found a node with same IP as master: %s",
						node.ObjectMeta.Name))
					return r
				}
			}
		}
	}
	r.Warn("DClu3004", nil, masterNotRunningAsANode)
	return r
}
