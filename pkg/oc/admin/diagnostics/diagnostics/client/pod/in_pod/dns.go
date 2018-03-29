package in_pod

import (
	"fmt"
	"math/rand"
	"net"

	"github.com/miekg/dns"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	PodCheckDnsName = "PodCheckDns"
	txtDP2016       = `
Query to nameserver %s for random subdomain of %s
returned an answer, indicating that this domain provides wildcard DNS.

This domain should not be in the search.

It can cause queries for external domains (e.g. github.com)
to resolve to the wildcard address. Full response:

%v`
)

// PodCheckDns is a Diagnostic to check that DNS within a pod works as expected
type PodCheckDns struct {
}

// Name is part of the Diagnostic interface and just returns name.
func (d PodCheckDns) Name() string {
	return PodCheckDnsName
}

// Description is part of the Diagnostic interface and just returns the diagnostic description.
func (d PodCheckDns) Description() string {
	return "Check that DNS within a pod works as expected"
}

func (d PodCheckDns) Requirements() (client bool, host bool) {
	return true, false
}

// CanRun is part of the Diagnostic interface; it determines if the conditions are right to run this diagnostic.
func (d PodCheckDns) CanRun() (bool, error) {
	return true, nil
}

// Check is part of the Diagnostic interface; it runs the actual diagnostic logic
func (d PodCheckDns) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult(PodCheckDnsName)

	if resolvConf, err := getResolvConf(r); err == nil {
		dnsServers := sanitizeDNSServers(resolvConf.Servers)
		connectAndResolve(resolvConf, dnsServers, r)
		resolveSearch(resolvConf, dnsServers, r)
	}
	return r
}

// sanitizeDNSServers does these things:
// - Reorder dns servers: ipv4 servers are ahead of ipv6 servers as we only support ipv4 addrs at this time
// - Put all dns servers in square brackets: This will disambiguate addr and port when there are many colons in the addr
func sanitizeDNSServers(servers []string) []string {
	ipv4Servers := []string{}
	ipv6Servers := []string{}

	for _, server := range servers {
		if ip := net.ParseIP(server); ip != nil {
			if ip.To4() != nil {
				ipv4Servers = append(ipv4Servers, fmt.Sprintf("[%s]", server))
			} else {
				ipv6Servers = append(ipv6Servers, fmt.Sprintf("[%s]", server))
			}
		}
	}

	return append(ipv4Servers, ipv6Servers...)
}

func connectAndResolve(resolvConf *dns.ClientConfig, dnsServers []string, r types.DiagnosticResult) {
	for serverIndex, server := range dnsServers {
		// put together a DNS query to configured nameservers for kubernetes.default
		msg := new(dns.Msg)
		msg.SetQuestion("kubernetes.default.svc.cluster.local.", dns.TypeA)
		msg.RecursionDesired = false
		if result, completed := dnsQueryWithTimeout(msg, server, 2); !completed {
			if serverIndex == 0 { // in a pod, master (SkyDNS) IP is injected as first nameserver
				r.Warn("DP2009", nil, fmt.Sprintf("A request to the master (SkyDNS) nameserver %s timed out.\nThis could be temporary but could also indicate network or DNS problems.\nThis nameserver is critical for resolving cluster DNS names.", server))
			} else {
				r.Warn("DP2010", nil, fmt.Sprintf("A request to the nameserver %s timed out.\nThis could be temporary but could also indicate network or DNS problems.", server))
			}
		} else {
			in, err := result.in, result.err
			if serverIndex == 0 { // in a pod, master (SkyDNS) IP is injected as first nameserver
				if err != nil {
					r.Error("DP2003", err, fmt.Sprintf("The first /etc/resolv.conf nameserver %s\ncould not resolve kubernetes.default.svc.cluster.local.\nError: %v\nThis nameserver points to the master's SkyDNS which is critical for\nresolving cluster names, e.g. for Services.", server, err))
				} else if len(in.Answer) == 0 {
					r.Error("DP2006", err, fmt.Sprintf("The first /etc/resolv.conf nameserver %s\ncould not resolve kubernetes.default.svc.cluster.local.\nReturn code: %v\nThis nameserver points to the master's SkyDNS which is critical for\nresolving cluster names, e.g. for Services.", server, dns.RcodeToString[in.MsgHdr.Rcode]))
				} else {
					r.Debug("DP2007", fmt.Sprintf("The first /etc/resolv.conf nameserver %s\nresolved kubernetes.default.svc.cluster.local. to:\n  %s", server, in.Answer[0]))
				}
			} else if err != nil {
				r.Warn("DP2004", err, fmt.Sprintf("Error querying nameserver %s:\n  %v\nThis may indicate a problem with non-cluster DNS.", server, err))
			} else {
				rcode := in.MsgHdr.Rcode
				switch rcode {
				case dns.RcodeSuccess, dns.RcodeNameError: // aka NXDOMAIN
					r.Debug("DP2005", fmt.Sprintf("Successful query to nameserver %s", server))
				default:
					r.Warn("DP2008", nil, fmt.Sprintf("Received unexpected return code '%s' from nameserver %s:\nThis may indicate a problem with non-cluster DNS.", dns.RcodeToString[rcode], server))
				}
			}
		}
	}
}

const letterBytes = "abcdefghijklmnopqrstuvwxyz0123456789"

func resolveSearch(resolvConf *dns.ClientConfig, dnsServers []string, r types.DiagnosticResult) {
	foundDomain := false
	randomString := func() string {
		b := make([]byte, 20)
		for i := range b {
			b[i] = letterBytes[rand.Intn(len(letterBytes))]
		}
		return string(b)
	}()
	seenDP2014 := sets.String{}
	seenDP2015 := sets.String{}
	for _, domain := range resolvConf.Search {
		if domain == "svc.cluster.local" {
			foundDomain = true // this will make kubernetes.default work
		}
		// put together a DNS query to configured nameservers for each search domain
		msg := new(dns.Msg)
		msg.SetQuestion("wildcard."+randomString+"."+domain+".", dns.TypeA)
		msg.RecursionDesired = true // otherwise we just get the authority section for the TLD
		for _, server := range dnsServers {
			result, completed := dnsQueryWithTimeout(msg, server, 2)
			switch {
			case !completed:
				if !seenDP2014.Has(server) {
					r.Warn("DP2014", nil, fmt.Sprintf("A request to the nameserver %s timed out.\nThis could be temporary but could also indicate network or DNS problems.", server))
					seenDP2014.Insert(server) // no need to keep warning about the same server for every domain
				}
			case result.err != nil:
				if !seenDP2015.Has(server) {
					r.Warn("DP2015", result.err, fmt.Sprintf("Error querying nameserver %s:\n  %v\nThis may indicate a problem with DNS.", server, result.err))
					seenDP2015.Insert(server) // don't repeat the error for the same nameserver; chances are it's the same error
				}
			case result.in.Answer == nil, len(result.in.Answer) == 0:
				r.Debug("DP2017", fmt.Sprintf("Nameserver %s responded to wildcard with no answer, which is expected.\n%v", server, result.in))
			default: // the random domain is not supposed to resolve
				r.Error("DP2016", nil, fmt.Sprintf(txtDP2016, server, domain, result.in))
			}
		}
	}
	if !foundDomain {
		r.Error("DP2019", nil, "Did not find svc.cluster.local among the configured search domains in /etc/resolv.conf.\nThis is likely to cause problems with certain components that expect to use partial cluster addresses.")
	}
}
