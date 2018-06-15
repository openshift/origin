package f5

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/golang/glog"

	knet "k8s.io/apimachinery/pkg/util/net"
)

const (
	// Default F5 partition path to use for syncing route config.
	F5DefaultPartitionPath = "/Common"
	F5VxLANTunnelName      = "vxlan5000"
	F5VxLANProfileName     = "vxlan-ose"
	HTTP_CONFLICT_CODE     = 409
)

// Error implements the error interface.
func (err F5Error) Error() string {
	var msg string

	if err.err != nil {
		msg = fmt.Sprintf("error: %v", err.err)
	} else if err.Message != nil {
		msg = fmt.Sprintf("HTTP code: %d; error from F5: %s",
			err.httpStatusCode, *err.Message)
	} else {
		msg = fmt.Sprintf("HTTP code: %d.", err.httpStatusCode)
	}

	return fmt.Sprintf("Encountered an error on %s request to URL %s: %s",
		err.verb, err.url, msg)
}

// passthroughRoute represents a passthrough route for the F5 router's internal
// state.  In the F5 BIG-IP host itself, we must store this information using
// two datagroups: one that makes routename to hostname so that we can
// reconstruct this state when initializing the router, and one that maps
// hostname to poolname for use by the iRule that handles passthrough routes.
type passthroughRoute struct {
	hostname string
	poolname string
}

// reencryptRoute represents a reencrypt route for the F5 router's internal state
// similar to the passthrough route
type reencryptRoute struct {
	hostname string
	poolname string
}

// f5LTM represents an F5 BIG-IP instance.
type f5LTM struct {
	// f5LTMCfg contains the configuration parameters for an F5 BIG-IP instance.
	f5LTMCfg

	// poolMembers maps pool name to set of pool members, where the pool
	// name is a string and the set of members of a pool is represented by
	// a map with value type bool.  A pool member will be identified by
	// a string of the format "ipaddress:port".
	poolMembers map[string]map[string]bool

	// routes maps vserver name to set of routes.
	routes map[string]map[string]bool

	// passthroughRoutes maps routename to passthroughroute{hostname, poolname}.
	passthroughRoutes map[string]passthroughRoute

	// reencryptRoutes maps routename to passthroughroute{hostname, poolname}.
	reencryptRoutes map[string]reencryptRoute
}

// f5LTMCfg holds configuration for connecting to and issuing iControl
// requests against an F5 BIG-IP instance.
type f5LTMCfg struct {
	// host specifies the hostname or IP address of the F5 BIG-IP host.
	host string

	// username specifies the username with which we should authenticate with the
	// F5 BIG-IP host.
	username string

	// password specifies the password with which we should authenticate with the
	// F5 BIG-IP host.
	password string

	// httpVserver specifies the name of the vserver object for which we should
	// manipulate policy to add and remove HTTP routes.
	httpVserver string

	// httpsVserver specifies the name of the vserver object for which we should
	// manipulate policy to add and remove HTTPS routes.
	httpsVserver string

	// privkey specifies the path to the SSH private-key file for F5 BIG-IP.  The
	// file must exist with this pathname inside the F5 router's filesystem
	// namespace.  The F5 router uses this key to copy certificates and keys to
	// the F5 BIG-IP host.
	privkey string

	// insecure specifies whether we should perform strict certificate validation
	// for connections to the F5 BIG-IP host.
	insecure bool

	// partitionPath specifies the F5 partition path to use. Partitions
	// are normally used to create an access control boundary for
	// F5 users and applications.
	partitionPath string

	// vxlanGateway is the ip address assigned to the local tunnel interface
	// inside F5 box. This address is the one that the packets generated from F5
	// will carry. The pods will return the packets to this address itself.
	// It is important that the gateway be one of the ip addresses of the subnet
	// that has been generated for F5.
	vxlanGateway string

	// internalAddress is the ip address of the vtep interface used to connect to
	// VxLAN overlay. It is the hostIP address listed in the subnet generated for F5
	internalAddress string

	// setupOSDNVxLAN is the boolean that conveys if F5 needs to setup a VxLAN
	// to hook up with openshift-sdn
	setupOSDNVxLAN bool
}

const (
	// policy is the name of the local traffic policy associated with the vservers
	// for insecure (plaintext, HTTP) routes.  We add and delete rules to and from
	// this policy to configure vhost-based routing.
	httpPolicyName = "openshift_insecure_routes"

	// https_policy is the name of the local traffic policy associated with the
	// vservers for secure (TLS/SSL, HTTPS) routes.
	httpsPolicyName = "openshift_secure_routes"

	// reencryptRoutesDataGroupName is the name of the datagroup that will be used
	// by our iRule for routing SSL-passthrough routes (see below).
	reencryptRoutesDataGroupName = "ssl_reencrypt_route_dg"

	// reencryptHostsDataGroupName is the name of the datagroup that will be used
	// by our iRule for routing SSL-passthrough routes (see below).
	reencryptHostsDataGroupName = "ssl_reencrypt_servername_dg"

	// passthroughRoutesDataGroupName is the name of the datagroup that will be used
	// by our iRule for routing SSL-passthrough routes (see below).
	passthroughRoutesDataGroupName = "ssl_passthrough_route_dg"

	// passthroughHostsDataGroupName is the name of the datagroup that will be used
	// by our iRule for routing SSL-passthrough routes (see below).
	passthroughHostsDataGroupName = "ssl_passthrough_servername_dg"

	// sslPassthroughIRuleName is the name assigned to the sslPassthroughIRule
	// iRule.
	sslPassthroughIRuleName = "openshift_passthrough_irule"

	// sslPassthroughIRule is an iRule that examines the servername in TLS
	// connections and routes requests to the corresponding pool if one exists.
	//
	// You probably will not read the following TCL code.  However, if you do, you
	// may wonder, "What's up with this 'if { a - b == abs(a - b) }' nonsense?
	// Why not simply use 'if { a > b }'? WHAT IS THIS NIMWITTERY?!?" Rest
	// assured, there is *no* nimwittery here! In fact, the explanation for this
	// curiosity is an incompatibility between Google's JSON encoder and F5's JSON
	// decoder: The former produces escape sequences for the characters <, >, and
	// & (specifically, \u003c, \u003e, and \u0026) that confuse the latter.  Thus
	// as long as we are using Google's JSON encoding library and F5's iControl
	// REST API, we must avoid using the <, >, and & characters.
	sslPassthroughIRule = `
when CLIENT_ACCEPTED {
  TCP::collect
}

when CLIENT_DATA {
  # Byte 0 is the content type.
  # Bytes 1-2 are the TLS version.
  # Bytes 3-4 are the TLS payload length.
  # Bytes 5-$tls_payload_len are the TLS payload.
  binary scan [TCP::payload] cSS tls_content_type tls_version tls_payload_len

  switch $tls_version {
    "769" -
    "770" -
    "771" {
      # Content type of 22 indicates the TLS payload contains a handshake.
      if { $tls_content_type == 22 } {
        # Byte 5 (the first byte of the handshake) indicates the handshake
        # record type, and a value of 1 signifies that the handshake record is
        # a ClientHello.
        binary scan [TCP::payload] @5c tls_handshake_record_type
        if { $tls_handshake_record_type == 1 } {
          # Bytes 6-8 are the handshake length (which we ignore).
          # Bytes 9-10 are the TLS version (which we ignore).
          # Bytes 11-42 are random data (which we ignore).

          # Byte 43 is the session ID length.  Following this are three
          # variable-length fields which we shall skip over.
          set record_offset 43

          # Skip the session ID.
          binary scan [TCP::payload] @${record_offset}c tls_session_id_len
          incr record_offset [expr {1 + $tls_session_id_len}]

          # Skip the cipher_suites field.
          binary scan [TCP::payload] @${record_offset}S tls_cipher_suites_len
          incr record_offset [expr {2 + $tls_cipher_suites_len}]

          # Skip the compression_methods field.
          binary scan [TCP::payload] @${record_offset}c tls_compression_methods_len
          incr record_offset [expr {1 + $tls_compression_methods_len}]

          # Get the number of extensions, and store the extensions.
          binary scan [TCP::payload] @${record_offset}S tls_extensions_len
          incr record_offset 2
          binary scan [TCP::payload] @${record_offset}a* tls_extensions

          for { set extension_start 0 }
              { $tls_extensions_len - $extension_start == abs($tls_extensions_len - $extension_start) }
              { incr extension_start 4 } {
            # Bytes 0-1 of the extension are the extension type.
            # Bytes 2-3 of the extension are the extension length.
            binary scan $tls_extensions @${extension_start}SS extension_type extension_len

            # Extension type 00 is the ServerName extension.
            if { $extension_type == "00" } {
              # Bytes 4-5 of the extension are the SNI length (we ignore this).

              # Byte 6 of the extension is the SNI type.
              set sni_type_offset [expr {$extension_start + 6}]
              binary scan $tls_extensions @${sni_type_offset}S sni_type

              # Type 0 is host_name.
              if { $sni_type == "0" } {
                # Bytes 7-8 of the extension are the SNI data (host_name)
                # length.
                set sni_len_offset [expr {$extension_start + 7}]
                binary scan $tls_extensions @${sni_len_offset}S sni_len

                # Bytes 9-$sni_len are the SNI data (host_name).
                set sni_start [expr {$extension_start + 9}]
                binary scan $tls_extensions @${sni_start}A${sni_len} tls_servername
              }
            }

            incr extension_start $extension_len
          }

          if { [info exists tls_servername] } {
            set servername_lower [string tolower $tls_servername]
            SSL::disable serverside
            if { [class match $servername_lower equals ssl_passthrough_servername_dg] } {
              pool [class match -value $servername_lower equals ssl_passthrough_servername_dg]
              SSL::disable
              HTTP::disable
            }
            elseif { [class match $servername_lower equals ssl_reencrypt_servername_dg] } {
              pool [class match -value $servername_lower equals ssl_reencrypt_servername_dg]
              SSL::enable serverside
            }
          }
        }
      }
    }
  }

  TCP::release
}
`
)

