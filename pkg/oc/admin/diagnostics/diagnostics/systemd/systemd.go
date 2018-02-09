package systemd

import (
	"fmt"
	"regexp"

	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/log"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/types"
)

type logEntry struct {
	Message   string `json:"MESSAGE"`
	TimeStamp string `json:"__REALTIME_TIMESTAMP"` // epoch + ns
}

// logMatcher provides a regex for scanning log messages and parameters to interpret them when found.
type logMatcher struct {
	Regexp         *regexp.Regexp
	Level          log.Level
	Id             string
	Interpretation string // log with above level+id if it's simple
	KeepAfterMatch bool   // usually note only first matched entry, ignore rest
	// Interpret, if provided, runs custom logic against a match. Its return bool is the equivalent of KeepAfterMatch.
	Interpret func(
		entry *logEntry,
		matches []string,
		r types.DiagnosticResult,
	) bool
}

type unitSpec struct {
	Names       []string
	StartMatch  *regexp.Regexp // regex to look for in log messages indicating startup
	LogMatchers []logMatcher   // suspect log patterns to check for - checked in order
}

//
// -------- These are things that feed into the diagnostics definitions -----------
//

// Reusable log matchers:
var badImageTemplate = logMatcher{
	Regexp: regexp.MustCompile(`Unable to find an image for .* due to an error processing the format: the key .* is not recognized`),
	Level:  log.InfoLevel,
	Interpretation: `
This error indicates the openshift command was given the --images flag
with an invalid format variable. Valid formats can include (literally)
${component} and ${version}; any others will cause this error.
This could be a typo or you might be intending to hardcode something,
such as a version which should be specified as e.g. v3.0, not ${v3.0}.
Note that the --images flag may be supplied via the master, node, or
"oc adm registry/router" invocations and should usually
be the same for each.`,
}

// captures for logMatcher Interpret functions to store state between matches
var tlsClientErrorSeen map[string]bool

