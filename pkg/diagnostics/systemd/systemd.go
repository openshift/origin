package systemd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/openshift/origin/pkg/diagnostics/discovery"
	"github.com/openshift/origin/pkg/diagnostics/log"
	"github.com/openshift/origin/pkg/diagnostics/types"
	"github.com/openshift/origin/pkg/diagnostics/types/diagnostic"
	"io"
	"os/exec"
	"regexp"
)

type logEntry struct {
	Message string // I feel certain we will want more fields at some point
}

type logMatcher struct { // regex for scanning log messages and interpreting them when found
	Regexp         *regexp.Regexp
	Level          log.Level
	Id             string
	Interpretation string // log with above level+id if it's simple
	KeepAfterMatch bool   // usually note only first matched entry, ignore rest
	Interpret      func(  // run this for custom logic on match
		env *discovery.Environment,
		entry *logEntry,
		matches []string,
	) bool // KeepAfterMatch?
}

type unitSpec struct {
	Name        string
	StartMatch  *regexp.Regexp // regex to look for in log messages indicating startup
	LogMatchers []logMatcher   // suspect log patterns to check for - checked in order
}

//
// -------- Things that feed into the diagnostics definitions -----------
// Search for Diagnostics for the actual diagnostics.

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
				Interpret: func(env *discovery.Environment, entry *logEntry, matches []string) bool {
					client := matches[1]
					prelude := fmt.Sprintf("Found 'openshift-master' journald log message:\n  %s\n", entry.Message)
					if tlsClientErrorSeen == nil { // first time this message was seen
						tlsClientErrorSeen = map[string]bool{client: true}
						// TODO: too generic, adjust message depending on subnet of the "from" address
						env.Log.Warnm("sdLogOMreBadCert", log.Msg{"client": client, "text": prelude + `
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
  try logging out of the console and back in again).`})
					} else if !tlsClientErrorSeen[client] {
						tlsClientErrorSeen[client] = true
						env.Log.Warnm("sdLogOMreBadCert", log.Msg{"client": client, "text": prelude +
							`This message was diagnosed above, but for a different client address.`})
					} // else, it's a repeat, don't mention it
					return true // show once for every client failing to connect, not just the first
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
		Name:        "openshift-sdn-master",
		StartMatch:  regexp.MustCompile("Starting OpenShift SDN Master"),
		LogMatchers: []logMatcher{},
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
		},
	},
	{
		Name:       "openshift-sdn-node",
		StartMatch: regexp.MustCompile("Starting OpenShift SDN node"),
		LogMatchers: []logMatcher{
			{
				Regexp: regexp.MustCompile("Could not find an allocated subnet for this minion.*Waiting.."),
				Level:  log.WarnLevel,
				Id:     "sdLogOSNnoSubnet",
				Interpretation: `
This warning occurs when openshift-sdn-node is trying to request the
SDN subnet it should be configured with according to openshift-sdn-master,
but either can't connect to it ("All the given peers are not reachable")
or has not yet been assigned a subnet ("Key not found").

This can just be a matter of waiting for the master to become fully
available and define a record for the node (aka "minion") to use,
and openshift-sdn-node will wait until that occurs, so the presence
of this message in the node log isn't necessarily a problem as
long as the SDN is actually working, but this message may help indicate
the problem if it is not working.

If the master is available and this node's record is defined and this
message persists, then it may be a sign of a different misconfiguration.
Unfortunately the message is not specific about why the connection failed.
Check MASTER_URL in /etc/sysconfig/openshift-sdn-node:
 * Is the protocol https? It should be http.
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

var systemdRelevant = func(env *discovery.Environment) (skip bool, reason string) {
	if !env.HasSystemd {
		return true, "systemd is not present on this host"
	}
	return false, ""
}

//
// -------- The actual diagnostics definitions -----------
//

var Diagnostics = map[string]diagnostic.Diagnostic{

	"AnalyzeLogs": {
		Description: "Check for problems in systemd service logs since each service last started",
		Condition:   systemdRelevant,
		Run: func(env *discovery.Environment) {
			for _, unit := range unitLogSpecs {
				if svc := env.SystemdUnits[unit.Name]; svc.Enabled || svc.Active {
					env.Log.Infom("sdCheckLogs", log.Msg{"tmpl": "Checking journalctl logs for '{{.name}}' service", "name": unit.Name})
					matchLogsSinceLastStart(unit, env)
				}
			}
		},
	},

	"UnitStatus": {
		Description: "Check status for OpenShift-related systemd units",
		Condition:   systemdRelevant,
		Run: func(env *discovery.Environment) {
			u := env.SystemdUnits
			unitRequiresUnit(env.Log, u["openshift-node"], u["iptables"], `
iptables is used by OpenShift nodes for container networking.
Connections to a container will fail without it.`)
			unitRequiresUnit(env.Log, u["openshift-node"], u["docker"], `OpenShift nodes use Docker to run containers.`)
			unitRequiresUnit(env.Log, u["openshift"], u["docker"], `OpenShift nodes use Docker to run containers.`)
			// node's dependency on openvswitch is a special case.
			// We do not need to enable ovs because openshift-node starts it for us.
			if u["openshift-node"].Active && !u["openvswitch"].Active {
				env.Log.Error("sdUnitSDNreqOVS", `
systemd unit openshift-node is running but openvswitch is not.
Normally openshift-node starts openvswitch once initialized.
It is likely that openvswitch has crashed or been stopped.

The software-defined network (SDN) enables networking between
containers on different nodes. Containers will not be able to
connect to each other without the openvswitch service carrying
this traffic.

An administrator can start openvswitch with:

  # systemctl start openvswitch

To ensure it is not repeatedly failing to run, check the status and logs with:

  # systemctl status openvswitch
  # journalctl -ru openvswitch `)
			}
			// Anything that is enabled but not running deserves notice
			for name, unit := range u {
				if unit.Enabled && !unit.Active {
					env.Log.Errorm("sdUnitInactive", log.Msg{"tmpl": `
The {{.unit}} systemd unit is intended to start at boot but is not currently active.
An administrator can start the {{.unit}} unit with:

  # systemctl start {{.unit}}

To ensure it is not failing to run, check the status and logs with:

  # systemctl status {{.unit}}
  # journalctl -ru {{.unit}}`, "unit": name})
				}
			}
		},
	},
}

