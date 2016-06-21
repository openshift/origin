package cluster

import (
	"errors"
	"fmt"
	"net"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	hostdiag "github.com/openshift/origin/pkg/diagnostics/host"
	"github.com/openshift/origin/pkg/diagnostics/types"
	"github.com/openshift/origin/pkg/service/admission"
)

// ServiceExternalIPs is a Diagnostic to check for the services in the cluster
// that do not comply with an updated master ExternalIPNetworkCIDR setting.
// Background: https://github.com/openshift/origin/issues/7808
type ServiceExternalIPs struct {
	MasterConfigFile string
	KclusterClient   *kclient.Client
}

const ServiceExternalIPsName = "ServiceExternalIPs"

func (d *ServiceExternalIPs) Name() string {
	return ServiceExternalIPsName
}

func (d *ServiceExternalIPs) Description() string {
	return "Check for existing services with ExternalIPs that are disallowed by master config"
}

func (d *ServiceExternalIPs) CanRun() (bool, error) {
	if len(d.MasterConfigFile) == 0 {
		return false, errors.New("No master config file was detected")
	}
	if d.KclusterClient == nil {
		return false, errors.New("Client config must include a cluster-admin context to run this diagnostic")
	}

	return true, nil
}

func (d *ServiceExternalIPs) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult(ServiceExternalIPsName)
	masterConfig, err := hostdiag.GetMasterConfig(r, d.MasterConfigFile)
	if err != nil {
		r.Info("DH2004", "Unreadable master config; skipping this diagnostic.")
		return r
	}

	admit, reject := []*net.IPNet{}, []*net.IPNet{}
	if cidrs := masterConfig.NetworkConfig.ExternalIPNetworkCIDRs; cidrs != nil {
		reject, admit, err = admission.ParseRejectAdmitCIDRRules(cidrs)
		if err != nil {
			r.Error("DH2007", err, fmt.Sprintf("Could not parse master config NetworkConfig.ExternalIPNetworkCIDRs: (%[1]T) %[1]v", err))
			return r
		}
	}
	services, err := d.KclusterClient.Services("").List(kapi.ListOptions{})
	if err != nil {
		r.Error("DH2005", err, fmt.Sprintf("Error while listing cluster services: (%[1]T) %[1]v", err))
		return r
	}

	errList := []string{}
	for _, service := range services.Items {
		if len(service.Spec.ExternalIPs) == 0 {
			continue
		}
		if len(admit) == 0 {
			errList = append(errList, fmt.Sprintf("Service %s.%s specifies ExternalIPs %v, but none are permitted.", service.Namespace, service.Name, service.Spec.ExternalIPs))
			continue
		}
		for _, ipString := range service.Spec.ExternalIPs {
			ip := net.ParseIP(ipString)
			if ip == nil {
				continue // we don't really care for the purposes of this diagnostic
			}
			if admission.NetworkSlice(reject).Contains(ip) || !admission.NetworkSlice(admit).Contains(ip) {
				errList = append(errList, fmt.Sprintf("Service %s.%s specifies ExternalIP %s that is not permitted by the master ExternalIPNetworkCIDRs setting.", service.Namespace, service.Name, ipString))
			}
		}
	}
	if len(errList) > 0 {
		r.Error("DH2006", nil, `The following problems were found with service ExternalIPs in the cluster.
These services were created before the master ExternalIPNetworkCIDRs setting changed to exclude them.
The default ExternalIPNetworkCIDRs now excludes all ExternalIPs on services.
`+strings.Join(errList, "\n"))
	}

	return r
}