// newF5LTM makes a new f5LTM object.
func newF5LTM(cfg f5LTMCfg) (*f5LTM, error) {
	if cfg.insecure == true {
		glog.Warning("Strict certificate verification is *DISABLED*")
	}

	if cfg.httpVserver == "" {
		glog.Warning("No vserver was specified for HTTP connections;" +
			" HTTP routes will not be configured")
	}

	if cfg.httpsVserver == "" {
		glog.Warning("No vserver was specified for HTTPS connections;" +
			" HTTPS routes will not be configured")
	}

	privkeyFileName := ""

	if cfg.privkey == "" {
		glog.Warning("No SSH key provided for the F5 BIG-IP host;" +
			" TLS configuration for applications is disabled")
	} else {
		// The following is a workaround that will be required until
		// https://github.com/kubernetes/kubernetes/issues/4789 "Fine-grained
		// control of secret data in volume" is resolved.

		// When the oadm command launches the F5 router, oadm copies the SSH private
		// key to a Secret object and mounts a secrets volume containing the Secret
		// object into the F5 router's container.  The ssh and scp commands require
		// that the key file mode prohibit access from any user but the owner.
		// Currently, there is no way to specify the file mode for files in
		// a secrets volume when defining the volume, the default file mode is 0444
		// (too permissive for ssh/scp), and the volume is read-only so it is
		// impossible to change the file mode.  Consequently, we must make a copy of
		// the key and change the file mode of the copy so that the ssh/scp commands
		// will accept it.

		oldPrivkeyFile, err := os.Open(cfg.privkey)
		if err != nil {
			glog.Errorf("Error opening file for F5 BIG-IP private key"+
				" from secrets volume: %v", err)
			return nil, err
		}

		newPrivkeyFile, err := ioutil.TempFile("", "privkey")
		if err != nil {
			glog.Errorf("Error creating tempfile for F5 BIG-IP private key: %v", err)
			return nil, err
		}

		_, err = io.Copy(newPrivkeyFile, oldPrivkeyFile)
		if err != nil {
			glog.Errorf("Error writing private key for F5 BIG-IP to tempfile: %v",
				err)
			return nil, err
		}

		err = oldPrivkeyFile.Close()
		if err != nil {
			// Warn because closing the old file should succeed, but continue because
			// we should be OK if the copy succeeded.
			glog.Warningf("Error closing file for private key for F5 BIG-IP"+
				" from secrets volume: %v", err)
		}

		err = newPrivkeyFile.Close()
		if err != nil {
			glog.Errorf("Error closing tempfile for private key for F5 BIG-IP: %v",
				err)
			return nil, err
		}

		// The chmod should not be necessary because ioutil.TempFile() creates files
		// with restrictive permissions, but we do it anyway to be sure.
		err = os.Chmod(newPrivkeyFile.Name(), 0400)
		if err != nil {
			glog.Warningf("Could not chmod the tempfile for F5 BIG-IP"+
				" private key: %v", err)
		}

		privkeyFileName = newPrivkeyFile.Name()
	}

	partitionPath := F5DefaultPartitionPath
	if len(cfg.partitionPath) > 0 {
		partitionPath = cfg.partitionPath
	}

	// Ensure path is rooted.
	partitionPath = path.Join("/", partitionPath)
	setupOSDNVxLAN := (len(cfg.vxlanGateway) != 0 && len(cfg.internalAddress) != 0)

	router := &f5LTM{
		f5LTMCfg: f5LTMCfg{
			host:            cfg.host,
			username:        cfg.username,
			password:        cfg.password,
			httpVserver:     cfg.httpVserver,
			httpsVserver:    cfg.httpsVserver,
			privkey:         privkeyFileName,
			insecure:        cfg.insecure,
			partitionPath:   partitionPath,
			vxlanGateway:    cfg.vxlanGateway,
			internalAddress: cfg.internalAddress,
			setupOSDNVxLAN:  setupOSDNVxLAN,
		},
		poolMembers: map[string]map[string]bool{},
		routes:      map[string]map[string]bool{},
	}

	return router, nil
}

//
// Helper routines for REST calls.
//

// restRequest makes a REST request to the F5 BIG-IP host's F5 iControl REST
// API.
//
// One of three things can happen as a result of a request to F5 iControl REST:
//
// (1) The request succeeds and F5 returns an HTTP 200 response, possibly with
//     a JSON result payload, which should have the fields defined in the
//     result argument.  In this case, restRequest decodes the payload into
//     the result argument and returns nil.
//
// (2) The request fails and F5 returns an HTTP 4xx or 5xx response with a
//     response payload.  Usually, this payload is JSON containing a numeric
//     code (which should be the same as the HTTP response code) and a string
//     message.  However, in some cases, the F5 iControl REST API returns an
//     HTML response payload instead.  restRequest attempts to decode the
//     response payload as JSON but ignores decoding failures on the assumption
//     that a failure to decode means that the response was in HTML.  Finally,
//     restRequest returns an F5Error with the URL, HTTP verb, HTTP status
//     code, and (if the response was JSON) error information from the response
//     payload.
//
// (3) The REST call fails in some other way, such as a socket error or an
//     error decoding the result payload.  In this case, restRequest returns
//     an F5Error with the URL, HTTP verb, HTTP status code (if any), and error
//     value.
func (f5 *f5LTM) restRequest(verb string, url string, payload io.Reader,
	result interface{}) error {
	tr := knet.SetTransportDefaults(&http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: f5.insecure},
	})

	errorResult := F5Error{verb: verb, url: url}

	req, err := http.NewRequest(verb, url, payload)
	if err != nil {
		errorResult.err = fmt.Errorf("http.NewRequest failed: %v", err)
		return errorResult
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(f5.username, f5.password)

	client := &http.Client{Transport: tr}

	glog.V(4).Infof("Request sent: %v\n", req)
	resp, err := client.Do(req)
	if err != nil {
		errorResult.err = fmt.Errorf("client.Do failed: %v", err)
		return errorResult
	}

	defer resp.Body.Close()

	errorResult.httpStatusCode = resp.StatusCode

	decoder := json.NewDecoder(resp.Body)
	if resp.StatusCode >= 400 {
		// F5 sometimes returns an HTML response even though we ask for JSON.
		// If decoding fails, assume we got an HTML response and ignore both
		// the response and the error.
		decoder.Decode(&errorResult)
		return errorResult
	} else if result != nil {
		err = decoder.Decode(result)
		if err != nil {
			errorResult.err = fmt.Errorf("Decoder.Decode failed: %v", err)
			return errorResult
		}
	}

	return nil
}

// restRequestPayload is a helper for F5 operations that take
// a payload.
func (f5 *f5LTM) restRequestPayload(verb string, url string,
	payload interface{}, result interface{}) error {
	jsonStr, err := json.Marshal(payload)
	if err != nil {
		return F5Error{verb: verb, url: url, err: err}
	}

	encodedPayload := bytes.NewBuffer(jsonStr)

	return f5.restRequest(verb, url, encodedPayload, result)
}

// get issues a GET request against the F5 iControl REST API.
func (f5 *f5LTM) get(url string, result interface{}) error {
	return f5.restRequest("GET", url, nil, result)
}

// post issues a POST request against the F5 iControl REST API.
func (f5 *f5LTM) post(url string, payload interface{}, result interface{}) error {
	return f5.restRequestPayload("POST", url, payload, result)
}

// patch issues a PATCH request against the F5 iControl REST API.
func (f5 *f5LTM) patch(url string, payload interface{}, result interface{}) error {
	return f5.restRequestPayload("PATCH", url, payload, result)
}

// delete issues a DELETE request against the F5 iControl REST API.
func (f5 *f5LTM) delete(url string, result interface{}) error {
	return f5.restRequest("DELETE", url, nil, result)
}

//
// iControl REST resource helper methods.
//

// encodeiControlUriPathComponent returns an encoded resource path for use
// in the URI for the iControl REST calls.
// For example for a path /Common/foo, the corresponding encoded iControl
// URI path component would be ~Common~foo and this can then be used in the
// iControl REST calls ala:
//    https://<ip>:<port>/mgmt/tm/ltm/policy/~Common~foo/rules
func encodeiControlUriPathComponent(pathName string) string {
	return strings.Replace(pathName, "/", "~", -1)
}

