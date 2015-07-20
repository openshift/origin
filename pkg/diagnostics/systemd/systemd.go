package systemd

import (
	"regexp"

	"fmt"
	"github.com/openshift/origin/pkg/diagnostics/log"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

type logEntry struct {
	Message   string `json:"MESSAGE"`
	TimeStamp string `json:"__REALTIME_TIMESTAMP"` // epoch + ns
}

type logMatcher struct { // regex for scanning log messages and interpreting them when found
	Regexp         *regexp.Regexp
	Level          log.Level
	Id             string
	Interpretation string // log with above level+id if it's simple
	KeepAfterMatch bool   // usually note only first matched entry, ignore rest
	Interpret      func(  // run this for custom logic on match
		entry *logEntry,
		matches []string,
	) (bool /* KeepAfterMatch? */, *types.DiagnosticResult)
}

type unitSpec struct {
	Name        string
	StartMatch  *regexp.Regexp // regex to look for in log messages indicating startup
	LogMatchers []logMatcher   // suspect log patterns to check for - checked in order
}

//
// -------- These are things that feed into the diagnostics definitions -----------
//

// Reusable log matchers:
var badImageTemplate = logMatcher{
	Regexp: regexp.MustCompile(`Unable to find an image for .* due to an error processing the format: %!v\\(MISSING\\)`),
	Level:  log.InfoLevel,
	Interpretation: `
This error indicates openshift was given the flag --images including an invalid format variable.
Valid formats can include (literally) ${component} and ${version}.
This could be a typo or you might be intending to hardcode something,
such as a version which should be specified as e.g. v3.0, not ${v3.0}.
Note that the --images flag may be supplied via the OpenShift master,
node, or "openshift ex registry/router" invocations and should usually
be the same for each.`,
}

// captures for logMatcher Interpret functions to store state between matches
var tlsClientErrorSeen map[string]bool

// TODO, these should probably be split into interfaces similar to Diagnostic
// Specify what units we can check and what to look for and say about it
var unitLogSpecs = []*unitSpec{
	{
		Name:       "openshift-master",
		StartMatch: regexp.MustCompile("Starting an OpenShift master"),
		LogMatchers: []logMatcher{
			badImageTemplate,
			{
				Regexp:         regexp.MustCompile("Unable to decode an event from the watch stream: local error: unexpected message"),
				Level:          log.InfoLevel,
				Id:             "sdLogOMIgnore",
				Interpretation: "You can safely ignore this message.",
			},
			{
				Regexp: regexp.MustCompile("HTTP probe error: Get .*/healthz: dial tcp .*:10250: connection refused"),
				Level:  log.InfoLevel,
				Id:     "sdLogOMhzRef",
				Interpretation: `
The OpenShift master does a health check on nodes that are defined in
its records, and this is the result when the node is not available yet.
Since the master records are typically created before the node is
available, this is not usually a problem, unless it continues in the
logs after the node is actually available.`,
			},
			{
				// TODO: don't rely on ipv4 format, should be ipv6 "soon"
				Regexp: regexp.MustCompile("http: TLS handshake error from ([\\d.]+):\\d+: remote error: bad certificate"),
				Level:  log.WarnLevel,
				Interpret: func(entry *logEntry, matches []string) (bool, *types.DiagnosticResult) {
					r := types.NewDiagnosticResult("openshift-master.journald")

					client := matches[1]
					prelude := fmt.Sprintf("Found 'openshift-master' journald log message:\n  %s\n", entry.Message)
					if tlsClientErrorSeen == nil { // first time this message was seen
						tlsClientErrorSeen = map[string]bool{client: true}
						// TODO: too generic, adjust message depending on subnet of the "from" address
						r.Warn("sdLogOMreBadCert", nil, prelude+`
This error indicates that a client attempted to connect to the master
HTTPS API server but broke off the connection because the master's
certificate is not validated by a cerificate authority (CA) acceptable
to the client. There are a number of ways this can occur, some more
problematic than others.

At this time, the OpenShift master certificate is signed by a private CA
(created the first time the master runs) and clients should have a copy of
that CA certificate in order to validate connections to the master. Most
likely, either:
1. the master has generated a new CA (after the administrator deleted
   the old one) and the client has a copy of the old CA cert, or
2. the client hasn't been configured with a private CA at all (or the
   wrong one), or
3. the client is attempting to reach the master at a URL that isn't
   covered by the master's server certificate, e.g. a public-facing
   name or IP that isn't known to the master automatically; this may
   need to be specified with the --public-master flag on the master
   in order to generate a new server certificate including it.

Clients of the master may include users, nodes, and infrastructure
components running as containers. Check the "from" IP address in the
log message:
* If it is from a SDN IP, it is likely from an infrastructure
  component. Check pod logs and recreate it with the correct CA cert.
  Routers and registries won't work properly with the wrong CA.
* If it is from a node IP, the client is likely a node. Check the
  openshift-node and openshift-sdn-node logs and reconfigure with the
  correct CA cert. Nodes will be unable to create pods until this is
  corrected.
* If it is from an external IP, it is likely from a user (CLI, browser,
  etc.). osc and openshift clients should be configured with the correct
  CA cert; browsers can also add CA certs but it is usually easier
  to just have them accept the server certificate on the first visit
  (so this message may simply indicate that the master generated a new
  server certificate, e.g. to add a different --public-master, and a
  browser hasn't accepted it yet and is still attempting API calls;
  try logging out of the console and back in again).`)

					} else if !tlsClientErrorSeen[client] {
						tlsClientErrorSeen[client] = true
						r.Warn("sdLogOMreBadCert", nil, prelude+`This message was diagnosed above, but for a different client address.`)
					} // else, it's a repeat, don't mention it
					return true /* show once for every client failing to connect, not just the first */, r
				},
			},
			{
				// user &{system:anonymous  [system:unauthenticated]} -> /api/v1beta1/services?namespace="
				Regexp: regexp.MustCompile("system:anonymous\\W*system:unauthenticated\\W*/api/v1beta1/services\\?namespace="),
				Level:  log.WarnLevel,
				Id:     "sdLogOMunauthNode",
				Interpretation: `
This indicates the OpenShift API server (master) received an unscoped
request to get Services. Requests like this probably come from an
OpenShift node trying to discover where it should proxy services.

However, the request was unauthenticated, so it was denied. The node
either did not offer a client certificate for credential, or offered an
invalid one (not signed by the certificate authority the master uses).
The node will not be able to function without this access.

Unfortunately, this message does not tell us *which* node is the
problem. But running diagnostics on your node hosts should find a log
message for any node with this problem.
`,
			},
		},
	},
	{
		Name:       "openshift-node",
		StartMatch: regexp.MustCompile("Starting an OpenShift node"),
		LogMatchers: []logMatcher{
			badImageTemplate,
			{
				Regexp: regexp.MustCompile(`error updating node status, will retry:.*system:(\S+) cannot get on minions with name "(\S+)" in default|Failed to list .*Forbidden: "\S+" system:node-\S+ cannot list on (pods|services) in`),
				Level:  log.ErrorLevel,
				Id:     "sdLogONnodePerm",
				Interpretation: `
openshift-node lacks the permission to update the node's status or request
its responsibilities from the OpenShift master API. This host will not
function as a node until this is resolved. Pods scheduled for this node
will remain in pending or unknown state forever.

This probably indicates a problem with policy as node credentials in beta3
allow access to anything (later, they will be constrained only to pods
that belong to them). This message indicates that the node credentials
are authenticated, but not authorized for the necessary access.

One way to encounter this is to start the master with data from an older
installation (e.g. beta2) in etcd. The default startup will not update
existing policy to allow node access as they would have if starting with
an empty etcd. In this case, the following command (as admin):

    osc get rolebindings -n master

... should show group system:nodes has the master/system:component role.
If that is missing, you may wish to rewrite the bootstrap policy with:

    POLICY=/var/lib/openshift/openshift.local.policy/policy.json
    CONF=/etc/openshift/master.yaml
    openshift admin overwrite-policy --filename=$POLICY --master-config=$CONF

If that is not the problem, then it may be that access controls on nodes
have been put in place and are blocking this request; check the error
message to see whether the node is attempting to use the wrong node name.
`,
			},
			{
				Regexp: regexp.MustCompile("Unable to load services: Get (http\\S+/api/v1beta1/services\\?namespace=): (.+)"), // e.g. x509: certificate signed by unknown authority
				Level:  log.ErrorLevel,
				Id:     "sdLogONconnMaster",
				Interpretation: `
openshift-node could not connect to the OpenShift master API in order
to determine its responsibilities. This host will not function as a node
until this is resolved. Pods scheduled for this node will remain in
pending or unknown state forever.`,
			},
			{
				Regexp: regexp.MustCompile(`Unable to load services: request.*403 Forbidden: Forbidden: "/api/v1beta1/services\?namespace=" denied by default`),
				Level:  log.ErrorLevel,
				Id:     "sdLogONMasterForbids",
				Interpretation: `
openshift-node could not connect to the OpenShift master API to determine
its responsibilities because it lacks the proper credentials. Nodes
should specify a client certificate in order to identify themselves to
the master. This message typically means that either no client key/cert
was supplied, or it is not validated by the certificate authority (CA)
the master uses. You should supply a correct client key and certificate
to the .kubeconfig specified in /etc/sysconfig/openshift-node

This host will not function as a node until this is resolved. Pods
scheduled for this node will remain in pending or unknown state forever.`,
			},
			{
				Regexp: regexp.MustCompile("Could not find an allocated subnet for this minion.*Waiting.."),
				Level:  log.WarnLevel,
				Id:     "sdLogOSNnoSubnet",
				Interpretation: `
This warning occurs when openshift-node is trying to request the
SDN subnet it should be configured with according to openshift-sdn-master,
but either can't connect to it ("All the given peers are not reachable")
or has not yet been assigned a subnet ("Key not found").

This can just be a matter of waiting for the master to become fully
available and define a record for the node (aka "minion") to use,
and openshift-node will wait until that occurs, so the presence
of this message in the node log isn't necessarily a problem as
long as the SDN is actually working, but this message may help indicate
the problem if it is not working.

If the master is available and this node's record is defined and this
message persists, then it may be a sign of a different misconfiguration.
Unfortunately the message is not specific about why the connection failed.
Check the master's URL in the node configuration.
 * Is the protocol http? It should be https.
 * Can you reach the address and port from the node using curl?
   ("404 page not found" is correct response)`,
			},
		},
	},
	{
		Name:       "docker",
		StartMatch: regexp.MustCompile(`Starting Docker Application Container Engine.`), // RHEL Docker at least
		LogMatchers: []logMatcher{
			{
				Regexp: regexp.MustCompile(`Usage: docker \\[OPTIONS\\] COMMAND`),
				Level:  log.ErrorLevel,
				Id:     "sdLogDbadOpt",
				Interpretation: `
This indicates that docker failed to parse its command line
successfully, so it just printed a standard usage message and exited.
Its command line is built from variables in /etc/sysconfig/docker
(which may be overridden by variables in /etc/sysconfig/openshift-sdn-node)
so check there for problems.

The OpenShift node will not work on this host until this is resolved.`,
			},
			{
				Regexp: regexp.MustCompile(`^Unable to open the database file: unable to open database file$`),
				Level:  log.ErrorLevel,
				Id:     "sdLogDopenDB",
				Interpretation: `
This indicates that docker failed to record its state to its database.
The most likely reason is that it is out of disk space. It is also
possible for other device or permissions problems to be at fault.

Sometimes this is due to excess completed containers not being cleaned
up. You can delete all completed containers with this command (running
containers will not be deleted):

  # docker rm $(docker ps -qa)

Whatever the reason, docker will not function in this state.
The OpenShift node will not work on this host until this is resolved.`,
			},
			{
				Regexp: regexp.MustCompile(`no space left on device$`),
				Level:  log.ErrorLevel,
				Id:     "sdLogDfull",
				Interpretation: `
This indicates that docker has run out of space for container volumes
or metadata (by default, stored in /var/lib/docker, but configurable).

docker will not function in this state. It requires that disk space be
added to the relevant filesystem or files deleted to make space.
Sometimes this is due to excess completed containers not being cleaned
up. You can delete all completed containers with this command (running
containers will not be deleted):

  # docker rm $(docker ps -qa)

The OpenShift node will not work on this host until this is resolved.`,
			},
			{ // generic error seen - do this last
				Regexp: regexp.MustCompile(`\\slevel="fatal"\\s`),
				Level:  log.ErrorLevel,
				Id:     "sdLogDfatal",
				Interpretation: `
This is not a known problem, but it is causing Docker to crash,
so the OpenShift node will not work on this host until it is resolved.`,
			},
		},
	},
	{
		Name:        "openvswitch",
		StartMatch:  regexp.MustCompile("Starting Open vSwitch"),
		LogMatchers: []logMatcher{},
	},
}
