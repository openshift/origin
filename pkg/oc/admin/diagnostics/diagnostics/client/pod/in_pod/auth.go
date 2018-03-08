package in_pod

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/miekg/dns"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knet "k8s.io/apimachinery/pkg/util/net"
	restclient "k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/cmd/flagtypes"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/types"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset"
)

const (
	PodCheckAuthName = "PodCheckAuth"
)

// PodCheckAuth is a Diagnostic to check that a pod can authenticate as expected
type PodCheckAuth struct {
	MasterUrl    string
	MasterCaPath string
	TokenPath    string
}

// Name is part of the Diagnostic interface and just returns name.
func (d PodCheckAuth) Name() string {
	return PodCheckAuthName
}

// Description is part of the Diagnostic interface and provides a user-focused description of what the diagnostic does.
func (d PodCheckAuth) Description() string {
	return "Check that service account credentials authenticate as expected"
}

func (d PodCheckAuth) Requirements() (client bool, host bool) {
	return true, false
}

// CanRun is part of the Diagnostic interface; it determines if the conditions are right to run this diagnostic.
func (d PodCheckAuth) CanRun() (bool, error) {
	return true, nil
}

// Check is part of the Diagnostic interface; it runs the actual diagnostic logic
func (d PodCheckAuth) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult(PodCheckAuthName)
	token, err := ioutil.ReadFile(d.TokenPath)
	if err != nil {
		r.Error("DP1001", err, fmt.Sprintf("could not read the service account token: %v", err))
		return r
	}
	d.authenticateToMaster(string(token), r)
	d.authenticateToRegistry(string(token), r)
	return r
}

// authenticateToMaster tests whether we can use the serviceaccount token
// to reach the master and authenticate
func (d PodCheckAuth) authenticateToMaster(token string, r types.DiagnosticResult) {
	clientConfig := &clientcmd.Config{
		MasterAddr:     flagtypes.Addr{Value: d.MasterUrl}.Default(),
		KubernetesAddr: flagtypes.Addr{Value: d.MasterUrl}.Default(),
		CommonConfig: restclient.Config{
			TLSClientConfig: restclient.TLSClientConfig{CAFile: d.MasterCaPath},
			BearerToken:     token,
		},
	}
	userClient, err := userclient.NewForConfig(clientConfig.OpenShiftConfig())
	if err != nil {
		r.Error("DP1002", err, fmt.Sprintf("could not create API clients from the service account client config: %v", err))
		return
	}
	rchan := make(chan error, 1) // for concurrency with timeout
	go func() {
		_, err := userClient.User().Users().Get("~", metav1.GetOptions{})
		rchan <- err
	}()

	select {
	case <-time.After(time.Second * 4): // timeout per query
		r.Warn("DP1005", nil, "A request to the master timed out.\nThis could be temporary but could also indicate network or DNS problems.")
	case err := <-rchan:
		if err != nil {
			r.Error("DP1003", err, fmt.Sprintf("Could not authenticate to the master with the service account credentials: %v", err))
		} else {
			r.Info("DP1004", "Service account token successfully authenticated to master")
		}
	}
	return
}

const registryHostname = "docker-registry.default.svc.cluster.local" // standard registry service DNS
const registryPort = "5000"

func (d PodCheckAuth) authenticateToRegistry(token string, r types.DiagnosticResult) {
	resolvConf, err := getResolvConf(r)
	if err != nil {
		return // any errors have been reported via "r", env is very borked, test cannot proceed
	}
	msg := new(dns.Msg)
	msg.SetQuestion(registryHostname+".", dns.TypeA)
	msg.RecursionDesired = false
	result, completed := dnsQueryWithTimeout(msg, resolvConf.Servers[0], 2)
	switch {
	case !completed:
		r.Error("DP1006", nil, fmt.Sprintf("DNS resolution for registry address %s timed out; this could indicate problems with DNS resolution or networking", registryHostname))
		return
	case result.err != nil:
		r.Error("DP1016", nil, fmt.Sprintf("DNS resolution for registry address %s returned an error; container DNS is likely incorrect. The error was: %v", registryHostname, result.err))
		return
	case result.in == nil, len(result.in.Answer) == 0:
		r.Warn("DP1007", nil, fmt.Sprintf("DNS resolution for registry address %s returned no results; either the integrated registry is not deployed, or container DNS configuration is incorrect.", registryHostname))
		return
	}

	// first try the secure connection in case they followed directions to secure the registry
	// (https://docs.openshift.org/latest/install_config/install/docker_registry.html#securing-the-registry)
	cacert, err := ioutil.ReadFile(d.MasterCaPath) // TODO: we assume same CA as master - better choice?
	if err != nil {
		r.Error("DP1008", err, fmt.Sprintf("Failed to read CA cert file %s:\n%v", d.MasterCaPath, err))
		return
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(cacert) {
		r.Error("DP1009", err, fmt.Sprintf("Could not use cert from CA cert file %s:\n%v", d.MasterCaPath, err))
		return
	}
	noSecClient := http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return fmt.Errorf("no redirect expected")
		},
		Timeout: time.Second * 2,
	}
	secClient := noSecClient
	secClient.Transport = knet.SetTransportDefaults(&http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}})
	secError := processRegistryRequest(&secClient, fmt.Sprintf("https://%s:%s/v2/", registryHostname, registryPort), token, r)
	if secError == nil {
		return // made the request successfully enough to diagnose
	}
	switch {
	case strings.Contains(secError.Error(), "tls: oversized record received"),
		strings.Contains(secError.Error(), "server gave HTTP response to HTTPS"):
		r.Debug("DP1015", "docker-registry not secured; falling back to cleartext connection")
		if nosecError := processRegistryRequest(&noSecClient, fmt.Sprintf("http://%s:%s/v2/", registryHostname, registryPort), token, r); nosecError != nil {
			r.Error("DP1013", nosecError, fmt.Sprintf("Unexpected error authenticating to the integrated registry:\n(%T) %[1]v", nosecError))
		}
	default:
		r.Error("DP1013", secError, fmt.Sprintf("Unexpected error authenticating to the integrated registry:\n(%T) %[1]v", secError))
	}
}

// makes a request, handles some http/connection errors, returns any others
func processRegistryRequest(client *http.Client, url string, token string, r types.DiagnosticResult) error {
	req, _ := http.NewRequest("HEAD", url, nil)
	req.SetBasicAuth("anyname", token)
	response, err := client.Do(req)
	if err == nil {
		switch response.StatusCode {
		case 401, 403:
			r.Error("DP1010", nil, "Service account token was not accepted by the integrated registry for authentication.\nThis indicates possible network problems or misconfiguration of the registry.")
		case 200:
			r.Info("DP1011", "Service account token was authenticated by the integrated registry.")
		default:
			r.Error("DP1012", nil, fmt.Sprintf("Unexpected status code from integrated registry authentication:\n%s", response.Status))
		}
		return nil
	} else if strings.Contains(err.Error(), "net/http: request canceled") {
		// (*url.Error) Head https://docker-registry.default.svc.cluster.local:5000/v2/: net/http: request canceled while waiting for connection
		r.Error("DP1014", err, "Request to integrated registry timed out; this typically indicates network or SDN problems.")
		return nil
	}
	return err

	// fall back to non-secured access
}