//
// -------- Functions used by the diagnostics -----------
//

func unitRequiresUnit(logger *log.Logger, unit types.SystemdUnit, requires types.SystemdUnit, reason string) {
	if (unit.Active || unit.Enabled) && !requires.Exists {
		logger.Errorm("sdUnitReqLoaded", log.Msg{"tmpl": `
systemd unit {{.unit}} depends on unit {{.required}}, which is not loaded.
{{.reason}}
An administrator probably needs to install the {{.required}} unit with:

  # yum install {{.required}}

If it is already installed, you may to reload the definition with:

  # systemctl reload {{.required}}
  `, "unit": unit.Name, "required": requires.Name, "reason": reason})
	} else if unit.Active && !requires.Active {
		logger.Errorm("sdUnitReqActive", log.Msg{"tmpl": `
systemd unit {{.unit}} is running but {{.required}} is not.
{{.reason}}
An administrator can start the {{.required}} unit with:

  # systemctl start {{.required}}

To ensure it is not failing to run, check the status and logs with:

  # systemctl status {{.required}}
  # journalctl -ru {{.required}}
  `, "unit": unit.Name, "required": requires.Name, "reason": reason})
	} else if unit.Enabled && !requires.Enabled {
		logger.Warnm("sdUnitReqEnabled", log.Msg{"tmpl": `
systemd unit {{.unit}} is enabled to run automatically at boot, but {{.required}} is not.
{{.reason}}
An administrator can enable the {{.required}} unit with:

  # systemctl enable {{.required}}
  `, "unit": unit.Name, "required": requires.Name, "reason": reason})
	}
}

func matchLogsSinceLastStart(unit *unitSpec, env *discovery.Environment) {
	cmd := exec.Command("journalctl", "-ru", unit.Name, "--output=json")
	// JSON comes out of journalctl one line per record
	lineReader, reader, err := func(cmd *exec.Cmd) (*bufio.Scanner, io.ReadCloser, error) {
		stdout, err := cmd.StdoutPipe()
		if err == nil {
			lineReader := bufio.NewScanner(stdout)
			if err = cmd.Start(); err == nil {
				return lineReader, stdout, nil
			}
		}
		return nil, nil, err
	}(cmd)
	if err != nil {
		env.Log.Errorm("sdLogReadErr", log.Msg{"tmpl": `
Diagnostics failed to query journalctl for the '{{.unit}}' unit logs.
This should be very unusual, so please report this error:
{{.error}}`, "unit": unit.Name, "error": errStr(err)})
		return
	}
	defer func() { // close out pipe once done reading
		reader.Close()
		cmd.Wait()
	}()
	entryTemplate := logEntry{Message: `json:"MESSAGE"`}
	matchCopy := append([]logMatcher(nil), unit.LogMatchers...) // make a copy, will remove matchers after they match something
	for lineReader.Scan() {                                     // each log entry is a line
		if len(matchCopy) == 0 { // if no rules remain to match
			break // don't waste time reading more log entries
		}
		bytes, entry := lineReader.Bytes(), entryTemplate
		if err := json.Unmarshal(bytes, &entry); err != nil {
			env.Log.Debugm("sdLogBadJSON", log.Msg{"message": string(bytes), "error": errStr(err),
				"tmpl": "Couldn't read the JSON for this log message:\n{{.message}}\nGot error {{.error}}"})
		} else {
			if unit.StartMatch.MatchString(entry.Message) {
				break // saw the log message where the unit started; done looking.
			}
			for index, match := range matchCopy { // match log message against provided matchers
				if strings := match.Regexp.FindStringSubmatch(entry.Message); strings != nil {
					// if matches: print interpretation, remove from matchCopy, and go on to next log entry
					keep := match.KeepAfterMatch
					if match.Interpret != nil {
						keep = match.Interpret(env, &entry, strings)
					} else {
						prelude := fmt.Sprintf("Found '%s' journald log message:\n  %s\n", unit.Name, entry.Message)
						env.Log.Log(match.Level, match.Id, log.Msg{"text": prelude + match.Interpretation, "unit": unit.Name, "logMsg": entry.Message})
					}
					if !keep { // remove matcher once seen
						matchCopy = append(matchCopy[:index], matchCopy[index+1:]...)
					}
					break
				}
			}
		}
	}
}

func errStr(err error) string {
	return fmt.Sprintf("(%T) %[1]v", err)
}