// iControlUriResourceId returns an encoded resource id (resource path
// including the partition), which can be used the iControl REST calls.
// For example, for a policy named openshift_secure_routes policy in the
// /Common partition, the encoded resource id would be:
//    ~Common~openshift_secure_routes
// which can then be used as a resource specifier in the URI ala:
//    https://<ip>:<port>/mgmt/tm/ltm/policy/~Common~openshift_secure_routes/rules
func (f5 *f5LTM) iControlUriResourceId(resourceName string) string {
	resourcePath := path.Join(f5.partitionPath, resourceName)
	return encodeiControlUriPathComponent(resourcePath)
}

//
// Routines for controlling F5.
//

// ensureVxLANTunnel sets up the VxLAN tunnel profile and tunnel+selfIP
func (f5 *f5LTM) ensureVxLANTunnel() error {
	glog.V(4).Infof("Checking and installing VxLAN setup")

	// create the profile
	url := fmt.Sprintf("https://%s/mgmt/tm/net/tunnels/vxlan", f5.host)
	profilePayload := f5CreateVxLANProfilePayload{
		Name:         F5VxLANProfileName,
		Partition:    f5.partitionPath,
		FloodingType: "multipoint",
		Port:         4789,
	}
	err := f5.post(url, profilePayload, nil)
	if err != nil && err.(F5Error).httpStatusCode != HTTP_CONFLICT_CODE {
		// error HTTP_CONFLICT_CODE is fine, it just means the tunnel profile already exists
		glog.V(4).Infof("Error while creating vxlan tunnel - %v", err)
		return err
	}

	// create the tunnel
	url = fmt.Sprintf("https://%s/mgmt/tm/net/tunnels/tunnel", f5.host)
	tunnelPayload := f5CreateVxLANTunnelPayload{
		Name:         F5VxLANTunnelName,
		Partition:    f5.partitionPath,
		Key:          0,
		LocalAddress: f5.internalAddress,
		Mode:         "bidirectional",
		Mtu:          "0",
		Profile:      path.Join(f5.partitionPath, F5VxLANProfileName),
		Tos:          "preserve",
		Transparent:  "disabled",
		UsePmtu:      "enabled",
	}
	err = f5.post(url, tunnelPayload, nil)
	if err != nil && err.(F5Error).httpStatusCode != HTTP_CONFLICT_CODE {
		// error HTTP_CONFLICT_CODE is fine, it just means the tunnel already exists
		return err
	}

	selfUrl := fmt.Sprintf("https://%s/mgmt/tm/net/self", f5.host)
	netSelfPayload := f5CreateNetSelfPayload{
		Name:                  f5.vxlanGateway,
		Partition:             f5.partitionPath,
		Address:               f5.vxlanGateway,
		AddressSource:         "from-user",
		Floating:              "disabled",
		InheritedTrafficGroup: "false",
		TrafficGroup:          path.Join("/Common", "traffic-group-local-only"), // Traffic group is global
		Unit:                  0,
		Vlan:                  path.Join(f5.partitionPath, F5VxLANTunnelName),
		AllowService:          "all",
	}
	// create the net self IP
	err = f5.post(selfUrl, netSelfPayload, nil)
	if err != nil && err.(F5Error).httpStatusCode != HTTP_CONFLICT_CODE {
		// error HTTP_CONFLICT_CODE is ok, netSelf already exists
		return err
	}

	return nil
}

// ensurePolicyExists checks whether the specified policy exists and creates it
// if not.
func (f5 *f5LTM) ensurePolicyExists(policyName string) error {
	glog.V(4).Infof("Checking whether policy %s exists...", policyName)

	policyResourceId := f5.iControlUriResourceId(policyName)
	policyUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/policy/%s",
		f5.host, policyResourceId)

	err := f5.get(policyUrl, nil)
	if err != nil && err.(F5Error).httpStatusCode != 404 {
		// 404 is expected, but anything else really is an error.
		return err
	}

	if err == nil {
		glog.V(4).Infof("Policy %s already exists; nothing to do.", policyName)
		return nil
	}

	glog.V(4).Infof("Policy %s does not exist; creating it now...", policyName)

	policiesUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/policy", f5.host)

	policyPath := path.Join(f5.partitionPath, policyName)

	if f5.setupOSDNVxLAN {
		// if vxlan needs to be setup, it will only happen
		// with ver12, for which we need to use a different payload
		policyPayload := f5Ver12Policy{
			Name:        policyPath,
			TmPartition: f5.partitionPath,
			Controls:    []string{"forwarding"},
			Requires:    []string{"http"},
			Strategy:    "best-match",
			Legacy:      true,
		}
		err = f5.post(policiesUrl, policyPayload, nil)
	} else {
		policyPayload := f5Policy{
			Name:      policyPath,
			Partition: f5.partitionPath,
			Controls:  []string{"forwarding"},
			Requires:  []string{"http"},
			Strategy:  "best-match",
		}
		err = f5.post(policiesUrl, policyPayload, nil)
	}

	if err != nil {
		return err
	}

	// We need a rule in the policy in order to be able to add the policy to the
	// vservers, so create a no-op rule now.

	glog.V(4).Infof("Policy %s created.  Adding no-op rule...", policyName)

	rulesUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/policy/%s/rules",
		f5.host, policyResourceId)

	rulesPayload := f5Rule{
		Name: "default_noop",
	}

	err = f5.post(rulesUrl, rulesPayload, nil)
	if err != nil {
		return err
	}

	glog.V(4).Infof("No-op rule added to policy %s.", policyName)

	return nil
}

// ensureVserverHasPolicy checks whether the specified policy is associated with
// the specified vserver and associates the policy with the vserver if not.
func (f5 *f5LTM) ensureVserverHasPolicy(vserverName, policyName string) error {
	glog.V(4).Infof("Checking whether vserver %s has policy %s...",
		vserverName, policyName)

	vserverResourceId := f5.iControlUriResourceId(vserverName)

	// We could use fmt.Sprintf("https://%s/mgmt/tm/ltm/virtual/%s/policies/%s",
	// f5.host, vserverResourceId, policyName) here, except that F5
	// iControl REST returns a 200 even if the policy does not exist.
	vserverPoliciesUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/virtual/%s/policies",
		f5.host, vserverResourceId)

	res := f5VserverPolicies{}

	err := f5.get(vserverPoliciesUrl, &res)
	if err != nil {
		return err
	}

	policyPath := path.Join(f5.partitionPath, policyName)

	for _, policy := range res.Policies {
		if policy.FullPath == policyPath {
			glog.V(4).Infof("Vserver %s has policy %s associated with it;"+
				" nothing to do.", vserverName, policyName)
			return nil
		}
	}

	glog.V(4).Infof("Adding policy %s to vserver %s...", policyName, vserverName)

	vserverPoliciesPayload := f5VserverPolicy{
		Name:      policyPath,
		Partition: f5.partitionPath,
	}

	err = f5.post(vserverPoliciesUrl, vserverPoliciesPayload, nil)
	if err != nil {
		return err
	}

	glog.V(4).Infof("Policy %s added to vserver %s.", policyName, vserverName)

	return nil
}

// ensureDatagroupExists checks whether the specified data-group exists and
// creates it if not.
//
// Note that ensureDatagroupExists assumes that the value-type of the data-group
// should be "string".
func (f5 *f5LTM) ensureDatagroupExists(datagroupName string) error {
	glog.V(4).Infof("Checking whether datagroup %s exists...", datagroupName)

	datagroupUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/data-group/internal/%s",
		f5.host, datagroupName)

	err := f5.get(datagroupUrl, nil)
	if err != nil && err.(F5Error).httpStatusCode != 404 {
		// 404 is expected, but anything else really is an error.
		return err
	}

	if err == nil {
		glog.V(4).Infof("Datagroup %s exists; nothing to do.",
			datagroupName)
		return nil
	}

	glog.V(4).Infof("Creating datagroup %s...", datagroupName)

	datagroupsUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/data-group/internal",
		f5.host)

	datagroupPayload := f5Datagroup{
		Name: datagroupName,
		Type: "string",
	}

	err = f5.post(datagroupsUrl, datagroupPayload, nil)
	if err != nil {
		return err
	}

	glog.V(4).Infof("Datagroup %s created.", datagroupName)

	return nil
}

// ensureIRuleExists checks whether an iRule with the specified name exists and
// creates an iRule with that name and the given code if not.
func (f5 *f5LTM) ensureIRuleExists(iRuleName, iRule string) error {
	glog.V(4).Infof("Checking whether iRule %s exists...", iRuleName)

	iRuleUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/rule/%s", f5.host,
		f5.iControlUriResourceId(iRuleName))

	err := f5.get(iRuleUrl, nil)
	if err != nil && err.(F5Error).httpStatusCode != 404 {
		// 404 is expected, but anything else really is an error.
		return err
	}

	if err == nil {
		glog.V(4).Infof("iRule %s already exists; nothing to do.", iRuleName)
		return nil
	}

	glog.V(4).Infof("IRule %s does not exist; creating it now...", iRuleName)

	iRulesUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/rule", f5.host)

	iRulePayload := f5IRule{
		Name:      iRuleName,
		Partition: f5.partitionPath,
		Code:      iRule,
	}

	err = f5.post(iRulesUrl, iRulePayload, nil)
	if err != nil {
		return err
	}

	glog.V(4).Infof("IRule %s created.", iRuleName)

	return nil
}