// TODO, these should probably be split into interfaces similar to Diagnostic
// Specify what units we can check and what to look for and say about it
var unitLogSpecs = []*unitSpec{
	{
		Names:      []string{"origin-master-api", "atomic-openshift-master-api"},
		StartMatch: regexp.MustCompile("Starting \\w+ Master"),
		LogMatchers: []logMatcher{
			badImageTemplate,
			{
				Regexp:         regexp.MustCompile("Unable to decode an event from the watch stream: local error: unexpected message"),
				Level:          log.InfoLevel,
				Id:             "DS2003",
				Interpretation: "You can safely ignore this message.",
			},
			{
				// TODO: don't rely on ipv4 format, should be ipv6 "soon"
				Regexp: regexp.MustCompile("http: TLS handshake error from ([\\d.]+):\\d+: remote error: bad certificate"),
				Level:  log.WarnLevel,
				Interpret: func(entry *logEntry, matches []string, r types.DiagnosticResult) bool {
					client := matches[1]
					prelude := fmt.Sprintf("Found master journald log message:\n  %s\n", entry.Message)
					if tlsClientErrorSeen == nil { // first time this message was seen
						tlsClientErrorSeen = map[string]bool{client: true}
						// TODO: too generic, adjust message depending on subnet of the "from" address
						r.Warn("DS2001", nil, prelude+`
This error indicates that a client attempted to connect to the master
HTTPS API server but broke off the connection because the master's
certificate is not validated by a cerificate authority (CA) acceptable
to the client. There are a number of ways this can occur, some more
problematic than others.

At this time, the master API certificate is signed by a private CA
(created the first time the master runs) and clients should have a copy of
that CA certificate in order to validate connections to the master. Most
likely, either:
1. the master has generated a new CA (e.g. after the administrator
   deleted the old one) and the client has a copy of the old CA cert, or
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
  node logs and reconfigure with the correct CA cert.
  Nodes will be unable to create pods until this is corrected.
* If it is from an external IP, it is likely from a user (CLI, browser,
  etc.). Command line clients should be configured with the correct
  CA cert; browsers can also add CA certs but it is usually easier
  to just have them accept the server certificate on the first visit
  (so this message may simply indicate that the master generated a new
  server certificate, e.g. to add a different --public-master, and a
  browser hasn't accepted it yet and is still attempting API calls;
  try logging out of the console and back in again).`)

					} else if !tlsClientErrorSeen[client] {
						tlsClientErrorSeen[client] = true
						r.Warn("DS2002", nil, prelude+`This message was diagnosed above, but for a different client address.`)
					} // else, it's a repeat, don't mention it
					return true // show once for every client failing to connect, not just the first
				},
			},
		},
	},
	{
		Names:      []string{"origin-node", "atomic-openshift-node"},
		StartMatch: regexp.MustCompile("Starting \\w+ Node"), //systemd puts this out; could change
		LogMatchers: []logMatcher{
			badImageTemplate,
			{
				Regexp: regexp.MustCompile(`Unable to register.*"system:anonymous"`),
				Level:  log.ErrorLevel,
				Id:     "DS2004",
				Interpretation: `
This node could not register with the master API because it lacks
the proper credentials. Nodes should specify a client certificate in
order to identify themselves to the master. This message typically means
that either no client key/cert was supplied, or it is not validated
by the certificate authority (CA) the master uses. You should supply
a correct client key and certificate in the .kubeconfig specified in
node-config.yaml

This host will not function as a node until this is resolved. Pods
scheduled for this node will remain in pending or unknown state forever.`,
			},
			{
				Regexp: regexp.MustCompile("Could not find an allocated subnet for"),
				Level:  log.WarnLevel,
				Id:     "DS2005",
				Interpretation: `
This warning occurs when the node is trying to request the
SDN subnet it should be configured with according to the master,
but either can't connect to it or has not yet been assigned a subnet.

This can occur before the master becomes fully available and defines a
record for the node to use; the node will wait until that occurs,
so the presence of this message in the node log isn't necessarily a
problem as long as the SDN is actually working, but this message may
help indicate the problem if it is not working.

If the master is available and this log message persists, then it may
be a sign of a different misconfiguration. Check the master's URL in
the node kubeconfig.
 * Is the protocol http? It should be https.
 * Can you reach the address and port from the node using curl -k?
`,
			},
		},
	},
	{
		Names:      []string{"docker"},
		StartMatch: regexp.MustCompile(`Starting Docker`), // RHEL Docker at least
		LogMatchers: []logMatcher{
			{
				Regexp: regexp.MustCompile(`Usage: docker \\[OPTIONS\\] COMMAND`),
				Level:  log.ErrorLevel,
				Id:     "DS2006",
				Interpretation: `
This indicates that docker failed to parse its command line
successfully, so it just printed a standard usage message and exited.
Its command line is built from variables in /etc/sysconfig/docker
(which may be overridden by variables in /etc/sysconfig/openshift-sdn-node)
so check there for problems.

The node will not run on this host until this is resolved.`,
			},
			{
				Regexp: regexp.MustCompile(`^Unable to open the database file: unable to open database file$`),
				Level:  log.ErrorLevel,
				Id:     "DS2007",
				Interpretation: `
This indicates that docker failed to record its state to its database.
The most likely reason is that it is out of disk space. It is also
possible for other device or permissions problems to be at fault.

Sometimes this is due to excess completed containers not being cleaned
up. You can delete all completed containers with this command (running
containers will not be deleted):

  # docker rm $(docker ps -qa)

Whatever the reason, docker will not function in this state.
The node will not run on this host until this is resolved.`,
			},
			{
				Regexp: regexp.MustCompile(`no space left on device$`),
				Level:  log.ErrorLevel,
				Id:     "DS2008",
				Interpretation: `
This indicates that docker has run out of space for container volumes
or metadata (by default, stored in /var/lib/docker, but configurable).

docker will not function in this state. It requires that disk space be
added to the relevant filesystem or files deleted to make space.
Sometimes this is due to excess completed containers not being cleaned
up. You can delete all completed containers with this command (running
containers will not be deleted):

  # docker rm $(docker ps -qa)

The node will not run on this host until this is resolved.`,
			},
			{ // generic error seen - do this last
				Regexp: regexp.MustCompile(`\\slevel="fatal"\\s`),
				Level:  log.ErrorLevel,
				Id:     "DS2009",
				Interpretation: `
This is not a known problem, but it is causing Docker to crash,
so the node will not run on this host until it is resolved.`,
			},
		},
	},
}
