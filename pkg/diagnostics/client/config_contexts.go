package client

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclientcmd "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	kclientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/diagnostics/log"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

type ConfigContext struct {
	RawConfig   *kclientcmdapi.Config
	ContextName string
}

const (
	currentContextMissing = `Your client config specifies a current context of '{{.context}}'
which is not defined; it is likely that a mistake was introduced while
manually editing your config. If this is a simple typo, you may be
able to fix it manually.
The OpenShift master creates a fresh config when it is started; it may be
useful to use this as a base if available.`

	currentContextSummary = `The current context from client config is '{{.context}}'
This will be used by default to contact your OpenShift server.
`
	contextDesc = `
For client config context '{{.context}}':
The server URL is '{{.server}}'
The user authentication is '{{.user}}'
The current project is '{{.project}}'
`
	currContextDesc = `
The current client config context is '{{.context}}':
The server URL is '{{.server}}'
The user authentication is '{{.user}}'
The current project is '{{.project}}'
`
	clientNoResolve = `
This usually means that the hostname does not resolve to an IP.
Hostnames should usually be resolved via DNS or an /etc/hosts file.
Ensure that the hostname resolves correctly from your host before proceeding.
Of course, your config could also simply have the wrong hostname specified.
`
	clientUnknownCa = `
This means that we cannot validate the certificate in use by the
OpenShift API server, so we cannot securely communicate with it.
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
This means that for client connections to the OpenShift API server, you
(or your kubeconfig) specified both a validating certificate authority
and that the client should bypass connection security validation.

This is not allowed because it is likely to be a mistake.

If you want to use --insecure-skip-tls-verify to bypass security (which
is usually a bad idea anyway), then you need to also clear the CA cert
from your command line options or kubeconfig file(s). Of course, it
would be far better to obtain and use a correct CA cert.
`
	clientInvCertName = `
This means that the certificate in use by the OpenShift API server
(master) does not match the hostname by which you are addressing it:
  %s
so a secure connection is not allowed. In theory, this *could* mean that
someone is intercepting your connection and presenting a certificate
that is valid but for a different server, which is why secure validation
fails in this case.

However, the most likely explanation is that the server certificate
needs to be updated to include the name you are using to reach it.

If the OpenShift server is generating its own certificates (which
is default), then the --public-master flag on the OpenShift master is
usually the easiest way to do this. If you need something more complicated
(for instance, multiple public addresses for the API, or your own CA),
then you will need to custom-generate the server certificate with the
right names yourself.

If you are unconcerned about any of this, you can add the
--insecure-skip-tls-verify flag to bypass secure (TLS) verification,
but this is risky and should not be necessary.
** Connections could be intercepted and your credentials stolen. **
`
	clientConnRefused = `
This means that when we tried to connect to the OpenShift API
server (master), we reached the host, but nothing accepted the port
connection. This could mean that the OpenShift master is stopped, or
that a firewall or security policy is blocking access at that port.

You will not be able to connect or do anything at all with OpenShift
until this server problem is resolved or you specify a corrected
server address.`

	clientConnTimeout = `
This means that when we tried to connect to the OpenShift API server
(master), we could not reach the host at all.
* You may have specified the wrong host address.
* This could mean the host is completely unavailable (down).
* This could indicate a routing problem or a firewall that simply
  drops requests rather than responding by reseting the connection.
* It does not generally mean that DNS name resolution failed (which
  would be a different error) though the problem could be that it
  gave the wrong address.`
	clientMalformedHTTP = `
This means that when we tried to connect to the OpenShift API server
(master) with a plain HTTP connection, the server did not speak
HTTP back to us. The most common explanation is that a secure server
is listening but you specified an http: connection instead of https:.
There could also be another service listening at the intended port
speaking some other protocol entirely.

You will not be able to connect or do anything at all with OpenShift
until this server problem is resolved or you specify a corrected
server address.`
	clientMalformedTLS = `
This means that when we tried to connect to the OpenShift API server
(master) with a secure HTTPS connection, the server did not speak
HTTPS back to us. The most common explanation is that the server
listening at that port is not the secure server you expected - it
may be a non-secure HTTP server or the wrong service may be
listening there, or you may have specified an incorrect port.

You will not be able to connect or do anything at all with OpenShift
until this server problem is resolved or you specify a corrected
server address.`
	clientUnauthn = `
This means that when we tried to make a request to the OpenShift API
server, your kubeconfig did not present valid credentials to
authenticate your client. Credentials generally consist of a client
key/certificate or an access token. Your kubeconfig may not have
presented any, or they may be invalid.`
	clientUnauthz = `
This means that when we tried to make a request to the OpenShift API
server, the request required credentials that were not presented.
This can happen when an authentication token expires. Try logging in
with this user again.`
)

var (
	invalidCertNameRx = regexp.MustCompile("x509: certificate is valid for (\\S+, )+not (\\S+)")
)