// ensureVserverHasIRule checks whether the specified iRule is associated with
// the specified vserver and associates the iRule with the vserver if not.
func (f5 *f5LTM) ensureVserverHasIRule(vserverName, iRuleName string) error {
	glog.V(4).Infof("Checking whether vserver %s has iRule %s...",
		vserverName, iRuleName)

	vserverUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/virtual/%s",
		f5.host, f5.iControlUriResourceId(vserverName))

	res := f5VserverIRules{}

	err := f5.get(vserverUrl, &res)
	if err != nil {
		return err
	}

	commonIRuleName := path.Join("/", f5.partitionPath, iRuleName)

	for _, name := range res.Rules {
		if name == commonIRuleName {
			glog.V(4).Infof("Vserver %s has iRule %s associated with it;"+
				" nothing to do.",
				vserverName, iRuleName)
			return nil
		}
	}

	glog.V(4).Infof("Adding iRule %s to vserver %s...", iRuleName, vserverName)

	sslPassthroughIRulePath := path.Join(f5.partitionPath, sslPassthroughIRuleName)
	vserverRulesPayload := f5VserverIRules{
		Rules: []string{sslPassthroughIRulePath},
	}

	err = f5.patch(vserverUrl, vserverRulesPayload, nil)
	if err != nil {
		return err
	}

	glog.V(4).Infof("IRule %s added to vserver %s.", iRuleName, vserverName)

	return nil
}

// checkPartitionPathExists checks if the partition path exists.
func (f5 *f5LTM) checkPartitionPathExists(pathName string) (bool, error) {
	glog.V(4).Infof("Checking if partition path %q exists...", pathName)

	uri := fmt.Sprintf("https://%s/mgmt/tm/sys/folder/%s",
		f5.host, encodeiControlUriPathComponent(pathName))

	err := f5.get(uri, nil)
	if err != nil {
		if err.(F5Error).httpStatusCode != 404 {
			glog.Errorf("partition path %q error: %v", pathName, err)
			return false, err
		}

		//  404 is ok means that the path doesn't exist == !err.
		return false, nil
	}

	glog.V(4).Infof("Partition path %q exists.", pathName)
	return true, nil
}

// addPartitionPath adds a new partition path to the folder hierarchy.
func (f5 *f5LTM) addPartitionPath(pathName string) (bool, error) {
	glog.V(4).Infof("Creating partition path %q ...", pathName)

	uri := fmt.Sprintf("https://%s/mgmt/tm/sys/folder", f5.host)

	payload := f5AddPartitionPathPayload{Name: pathName}
	err := f5.post(uri, payload, nil)
	if err != nil {
		if err.(F5Error).httpStatusCode != HTTP_CONFLICT_CODE {
			glog.Errorf("Error adding partition path %q error: %v", pathName, err)
			return false, err
		}

		// If the path already exists, don't return an error.
		glog.Warningf("Partition path %q not added as it already exists.", pathName)
		return false, nil
	}

	return true, nil
}

// ensurePartitionPathExists checks whether the specified partition path
// hierarchy exists and creates it if it does not.
func (f5 *f5LTM) ensurePartitionPathExists(pathName string) error {
	glog.V(4).Infof("Ensuring partition path %s exists...", pathName)

	exists, err := f5.checkPartitionPathExists(pathName)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	// We have to loop through the path hierarchy and add components
	// individually if they don't exist.

	// Get path components - we need to remove the leading empty path
	// component after splitting (make it absolute if it is not already
	// and skip the first element).
	// As an example, for a path named "/a/b/c", strings.Split returns
	//   []string{"", "a", "b", "c"}
	// and we skip the empty string.
	p := "/"
	pathComponents := strings.Split(path.Join("/", pathName)[1:], "/")
	for _, v := range pathComponents {
		p = path.Join(p, v)

		exists, err := f5.checkPartitionPathExists(p)
		if err != nil {
			return err
		}
		if !exists {
			if _, err := f5.addPartitionPath(p); err != nil {
				return err
			}
		}
	}

	glog.V(4).Infof("Partition path %s added.", pathName)

	return nil
}

// Initialize ensures that OpenShift-specific configuration is in place on the
// F5 BIG-IP host.  In particular, Initialize creates policies for HTTP and
// HTTPS traffic, as well as an iRule and data-groups for passthrough routes,
// and associates these objects with the appropriate vservers, if necessary.
func (f5 *f5LTM) Initialize() error {
	err := f5.ensurePartitionPathExists(f5.partitionPath)
	if err != nil {
		return err
	}

	err = f5.ensurePolicyExists(httpPolicyName)
	if err != nil {
		return err
	}

	if f5.httpVserver != "" {
		err = f5.ensureVserverHasPolicy(f5.httpVserver, httpPolicyName)
		if err != nil {
			return err
		}
	}

	err = f5.ensurePolicyExists(httpsPolicyName)
	if err != nil {
		return err
	}

	err = f5.ensureDatagroupExists(reencryptRoutesDataGroupName)
	if err != nil {
		return err
	}

	err = f5.ensureDatagroupExists(reencryptHostsDataGroupName)
	if err != nil {
		return err
	}

	err = f5.ensureDatagroupExists(passthroughRoutesDataGroupName)
	if err != nil {
		return err
	}

	err = f5.ensureDatagroupExists(passthroughHostsDataGroupName)
	if err != nil {
		return err
	}

	if f5.httpsVserver != "" {
		err = f5.ensureVserverHasPolicy(f5.httpsVserver, httpsPolicyName)
		if err != nil {
			return err
		}

		err = f5.ensureIRuleExists(sslPassthroughIRuleName, sslPassthroughIRule)
		if err != nil {
			return err
		}

		err = f5.ensureVserverHasIRule(f5.httpsVserver, sslPassthroughIRuleName)
		if err != nil {
			return err
		}
	}

	if f5.setupOSDNVxLAN {
		err = f5.ensureVxLANTunnel()
		if err != nil {
			return err
		}
	}

	glog.V(4).Infof("F5 initialization is complete.")

	return nil
}

func checkIPAndGetMac(ipStr string) (string, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		errStr := fmt.Sprintf("vtep IP '%s' is not a valid IP address", ipStr)
		glog.Warning(errStr)
		return "", fmt.Errorf(errStr)
	}
	ip4 := ip.To4()
	if ip4 == nil {
		errStr := fmt.Sprintf("vtep IP '%s' is not a valid IPv4 address", ipStr)
		glog.Warning(errStr)
		return "", fmt.Errorf(errStr)
	}
	macAddr := fmt.Sprintf("0a:0a:%02x:%02x:%02x:%02x", ip4[0], ip4[1], ip4[2], ip4[3])
	return macAddr, nil
}

// AddVtep adds the Vtep IP to the VxLAN device's FDB
func (f5 *f5LTM) AddVtep(ipStr string) error {
	if !f5.setupOSDNVxLAN {
		return nil
	}
	macAddr, err := checkIPAndGetMac(ipStr)
	if err != nil {
		return err
	}

	err = f5.ensurePartitionPathExists(f5.partitionPath)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://%s/mgmt/tm/net/fdb/tunnel/%s~%s/records", f5.host, strings.Replace(f5.partitionPath, "/", "~", -1), F5VxLANTunnelName)
	payload := f5AddFDBRecordPayload{
		Name:     macAddr,
		Endpoint: ipStr,
	}
	err = f5.post(url, payload, nil)
	if err != nil && err.(F5Error).httpStatusCode != HTTP_CONFLICT_CODE {
		// error HTTP_CONFLICT_CODE is fine, it just means the fdb entry exists already (and we have a unique key tied to the vtep ip ;)
		return err
	}
	return nil
}

// RemoveVtep removes the Vtep IP from the VxLAN device's FDB
func (f5 *f5LTM) RemoveVtep(ipStr string) error {
	if !f5.setupOSDNVxLAN {
		return nil
	}
	macAddr, err := checkIPAndGetMac(ipStr)
	if err != nil {
		return err
	}

	err = f5.ensurePartitionPathExists(f5.partitionPath)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://%s/mgmt/tm/net/fdb/tunnel/%s~%s/records/%s", f5.host, strings.Replace(f5.partitionPath, "/", "~", -1), F5VxLANTunnelName, macAddr)
	return f5.delete(url, nil)
}

// CreatePool creates a pool named poolname on F5 BIG-IP.
func (f5 *f5LTM) CreatePool(poolname string) error {
	url := fmt.Sprintf("https://%s/mgmt/tm/ltm/pool", f5.host)

	// The http monitor is still used from the /Common partition.
	// From @Miciah: In the future, we should allow the administrator
	// to specify a different monitor to use.
	payload := f5Pool{
		Mode:      "round-robin",
		Monitor:   "min 1 of /Common/http /Common/https",
		Partition: f5.partitionPath,
		Name:      poolname,
	}

	err := f5.post(url, payload, nil)
	if err != nil {
		return err
	}

	// We don't really need to initialise f5.poolMembers[poolname] because
	// we always check whether it is initialised before using it, but
	// initialising it to an empty map here saves a REST call later the first
	// time f5.PoolHasMember is invoked with poolname.
	f5.poolMembers[poolname] = map[string]bool{}

	glog.V(4).Infof("Pool %s created.", poolname)

	return nil
}

