package client

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	kclientcmd "k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	kclientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

// ConfigContext diagnostics (one per context) validate that the client config context is complete and has connectivity to the master.
type ConfigContext struct {
	RawConfig   *kclientcmdapi.Config
	ContextName string
}

const (
	ConfigContextsName = "ConfigContexts"

	contextDesc = `
For client config context '%s':
The server URL is '%s'
The user authentication is '%s'
The current project is '%s'
`
	currContextDesc = `
The current client config context is '%s':
The server URL is '%s'
The user authentication is '%s'
The current project is '%s'
`
	clientNoResolve = `
This usually means that the hostname does not resolve to an IP.
Hostnames should usually be resolved via DNS or an /etc/hosts file.
Ensure that the hostname resolves correctly from your host before proceeding.
Of course, your config could also simply have the wrong hostname specified.
`
	clientUnknownCa = `
This means that we cannot validate the certificate in use by the
master API server, so we cannot securely communicate with it.
Connections could be intercepted and your credentials stolen.

Since the server certificate we see when connecting is not validated
by public certificate authorities (CAs), you probably need to specify a
certificate from a private CA to validate the connection.

Your config may be specifying the wrong CA cert, or none, or there
could actually be a man-in-the-middle attempting to intercept your
connection.  If you are unconcerned about any of this, you can add the
--insecure-skip-tls-verify flag to bypass secure (TLS) verification,
but this is risky and should not be necessary.
** Connections could be intercepted and your credentials stolen. **
`
	clientUnneededCa = `
This means that for client connections to the master API server, you
(or your kubeconfig) specified both a validating certificate authority
and that the client should bypass connection security validation.

This is not allowed because it is likely to be a mistake.

If you want to use --insecure-skip-tls-verify to bypass security (which
is usually a bad idea anyway), then you need to also clear the CA cert
from your command line options or kubeconfig file(s). Of course, it
would be far better to obtain and use a correct CA cert.
`
	clientInvCertName = `
This means that the certificate in use by the master API server
(master) does not match the hostname by which you are addressing it:
  %s
so a secure connection is not allowed. In theory, this *could* mean that
someone is intercepting your connection and presenting a certificate
that is valid but for a different server, which is why secure validation
fails in this case.

However, the most likely explanation is that the server certificate
needs to be updated to include the name you are using to reach it.

If the master API server is generating its own certificates (which
is the default), then specifying the public master address in the
master-config.yaml or with the --public-master flag is usually the easiest
way to do this. If you need something more complicated (for instance,
multiple public addresses for the API, or your own CA), then you will need
to custom-generate the server certificate with the right names yourself.

If you are unconcerned about any of this, you can add the
--insecure-skip-tls-verify flag to bypass secure (TLS) verification,
but this is risky and should not be necessary.
** Connections could be intercepted and your credentials stolen. **
`
	clientConnRefused = `
This means that when we tried to connect to the master API server, we
reached the host, but nothing accepted the port connection. This could
mean that the master is stopped, or that a firewall or security policy
is blocking access at that port.

You will not be able to connect or do anything at all with this server
until this server problem is resolved or you specify a corrected
server address.`

	clientConnTimeout = `
This means that when we tried to connect to the master API server,
we could not reach the host at all.
* You may have specified the wrong host address.
* This could mean the host is completely unavailable (down).
* This could indicate a routing problem or a firewall that simply
  drops requests rather than responding by resetting the connection.
* It does not generally mean that DNS name resolution failed (which
  would be a different error) though the problem could be that it
  gave the wrong address.`
	clientMalformedHTTP = `
This means that when we tried to connect to the master API server with
a plain HTTP connection, the server did not speak HTTP back to us. The
most common explanation is that a secure server is listening but you
specified an http: connection instead of https:.  There could also
be another service listening at the intended port speaking some other
protocol entirely.

You will not be able to connect or do anything at all with the server
until this server problem is resolved or you specify a corrected
server address.`
	clientMalformedTLS = `
This means that when we tried to connect to the master API server
with a secure HTTPS connection, the server did not speak HTTPS back to
us. The most common explanation is that the server listening at that
port is not the secure server you expected - it may be a non-secure
HTTP server or the wrong service may be listening there, or you may have
specified an incorrect port.

You will not be able to connect or do anything at all with the server
until this server problem is resolved or you specify a corrected
server address.`
	clientUnauthn = `
This means that when we tried to make a request to the master API
server, your kubeconfig did not present valid credentials to
authenticate your client. Credentials generally consist of a client
key/certificate or an access token. Your kubeconfig may not have
presented any, or they may be invalid.`
	clientUnauthz = `
This means that when we tried to make a request to the master API
server, the request required credentials that were not presented. This
can happen with an expired or invalid authentication token. Try logging
in with this user again.`
)

var (
	invalidCertNameRx = regexp.MustCompile("x509: certificate is valid for (\\S+, )+not (\\S+)")
)

func (d ConfigContext) Name() string {
	return fmt.Sprintf("%s[%s]", ConfigContextsName, d.ContextName)
}

func (d ConfigContext) Description() string {
	return "Validate client config context is complete and has connectivity"
}