func (d ConfigContext) Name() string {
	return fmt.Sprintf("ConfigContext[%s]", d.ContextName)
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

func (d ConfigContext) Check() *types.DiagnosticResult {
	r := types.NewDiagnosticResult("ConfigContext")

	isDefaultContext := d.RawConfig.CurrentContext == d.ContextName

	// prepare bad news message
	errorKey := "clientCfgError"
	unusableLine := fmt.Sprintf("The client config context '%s' is unusable", d.ContextName)
	if isDefaultContext {
		errorKey = "currentccError"
		unusableLine = fmt.Sprintf("The current client config context '%s' is unusable", d.ContextName)
	}

	// check that the context and its constitutuents are defined in the kubeconfig
	context, exists := d.RawConfig.Contexts[d.ContextName]
	if !exists {
		r.Errorf(errorKey, nil, "%s:\n Client config context '%s' is not defined.", unusableLine, d.ContextName)
		return r
	}
	clusterName := context.Cluster
	cluster, exists := d.RawConfig.Clusters[clusterName]
	if !exists {
		r.Errorf(errorKey, nil, "%s:\n Client config context '%s' has a cluster '%s' which is not defined.", unusableLine, d.ContextName, clusterName)
		return r
	}
	authName := context.AuthInfo
	if _, exists := d.RawConfig.AuthInfos[authName]; !exists {
		r.Errorf(errorKey, nil, "%s:\n Client config context '%s' has a user identity '%s' which is not defined.", unusableLine, d.ContextName, authName)
		return r
	}

	// we found a fully-defined context
	project := context.Namespace
	if project == "" {
		project = kapi.NamespaceDefault // OpenShift/k8s fills this in if missing
	}
	msgData := log.Hash{"context": d.ContextName, "server": cluster.Server, "user": authName, "project": project}
	msgText := contextDesc
	if isDefaultContext {
		msgText = currContextDesc
	}

	// Actually send a request to see if context has connectivity.
	// Note: we cannot reuse factories as they cache the clients, so build new factory for each context.
	osClient, _, err := osclientcmd.NewFactory(kclientcmd.NewDefaultClientConfig(*d.RawConfig, &kclientcmd.ConfigOverrides{Context: *context})).Clients()
	// client create now fails if cannot connect to server, so address connectivity errors below
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
			msgData["projects"] = list
			if len(list) == 0 {
				r.Infot("CCctxSuccess", msgText+"Successfully requested project list, but it is empty, so user has no access to anything.", msgData)
			} else {
				r.Infot("CCctxSuccess", msgText+"Successfully requested project list; has access to project(s):\n  {{.projects}}", msgData)
			}
			return r
		}
	}

	// something went wrong; couldn't create client or get project list.
	// interpret the terse error messages with helpful info.
	errMsg := err.Error()
	msgData["errMsg"] = fmt.Sprintf("(%T) %[1]v", err)
	var reason, errId string
	switch {
	case regexp.MustCompile("dial tcp: lookup (\\S+): no such host").MatchString(errMsg):
		errId, reason = "clientNoResolve", clientNoResolve
	case strings.Contains(errMsg, "x509: certificate signed by unknown authority"):
		errId, reason = "clientUnknownCa", clientUnknownCa
	case strings.Contains(errMsg, "specifying a root certificates file with the insecure flag is not allowed"):
		errId, reason = "clientUnneededCa", clientUnneededCa
	case invalidCertNameRx.MatchString(errMsg):
		match := invalidCertNameRx.FindStringSubmatch(errMsg)
		serverHost := match[len(match)-1]
		errId, reason = "clientInvCertName", fmt.Sprintf(clientInvCertName, serverHost)
	case regexp.MustCompile("dial tcp (\\S+): connection refused").MatchString(errMsg):
		errId, reason = "clientConnRefused", clientConnRefused
	case regexp.MustCompile("dial tcp (\\S+): (?:connection timed out|i/o timeout|no route to host)").MatchString(errMsg):
		errId, reason = "clientConnTimeout", clientConnTimeout
	case strings.Contains(errMsg, "malformed HTTP response"):
		errId, reason = "clientMalformedHTTP", clientMalformedHTTP
	case strings.Contains(errMsg, "tls: oversized record received with length"):
		errId, reason = "clientMalformedTLS", clientMalformedTLS
	case strings.Contains(errMsg, `403 Forbidden: Forbidden: "/osapi/v1beta1/projects?namespace=" denied by default`):
		errId, reason = "clientUnauthn", clientUnauthn
	case regexp.MustCompile("401 Unauthorized: Unauthorized$").MatchString(errMsg):
		errId, reason = "clientUnauthz", clientUnauthz
	default:
		errId, reason = "clientUnknownConnErr", `Diagnostics does not have an explanation for what this means. Please report this error so one can be added.`
	}
	r.Errort(errId, err, msgText+"{{.errMsg}}\n"+reason, msgData)
	return r
}