// DeletePool deletes the specified pool from F5 BIG-IP, and deletes
// f5.poolMembers[poolname].
func (f5 *f5LTM) DeletePool(poolname string) error {
	url := fmt.Sprintf("https://%s/mgmt/tm/ltm/pool/%s", f5.host,
		f5.iControlUriResourceId(poolname))

	err := f5.delete(url, nil)
	if err != nil {
		return err
	}

	// Note: We *must* use delete here rather than merely assigning false because
	// len() includes false items, and we want len() to return an accurate count
	// of members.  Also, we probably save some memory by using delete.
	delete(f5.poolMembers, poolname)

	glog.V(4).Infof("Pool %s deleted.", poolname)

	return nil
}

// GetPoolMembers returns f5.poolMembers[poolname], first initializing it from
// F5 if it is zero.
func (f5 *f5LTM) GetPoolMembers(poolname string) (map[string]bool, error) {
	members, ok := f5.poolMembers[poolname]
	if ok {
		return members, nil
	}

	url := fmt.Sprintf("https://%s/mgmt/tm/ltm/pool/%s/members",
		f5.host, f5.iControlUriResourceId(poolname))

	res := f5PoolMemberset{}

	err := f5.get(url, &res)
	if err != nil {
		return nil, err
	}

	// Note that we do not initialise f5.poolMembers[poolname] unless we know that
	// the pool exists (i.e., the above GET request for the pool succeeds).
	f5.poolMembers[poolname] = map[string]bool{}

	for _, member := range res.Members {
		f5.poolMembers[poolname][member.Name] = true
	}

	return f5.poolMembers[poolname], nil
}

// PoolExists checks whether the specified pool exists.  Internally, PoolExists
// uses f5.poolMembers[poolname], as a side effect initialising it if it is
// zero.
func (f5 *f5LTM) PoolExists(poolname string) (bool, error) {
	_, err := f5.GetPoolMembers(poolname)
	if err == nil {
		return true, nil
	}

	if err.(F5Error).httpStatusCode == 404 {
		return false, nil
	}

	return false, err
}

// PoolHasMember checks whether the given member is in the specified pool on F5
// BIG-IP.  Internally, PoolHasMember uses f5.poolMembers[poolname], causing it
// to be initialised first if it is zero.
func (f5 *f5LTM) PoolHasMember(poolname, member string) (bool, error) {
	members, err := f5.GetPoolMembers(poolname)
	if err != nil {
		return false, err
	}

	return members[member], nil
}

// AddPoolMember adds the given member to the specified pool on F5 BIG-IP, and
// updates f5.poolMembers[poolname].
func (f5 *f5LTM) AddPoolMember(poolname, member string) error {
	hasMember, err := f5.PoolHasMember(poolname, member)
	if err != nil {
		return err
	}
	if hasMember {
		glog.V(4).Infof("Pool %s already has member %s.\n", poolname, member)
		return nil
	}

	glog.V(4).Infof("Adding pool member %s to pool %s.", member, poolname)

	url := fmt.Sprintf("https://%s/mgmt/tm/ltm/pool/%s/members",
		f5.host, f5.iControlUriResourceId(poolname))

	payload := f5PoolMember{
		Name: member,
	}

	err = f5.post(url, payload, nil)
	if err != nil {
		return err
	}

	members, err := f5.GetPoolMembers(poolname)
	if err != nil {
		return err
	}

	members[member] = true

	glog.V(4).Infof("Added pool member %s to pool %s.",
		member, poolname)

	return nil
}

// DeletePoolMember deletes the given member from the specified pool on F5
// BIG-IP, and updates f5.poolMembers[poolname].
func (f5 *f5LTM) DeletePoolMember(poolname, member string) error {
	// The invocation of f5.PoolHasMember has the side effect that it will
	// initialise f5.poolMembers[poolname], which is used below, if necessary.
	hasMember, err := f5.PoolHasMember(poolname, member)
	if err != nil {
		return err
	}
	if !hasMember {
		glog.V(4).Infof("Pool %s does not have member %s.\n", poolname, member)
		return nil
	}

	url := fmt.Sprintf("https://%s/mgmt/tm/ltm/pool/%s/members/%s",
		f5.host, f5.iControlUriResourceId(poolname), member)

	err = f5.delete(url, nil)
	if err != nil {
		return err
	}

	delete(f5.poolMembers[poolname], member)

	glog.V(4).Infof("Pool member %s deleted from pool %s.", member, poolname)

	return nil
}

// getRoutes returns f5.routes[policyname], first initializing it from F5 if it
// is zero.
func (f5 *f5LTM) getRoutes(policyname string) (map[string]bool, error) {
	routes, ok := f5.routes[policyname]
	if ok {
		return routes, nil
	}

	url := fmt.Sprintf("https://%s/mgmt/tm/ltm/policy/%s/rules",
		f5.host, f5.iControlUriResourceId(policyname))

	res := f5PolicyRuleset{}

	err := f5.get(url, &res)
	if err != nil {
		return nil, err
	}

	routes = map[string]bool{}

	for _, rule := range res.Rules {
		routes[rule.Name] = true
	}

	f5.routes[policyname] = routes

	return routes, nil
}

// routeExists checks whether the an F5 profile rule exists for the specified
// route.  Note that routeExists assumes that the route name will be the same
// as the rule name.
func (f5 *f5LTM) routeExists(policyname, routename string) (bool, error) {
	routes, err := f5.getRoutes(policyname)
	if err != nil {
		return false, err
	}

	return routes[routename], nil
}

// InsecureRouteExists checks whether the specified insecure route exists.
func (f5 *f5LTM) InsecureRouteExists(routename string) (bool, error) {
	return f5.routeExists(httpPolicyName, routename)
}

// SecureRouteExists checks whether the specified secure route exists.
func (f5 *f5LTM) SecureRouteExists(routename string) (bool, error) {
	return f5.routeExists(httpsPolicyName, routename)
}

// ReencryptRouteExists checks whether the specified reencrypt route exists.
func (f5 *f5LTM) ReencryptRouteExists(routename string) (bool, error) {
	routes, err := f5.getReencryptRoutes()
	if err != nil {
		return false, err
	}

	_, ok := routes[routename]

	return ok, nil
}

// PassthroughRouteExists checks whether the specified passthrough route exists.
func (f5 *f5LTM) PassthroughRouteExists(routename string) (bool, error) {
	routes, err := f5.getPassthroughRoutes()
	if err != nil {
		return false, err
	}

	_, ok := routes[routename]

	return ok, nil
}

// addRoute adds a new rule to the specified F5 policy.  This rule will compare
// the virtual host and URL path of incoming requests against the given hostname
// and pathname (if one is specified).  When the rule matches a request, it will
// route the request to the specified pool.
//
// addRoute re-uses the name of the OpenShift route as the name of the F5
// policy rule.  The rule name must be safe to use in JSON and in URLs (for
// example, slashes or backslashes would cause problems), but this condition
// is met when using the route name because it has the form
// openshift_<namespace>_<servicename>, a namespace must match the regex
// /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/, and service name must match the regex
// /^[a-z]([-a-z0-9]+)?$/.
func (f5 *f5LTM) addRoute(policyname, routename, poolname, hostname,
	pathname string) error {
	success := false

	policyResourceId := f5.iControlUriResourceId(policyname)
	rulesUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/policy/%s/rules",
		f5.host, policyResourceId)

	rulesPayload := f5Rule{
		Name: routename,
	}

	err := f5.post(rulesUrl, rulesPayload, nil)
	if err != nil {
		if err.(F5Error).httpStatusCode == HTTP_CONFLICT_CODE {
			glog.V(4).Infof("Warning: Rule %s already exists; continuing with"+
				" initialization in case the existing rule is only partially"+
				" initialized...", routename)
		} else {
			return err
		}
	}

	// If adding the condition or action to the rule fails later on,
	// delete the rule.
	defer func() {
		if success != true {
			err := f5.deleteRoute(policyname, routename)
			if err != nil && err.(F5Error).httpStatusCode != 404 {
				glog.V(4).Infof("Warning: Creating rule %s failed,"+
					" and then cleanup got an error: %v", routename, err)
			}
		}
	}()

	conditionUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/policy/%s/rules/%s/conditions",
		f5.host, policyResourceId, routename)

	conditionPayload := f5RuleCondition{
		Name:            "0",
		CaseInsensitive: true,
		HttpHost:        true,
		Host:            true,
		Index:           0,
		Equals:          true,
		Request:         true,
		Values:          []string{hostname},
	}

	err = f5.post(conditionUrl, conditionPayload, nil)
	if err != nil {
		return err
	}

	if pathname != "" {
		// Each segment of the pathname must be added to the rule as a separate
		// condition.
		segments := strings.Split(pathname, "/")
		conditionPayload.HttpHost = false
		conditionPayload.Host = false
		conditionPayload.HttpUri = true
		conditionPayload.PathSegment = true
		for i, segment := range segments[1:] {
			if segment == "" {
				continue
			}
			idx := fmt.Sprintf("%d", i+1)
			conditionPayload.Name = idx
			conditionPayload.Index = i + 1
			conditionPayload.Values = []string{segment}
			err = f5.post(conditionUrl, conditionPayload, nil)
			if err != nil {
				return err
			}
		}
	}

	actionUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/policy/%s/rules/%s/actions",
		f5.host, policyResourceId, routename)

	actionPayload := f5RuleAction{
		Name:    "0",
		Forward: true,
		Pool:    fmt.Sprintf("%s/%s", f5.partitionPath, poolname),
		Request: true,
		Select:  true,
		Vlan:    0,
	}

	err = f5.post(actionUrl, actionPayload, nil)
	if err != nil {
		return err
	}

	success = true

	routes, err := f5.getRoutes(policyname)
	if err != nil {
		return err
	}

	routes[routename] = true

	return nil
}