func (d ConfigContext) CanRun() (bool, error) {
	if d.RawConfig == nil {
		// TODO make prettier?
		return false, errors.New("There is no client config file")
	}

	if len(d.ContextName) == 0 {
		return false, errors.New("There is no current context")
	}

	return true, nil
}

func (d ConfigContext) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult(ConfigContextsName)

	isDefaultContext := d.RawConfig.CurrentContext == d.ContextName

	// prepare bad news message
	errorKey := "DCli0001"
	unusableLine := fmt.Sprintf("The client config context '%s' is unusable", d.ContextName)
	if isDefaultContext {
		errorKey = "DCli0002"
		unusableLine = fmt.Sprintf("The current client config context '%s' is unusable", d.ContextName)
	}

	// check that the context and its constitutuents are defined in the kubeconfig
	context, exists := d.RawConfig.Contexts[d.ContextName]
	if !exists {
		r.Error(errorKey, nil, fmt.Sprintf("%s:\n Client config context '%s' is not defined.", unusableLine, d.ContextName))
		return r
	}
	clusterName := context.Cluster
	cluster, exists := d.RawConfig.Clusters[clusterName]
	if !exists {
		r.Error(errorKey, nil, fmt.Sprintf("%s:\n Client config context '%s' has a cluster '%s' which is not defined.", unusableLine, d.ContextName, clusterName))
		return r
	}
	authName := context.AuthInfo
	if _, exists := d.RawConfig.AuthInfos[authName]; !exists {
		r.Error(errorKey, nil, fmt.Sprintf("%s:\n Client config context '%s' has a user '%s' which is not defined.", unusableLine, d.ContextName, authName))
		return r
	}

	// we found a fully-defined context
	project := context.Namespace
	if project == "" {
		project = kapi.NamespaceDefault // k8s fills this in anyway if missing from the context
	}
	msgText := contextDesc
	if isDefaultContext {
		msgText = currContextDesc
	}
	msgText = fmt.Sprintf(msgText, d.ContextName, cluster.Server, authName, project)

	// Actually send a request to see if context has connectivity.
	// Note: we cannot reuse factories as they cache the clients, so build new factory for each context.
	osClient, _, err := osclientcmd.NewFactory(kclientcmd.NewDefaultClientConfig(*d.RawConfig, &kclientcmd.ConfigOverrides{Context: *context})).Clients()
	// client create now *fails* if cannot connect to server; so, address connectivity errors below
	if err == nil {
		if projects, projerr := osClient.Projects().List(labels.Everything(), fields.Everything()); projerr != nil {
			err = projerr
		} else { // success!
			list := []string{}
			for i, project := range projects.Items {
				if i > 9 {
					list = append(list, "...")
					break
				}
				list = append(list, project.Name)
			}
			if len(list) == 0 {
				r.Info("DCli0003", msgText+"Successfully requested project list, but it is empty, so user has no access to anything.")
			} else {
				r.Info("DCli0004", msgText+fmt.Sprintf("Successfully requested project list; has access to project(s):\n  %v", list))
			}
			return r
		}
	}

	// something went wrong; couldn't create client or get project list.
	// interpret the terse error messages with helpful info.
	errMsg := err.Error()
	errFull := fmt.Sprintf("(%T) %[1]v\n", err)
	var reason, errId string
	switch {
	case regexp.MustCompile("dial tcp: lookup (\\S+): no such host").MatchString(errMsg):
		errId, reason = "DCli0005", clientNoResolve
	case strings.Contains(errMsg, "x509: certificate signed by unknown authority"):
		errId, reason = "DCli0006", clientUnknownCa
	case strings.Contains(errMsg, "specifying a root certificates file with the insecure flag is not allowed"):
		errId, reason = "DCli0007", clientUnneededCa
	case invalidCertNameRx.MatchString(errMsg):
		match := invalidCertNameRx.FindStringSubmatch(errMsg)
		serverHost := match[len(match)-1]
		errId, reason = "DCli0008", fmt.Sprintf(clientInvCertName, serverHost)
	case regexp.MustCompile("dial tcp (\\S+): connection refused").MatchString(errMsg):
		errId, reason = "DCli0009", clientConnRefused
	case regexp.MustCompile("dial tcp (\\S+): (?:connection timed out|i/o timeout|no route to host)").MatchString(errMsg):
		errId, reason = "DCli0010", clientConnTimeout
	case strings.Contains(errMsg, "malformed HTTP response"):
		errId, reason = "DCli0011", clientMalformedHTTP
	case strings.Contains(errMsg, "tls: oversized record received with length"):
		errId, reason = "DCli0012", clientMalformedTLS
	case strings.Contains(errMsg, `User "system:anonymous" cannot`):
		errId, reason = "DCli0013", clientUnauthn
	case strings.Contains(errMsg, "provide credentials"):
		errId, reason = "DCli0014", clientUnauthz
	default:
		errId, reason = "DCli0015", `Diagnostics does not have an explanation for what this means. Please report this error so one can be added.`
	}
	r.Error(errId, err, msgText+errFull+reason)
	return r
}