// AddSecureRoute adds an F5 profile rule for the specified insecure route to F5
// BIG-IP, so that requests to the specified hostname and pathname will be
// routed to the specified pool.
func (f5 *f5LTM) AddInsecureRoute(routename, poolname, hostname,
	pathname string) error {
	return f5.addRoute(httpPolicyName, routename, poolname, hostname, pathname)
}

// AddSecureRoute adds an F5 profile rule for the specified secure route to F5
// BIG-IP, so that requests to the specified hostname and pathname will be
// routed to the specified pool.
func (f5 *f5LTM) AddSecureRoute(routename, poolname, hostname,
	pathname string) error {
	return f5.addRoute(httpsPolicyName, routename, poolname, hostname, pathname)
}

// getReencryptRoutes returns f5.reencryptRoutes, first initializing it from
// F5 if it is zero.
func (f5 *f5LTM) getReencryptRoutes() (map[string]reencryptRoute, error) {
	routes := f5.reencryptRoutes
	if routes != nil {
		return routes, nil
	}

	hostsUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/data-group/internal/%s",
		f5.host, reencryptHostsDataGroupName)

	hostsRes := f5Datagroup{}

	err := f5.get(hostsUrl, &hostsRes)
	if err != nil {
		return nil, err
	}

	routesUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/data-group/internal/%s",
		f5.host, reencryptRoutesDataGroupName)

	routesRes := f5Datagroup{}

	err = f5.get(routesUrl, &routesRes)
	if err != nil {
		return nil, err
	}

	hosts := map[string]string{}

	for _, hostRecord := range hostsRes.Records {
		hosts[hostRecord.Key] = hostRecord.Value
	}

	f5.reencryptRoutes = map[string]reencryptRoute{}

	for _, routeRecord := range routesRes.Records {
		routename := routeRecord.Key
		hostname := routeRecord.Value

		poolname, foundPoolname := hosts[hostname]
		if !foundPoolname {
			glog.Warningf("%s datagroup maps route %s to hostname %s,"+
				" but %s datagroup does not have an entry for that hostname"+
				" to map it to a pool.  Dropping route %s from datagroup %s...",
				reencryptRoutesDataGroupName, routename, hostname,
				reencryptHostsDataGroupName,
				routename, reencryptRoutesDataGroupName)
			continue
		}

		f5.reencryptRoutes[routename] = reencryptRoute{
			hostname: hostname,
			poolname: poolname,
		}
	}

	return f5.reencryptRoutes, nil
}

// updateReencryptRoutes updates the data-groups for reencrypt routes using
// the internal object's state.
func (f5 *f5LTM) updateReencryptRoutes() error {
	routes, err := f5.getReencryptRoutes()
	if err != nil {
		return err
	}

	// It would be *super* great if we could use CRUD operations on data-groups as
	// we do on pools, rules, and profiles, but we cannot: each data-group is
	// represented in JSON as an array, so we must PATCH the array in its
	// entirety.

	hostsRecords := []f5DatagroupRecord{}
	routesRecords := []f5DatagroupRecord{}
	for routename, route := range routes {
		hostsRecords = append(hostsRecords,
			f5DatagroupRecord{Key: route.hostname, Value: route.poolname})
		routesRecords = append(routesRecords,
			f5DatagroupRecord{Key: routename, Value: route.hostname})
	}

	hostsDatagroupUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/data-group/internal/%s",
		f5.host, reencryptHostsDataGroupName)

	hostsDatagroupPayload := f5Datagroup{
		Records: hostsRecords,
	}

	err = f5.patch(hostsDatagroupUrl, hostsDatagroupPayload, nil)
	if err != nil {
		return err
	}

	glog.V(4).Infof("Datagroup %s updated.", reencryptHostsDataGroupName)

	routesDatagroupUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/data-group/internal/%s",
		f5.host, reencryptRoutesDataGroupName)

	routesDatagroupPayload := f5Datagroup{
		Records: routesRecords,
	}

	err = f5.patch(routesDatagroupUrl, routesDatagroupPayload, nil)
	if err != nil {
		return err
	}

	glog.V(4).Infof("Datagroup %s updated.", reencryptRoutesDataGroupName)

	return nil
}

// getPassthroughRoutes returns f5.passthroughRoutes, first initializing it from
// F5 if it is zero.
func (f5 *f5LTM) getPassthroughRoutes() (map[string]passthroughRoute, error) {
	routes := f5.passthroughRoutes
	if routes != nil {
		return routes, nil
	}

	hostsUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/data-group/internal/%s",
		f5.host, passthroughHostsDataGroupName)

	hostsRes := f5Datagroup{}

	err := f5.get(hostsUrl, &hostsRes)
	if err != nil {
		return nil, err
	}

	routesUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/data-group/internal/%s",
		f5.host, passthroughRoutesDataGroupName)

	routesRes := f5Datagroup{}

	err = f5.get(routesUrl, &routesRes)
	if err != nil {
		return nil, err
	}

	hosts := map[string]string{}

	for _, hostRecord := range hostsRes.Records {
		hosts[hostRecord.Key] = hostRecord.Value
	}

	f5.passthroughRoutes = map[string]passthroughRoute{}

	for _, routeRecord := range routesRes.Records {
		routename := routeRecord.Key
		hostname := routeRecord.Value

		poolname, foundPoolname := hosts[hostname]
		if !foundPoolname {
			glog.Warningf("%s datagroup maps route %s to hostname %s,"+
				" but %s datagroup does not have an entry for that hostname"+
				" to map it to a pool.  Dropping route %s from datagroup %s...",
				passthroughRoutesDataGroupName, routename, hostname,
				passthroughHostsDataGroupName,
				routename, passthroughRoutesDataGroupName)
			continue
		}

		f5.passthroughRoutes[routename] = passthroughRoute{
			hostname: hostname,
			poolname: poolname,
		}
	}

	return f5.passthroughRoutes, nil
}

// updatePassthroughRoutes updates the data-groups for passthrough routes using
// the internal object's state.
func (f5 *f5LTM) updatePassthroughRoutes() error {
	routes, err := f5.getPassthroughRoutes()
	if err != nil {
		return err
	}

	// It would be *super* great if we could use CRUD operations on data-groups as
	// we do on pools, rules, and profiles, but we cannot: each data-group is
	// represented in JSON as an array, so we must PATCH the array in its
	// entirety.

	hostsRecords := []f5DatagroupRecord{}
	routesRecords := []f5DatagroupRecord{}
	for routename, route := range routes {
		hostsRecords = append(hostsRecords,
			f5DatagroupRecord{Key: route.hostname, Value: route.poolname})
		routesRecords = append(routesRecords,
			f5DatagroupRecord{Key: routename, Value: route.hostname})
	}

	hostsDatagroupUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/data-group/internal/%s",
		f5.host, passthroughHostsDataGroupName)

	hostsDatagroupPayload := f5Datagroup{
		Records: hostsRecords,
	}

	err = f5.patch(hostsDatagroupUrl, hostsDatagroupPayload, nil)
	if err != nil {
		return err
	}

	glog.V(4).Infof("Datagroup %s updated.", passthroughHostsDataGroupName)

	routesDatagroupUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/data-group/internal/%s",
		f5.host, passthroughRoutesDataGroupName)

	routesDatagroupPayload := f5Datagroup{
		Records: routesRecords,
	}

	err = f5.patch(routesDatagroupUrl, routesDatagroupPayload, nil)
	if err != nil {
		return err
	}

	glog.V(4).Infof("Datagroup %s updated.", passthroughRoutesDataGroupName)

	return nil
}

// AddReencryptRoute adds the required data-group records for the specified
// reeencrypt route to F5 BIG-IP, so that requests to the specified hostname
// will be routed to the specified pool through the iRule
func (f5 *f5LTM) AddReencryptRoute(routename, poolname, hostname string) error {
	routes, err := f5.getReencryptRoutes()
	if err != nil {
		return err
	}

	routes[routename] = reencryptRoute{hostname: hostname, poolname: poolname}

	return f5.updateReencryptRoutes()
}

// AddPassthroughRoute adds the required data-group records for the specified
// passthrough route to F5 BIG-IP, so that requests to the specified hostname
// will be routed to the specified pool.
func (f5 *f5LTM) AddPassthroughRoute(routename, poolname, hostname string) error {
	routes, err := f5.getPassthroughRoutes()
	if err != nil {
		return err
	}

	routes[routename] = passthroughRoute{hostname: hostname, poolname: poolname}

	return f5.updatePassthroughRoutes()
}

// DeleteReencryptRoute deletes the data-group records for the specified
// reencrypt route from F5 BIG-IP.
func (f5 *f5LTM) DeleteReencryptRoute(routename string) error {
	routes, err := f5.getReencryptRoutes()
	if err != nil {
		return err
	}

	_, exists := routes[routename]
	if !exists {
		return fmt.Errorf("Reencrypt route %s does not exist.", routename)
	}

	delete(routes, routename)

	return f5.updateReencryptRoutes()
}

// DeletePassthroughRoute deletes the data-group records for the specified
// passthrough route from F5 BIG-IP.
func (f5 *f5LTM) DeletePassthroughRoute(routename string) error {
	routes, err := f5.getPassthroughRoutes()
	if err != nil {
		return err
	}

	_, exists := routes[routename]
	if !exists {
		return fmt.Errorf("Passthrough route %s does not exist.", routename)
	}

	delete(routes, routename)

	return f5.updatePassthroughRoutes()
}

// deleteRoute deletes the F5 policy rule for the given routename from the given
// policy.
func (f5 *f5LTM) deleteRoute(policyname, routename string) error {
	ruleUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/policy/%s/rules/%s",
		f5.host, f5.iControlUriResourceId(policyname), routename)

	err := f5.delete(ruleUrl, nil)
	if err != nil {
		return err
	}

	delete(f5.routes[policyname], routename)

	glog.V(4).Infof("Route %s deleted.", routename)

	return nil
}

// DeleteInsecureRoute deletes the F5 policy rule for the given insecure route.
func (f5 *f5LTM) DeleteInsecureRoute(routename string) error {
	return f5.deleteRoute(httpPolicyName, routename)
}

// DeleteSecureRoute deletes the F5 policy rule for the given secure route.
func (f5 *f5LTM) DeleteSecureRoute(routename string) error {
	return f5.deleteRoute(httpsPolicyName, routename)
}

// sshOptions is an array of flags that we use for all ssh and scp commands to
// F5 BIG-IP.
var sshOptions []string = []string{
	"-o", "StrictHostKeyChecking=no",
	"-o", "GSSAPIAuthentication=no",
	"-o", "PasswordAuthentication=no",
	"-o", "PubkeyAuthentication=yes",
	"-o", "VerifyHostKeyDNS=no",
	"-o", "UserKnownHostsFile=/dev/null",
}

// buildSshArgs constructs the list of arguments for an ssh or scp command to F5
// BIG-IP.
func (f5 *f5LTM) buildSshArgs(args ...string) []string {
	return append(append(sshOptions, "-i", f5.privkey), args...)
}

// AddCert adds the provided TLS certificate and private key to F5 BIG-IP for
// client-side TLS (i.e., encryption between the client and F5 BIG-IP),
// configures a corresponding client-ssl SSL profile, and associates it with the
// HTTPS vserver.  If a destination certificate is provided, AddCert adds that
// for server-side TLS (i.e., encryption between F5 BIG-IP and the pod),
// configures a corresponding server-ssl SSL profile, and associates it too with
// the vserver.
func (f5 *f5LTM) AddCert(routename, hostname, cert, privkey,
	destCACert string) error {
	if f5.privkey == "" {
		return fmt.Errorf("Cannot configure TLS for route %s"+
			" because router was not provided an SSH private key",
			routename)
	}

	var deleteServerSslProfile,
		deleteClientSslProfileFromVserver, deleteClientSslProfile,
		deletePrivateKey, deleteCert, deleteCACert bool

	success := false

	defer func() {
		if success != true {
			f5.deleteCertParts(routename, false, deleteServerSslProfile,
				deleteClientSslProfileFromVserver, deleteClientSslProfile,
				deletePrivateKey, deleteCert, deleteCACert)
		}
	}()

	var err error

	certname := fmt.Sprintf("%s-https-cert", routename)
	err = f5.uploadCert(cert, certname)
	if err != nil {
		return err
	}
	deleteCert = true

	keyname := fmt.Sprintf("%s-https-key", routename)
	err = f5.uploadKey(privkey, keyname)
	if err != nil {
		return err
	}
	deletePrivateKey = true

	clientSslProfileName := fmt.Sprintf("%s-client-ssl-profile", routename)
	err = f5.createClientSslProfile(clientSslProfileName,
		hostname, certname, keyname)
	if err != nil {
		return err
	}
	deleteClientSslProfile = true

	err = f5.associateClientSslProfileWithVserver(clientSslProfileName,
		f5.httpsVserver)
	if err != nil {
		return err
	}
	deleteClientSslProfileFromVserver = true

	if destCACert != "" {
		cacertname := fmt.Sprintf("%s-https-chain", routename)
		err = f5.uploadCert(destCACert, cacertname)
		if err != nil {
			return err
		}
		deleteCACert = true

		serverSslProfileName := fmt.Sprintf("%s-server-ssl-profile", routename)
		err = f5.createServerSslProfile(serverSslProfileName,
			hostname, cacertname)
		if err != nil {
			return err
		}
		deleteServerSslProfile = true

		err = f5.associateServerSslProfileWithVserver(serverSslProfileName,
			f5.httpsVserver)
		if err != nil {
			return err
		}
	}

	success = true

	return nil
}

// execCommand is just an alias for exec.Command.  Using this alias enables test
// code to mock out exec.Command by modifying the variable.
var execCommand = exec.Command

// uploadCert uploads the given certificate to F5 BIG-IP and "installs" it so
// that it can be used in subsequent F5 iControl REST requests.
func (f5 *f5LTM) uploadCert(cert, certname string) error {
	glog.V(4).Infof("Writing tempfile for certificate %s...", certname)

	certfile, err := ioutil.TempFile("", "cert")
	if err != nil {
		glog.Errorf("Error tempfile for certificate %s: %v", certname, err)
		return err
	}
	defer os.Remove(certfile.Name())

	n, err := certfile.WriteString(cert)
	if err != nil {
		glog.Errorf("Error writing tempfile for certificate %s: %v", certname, err)
		return err
	}
	if n != len(cert) {
		glog.Errorf("Wrong number of bytes written to tempfile for certificate %s:"+
			" expected %d bytes, wrote %d", certname, len(cert), n)
		return err
	}
	err = certfile.Close()
	if err != nil {
		glog.Errorf("Error closing tempfile for certificate %s: %v", certname, err)
		return err
	}

	glog.V(4).Infof("Copying tempfile for certificate %s to F5 BIG-IP...",
		certname)
	certfilePath := fmt.Sprintf("/var/tmp/%s.cert", certname)
	sshUserHost := fmt.Sprintf("%s@%s", f5.username, f5.host)
	certfileDest := fmt.Sprintf("%s:%s", sshUserHost, certfilePath)
	args := f5.buildSshArgs(certfile.Name(), certfileDest)
	defer func() {
		glog.V(4).Infof("Cleaning up tempfile for certificate %s on F5 BIG-IP...",
			certname)
		args := f5.buildSshArgs(sshUserHost, "rm", "-f", certfilePath)
		out, err := execCommand("ssh", args...).CombinedOutput()
		if err != nil {
			glog.Errorf("Error deleting tempfile for certificate %s from F5 BIG-IP.\n"+
				"\tOutput from ssh command: %s\n\tError: %v",
				certname, out, err)
		}
	}()
	out, err := execCommand("scp", args...).CombinedOutput()
	if err != nil {
		glog.Errorf("Error copying certificate %s to F5 BIG-IP.\n"+
			"\tOutput from scp command: %s\n\tError: %v",
			certname, out, err)
		return err
	}

	glog.V(4).Infof("Installing certificate %s on F5 BIG-IP...",
		certname)

	installCertCommandUrl := fmt.Sprintf("https://%s/mgmt/tm/sys/crypto/cert",
		f5.host)

	installCertCommandPayload := f5InstallCommandPayload{
		Command:  "install",
		Name:     certname,
		Filename: certfilePath,
	}

	return f5.post(installCertCommandUrl, installCertCommandPayload, nil)
}

// uploadKey uploads the given key to F5 BIG-IP and "installs" it so that it can
// be used in subsequent F5 iControl REST requests.
func (f5 *f5LTM) uploadKey(privkey, keyname string) error {
	glog.V(4).Infof("Writing tempfile for key %s...", keyname)

	keyfile, err := ioutil.TempFile("", "key")
	if err != nil {
		glog.Errorf("Error creating tempfile for key %s: %v", keyname, err)
		return err
	}
	defer os.Remove(keyfile.Name())

	n, err := keyfile.WriteString(privkey)
	if err != nil {
		glog.Errorf("Error writing key %s to tempfile: %v", keyname, err)
		return err
	}
	if n != len(privkey) {
		glog.Errorf("Wrong number of bytes written to tempfile for key %s:"+
			" expected %d bytes, wrote %d", keyname, len(privkey), n)
		return err
	}
	err = keyfile.Close()
	if err != nil {
		glog.Errorf("Error closing tempfile for key %s: %v", keyfile.Name(),
			err)
		return err
	}

	glog.V(4).Infof("Copying tempfile for key %s to F5 BIG-IP...", keyname)

	keyfilePath := fmt.Sprintf("/var/tmp/%s.key", keyname)
	sshUserHost := fmt.Sprintf("%s@%s", f5.username, f5.host)
	keyfileDest := fmt.Sprintf("%s:%s", sshUserHost, keyfilePath)
	args := f5.buildSshArgs(keyfile.Name(), keyfileDest)
	defer func() {
		glog.V(4).Infof("Cleaning up tempfile for key %s on F5 BIG-IP...",
			keyname)
		args := f5.buildSshArgs(sshUserHost, "rm", "-f", keyfilePath)
		out, err := execCommand("ssh", args...).CombinedOutput()
		if err != nil {
			glog.Errorf("Error deleting tempfile for key %ss from F5 BIG-IP.\n"+
				"\tOutput from ssh command: %s\n\tError: %v",
				keyname, out, err)
		}
	}()
	out, err := execCommand("scp", args...).CombinedOutput()
	if err != nil {
		glog.Errorf("Error copying key %s to F5 BIG-IP.\n"+
			"\tOutput from scp command: %s\n\tError: %v",
			keyname, out, err)
		return err
	}

	glog.V(4).Infof("Installing key %s on F5 BIG-IP...", keyname)

	installKeyCommandUrl := fmt.Sprintf("https://%s/mgmt/tm/sys/crypto/key",
		f5.host)

	installKeyCommandPayload := f5InstallCommandPayload{
		Command:  "install",
		Name:     keyname,
		Filename: keyfilePath,
	}

	return f5.post(installKeyCommandUrl, installKeyCommandPayload, nil)
}

// createClientSslProfile creates a client-ssl profile with the given name and
// for the specified hostname, certificate, and key in F5 BIG-IP.
func (f5 *f5LTM) createClientSslProfile(profilename,
	hostname, certname, keyname string) error {
	glog.V(4).Infof("Creating client-ssl profile %s...", profilename)

	clientSslProfileUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/profile/client-ssl",
		f5.host)

	clientSslProfilePayload := f5SslProfilePayload{
		// Although we do not specify extensions when installing the certificate and
		// private key, we *must* specify the extensions when referencing the
		// certificate in *this* request, or else F5 gets confused and returns
		// a misleading error message ("Client SSL profile must have RSA
		// certificate/key pair.").
		Certificate: fmt.Sprintf("%s.crt", certname),
		Key:         fmt.Sprintf("%s.key", keyname),
		Name:        profilename,
		ServerName:  hostname,
	}

	return f5.post(clientSslProfileUrl, clientSslProfilePayload, nil)
}

// createServerSslProfile creates a server-ssl profile with the given name and
// for the specified hostname and CA certificate in F5 BIG-IP.
func (f5 *f5LTM) createServerSslProfile(profilename,
	hostname, cacertname string) error {
	glog.V(4).Infof("Creating server-ssl profile %s...", profilename)

	serverSslProfileUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/profile/server-ssl",
		f5.host)

	serverSslProfilePayload := f5SslProfilePayload{
		// Similar as for createClientSslProfile, we must add an extension when
		// referencing the CA certificate here.
		Chain:      fmt.Sprintf("%s.crt", cacertname),
		Name:       profilename,
		ServerName: hostname,
	}

	return f5.post(serverSslProfileUrl, serverSslProfilePayload, nil)
}

// associateClientSslProfileWithVserver associates the specified client-ssl
// profile with the specified vserver in F5 BIG-IP.
func (f5 *f5LTM) associateClientSslProfileWithVserver(profilename,
	vservername string) error {
	glog.V(4).Infof("Associating client-ssl profile %s with vserver %s...",
		profilename, vservername)

	vserverProfileUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/virtual/%s/profiles",
		f5.host, f5.iControlUriResourceId(vservername))

	vserverProfilePayload := f5VserverProfilePayload{
		Name:    profilename,
		Context: "clientside",
	}

	return f5.post(vserverProfileUrl, vserverProfilePayload, nil)
}

// associateServerSslProfileWithVserver associates the specified server-ssl
// profile with the specified vserver in F5 BIG-IP.
func (f5 *f5LTM) associateServerSslProfileWithVserver(profilename,
	vservername string) error {
	glog.V(4).Infof("Associating server-ssl profile %s with vserver %s...",
		profilename, vservername)

	vserverProfileUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/virtual/%s/profiles",
		f5.host, f5.iControlUriResourceId(vservername))

	vserverProfilePayload := f5VserverProfilePayload{
		Name:    profilename,
		Context: "serverside",
	}

	return f5.post(vserverProfileUrl, vserverProfilePayload, nil)
}

// DeleteCert deletes the TLS certificate and key for the specified route, as
// well as any related client-ssl or server-ssl profile, from F5 BIG-IP.
func (f5 *f5LTM) DeleteCert(routename string) error {
	return f5.deleteCertParts(routename, true, true, true, true, true, true, true)
}

// deleteCertParts deletes the TLS-related configuration items from F5 BIG-IP,
// as specified by the Boolean arguments.
func (f5 *f5LTM) deleteCertParts(routename string,
	deleteServerSslProfileFromVserver, deleteServerSslProfile,
	deleteClientSslProfileFromVserver, deleteClientSslProfile,
	deletePrivateKey, deleteCert, deleteCACert bool) error {

	if deleteServerSslProfileFromVserver {
		glog.V(4).Infof("Deleting server-ssl profile for route %s from vserver %s...",
			routename, f5.httpsVserver)
		serverSslProfileName := fmt.Sprintf("%s-server-ssl-profile", routename)
		serverSslVserverProfileUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/virtual/%s/profiles/%s",
			f5.host, f5.iControlUriResourceId(f5.httpsVserver), serverSslProfileName)
		err := f5.delete(serverSslVserverProfileUrl, nil)
		if err != nil {
			// Iff the profile is not associated with the vserver, we can continue on to
			// delete the vserver.
			if err.(F5Error).httpStatusCode != 404 {
				glog.V(4).Infof("Error deleting server-ssl profile for route %s"+
					" from vserver %s: %v", routename, f5.httpsVserver, err)
				return err
			}
		}
	}

	if deleteServerSslProfile {
		glog.V(4).Infof("Deleting server-ssl profile for route %s...", routename)
		serverSslProfileName := fmt.Sprintf("%s-server-ssl-profile", routename)
		serverSslProfileUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/profile/server-ssl/%s",
			f5.host, serverSslProfileName)
		err := f5.delete(serverSslProfileUrl, nil)
		if err != nil {
			// Iff the profile does not exist, we can continue on to delete the other
			// stuff.
			if err.(F5Error).httpStatusCode != 404 {
				glog.V(4).Infof("Error deleting server-ssl profile for route %s: %v",
					routename, err)
				return err
			}
		}
	}

	if deleteClientSslProfileFromVserver {
		glog.V(4).Infof("Deleting client-ssl profile for route %s"+
			" from vserver %s...", routename, f5.httpsVserver)
		clientSslProfileName := fmt.Sprintf("%s-client-ssl-profile", routename)
		clientSslVserverProfileUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/virtual/%s/profiles/%s",
			f5.host, f5.iControlUriResourceId(f5.httpsVserver), clientSslProfileName)
		err := f5.delete(clientSslVserverProfileUrl, nil)
		if err != nil {
			// Iff the profile is not associated with the vserver, we can continue on
			// to delete the vserver.
			if err.(F5Error).httpStatusCode != 404 {
				glog.V(4).Infof("Error deleting client-ssl profile for route %s"+
					" from vserver %s: %v",
					routename, f5.httpsVserver, err)
				return err
			}
		}
	}

	if deleteClientSslProfile {
		glog.V(4).Infof("Deleting client-ssl profile for route %s...", routename)
		clientSslProfileName := fmt.Sprintf("%s-client-ssl-profile", routename)
		clientSslProfileUrl := fmt.Sprintf("https://%s/mgmt/tm/ltm/profile/client-ssl/%s",
			f5.host, clientSslProfileName)
		err := f5.delete(clientSslProfileUrl, nil)
		if err != nil {
			// Iff the profile does not exist, we can continue on to delete the other
			// stuff.
			if err.(F5Error).httpStatusCode != 404 {
				glog.V(4).Infof("Error deleting client-ssl profile for route %s: %v",
					routename, err)
				return err
			}
		}
	}

	if deletePrivateKey {
		glog.V(4).Infof("Deleting TLS private key for route %s...", routename)
		keyname := fmt.Sprintf("%s-https-key", routename)
		keyUrl := fmt.Sprintf("https://%s/mgmt/tm/sys/file/ssl-key/%s.key",
			f5.host, keyname)
		err := f5.delete(keyUrl, nil)
		if err != nil {
			glog.V(4).Infof("Error deleting TLS private key for route %s: %v",
				routename, err)
		}
	}

	if deleteCert {
		glog.V(4).Infof("Deleting TLS certificate for route %s...", routename)
		certname := fmt.Sprintf("%s-https-cert", routename)
		certUrl := fmt.Sprintf("https://%s/mgmt/tm/sys/file/ssl-cert/%s.crt",
			f5.host, certname)
		err := f5.delete(certUrl, nil)
		if err != nil {
			glog.V(4).Infof("Error deleting TLS certificate for route %s: %v",
				routename, err)
			return err
		}
	}

	if deleteCACert {
		glog.V(4).Infof("Deleting certificate chain for route %s...", routename)
		cacertname := fmt.Sprintf("%s-https-chain", routename)
		cacertUrl := fmt.Sprintf("https://%s/mgmt/tm/sys/file/ssl-cert/%s.crt",
			f5.host, cacertname)
		err := f5.delete(cacertUrl, nil)
		if err != nil {
			glog.V(4).Infof("Error deleting TLS CA certificate for route %s: %v",
				routename, err)
			return err
		}
	}

	return nil
}
