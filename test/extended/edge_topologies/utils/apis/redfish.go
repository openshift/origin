package apis

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/openshift/origin/test/extended/edge_topologies/utils/core"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/kubernetes/test/e2e/framework"
)

// ParseRedfishAddress parses a Redfish address into its components.
// Input format: "redfish+https://host:port/redfish/v1/Systems/1" (IPv6 uses bracket notation).
func ParseRedfishAddress(address string) (host, port, path string, err error) {
	if !strings.HasPrefix(address, "redfish+") {
		return "", "", "", fmt.Errorf("invalid Redfish address: %q: missing redfish+ prefix", address)
	}
	stripped := strings.TrimPrefix(address, "redfish+")
	parsed, err := url.Parse(stripped)
	if err != nil {
		return "", "", "", fmt.Errorf("parse redfish address %q: %w", address, err)
	}

	host = parsed.Hostname()
	port = parsed.Port()
	path = parsed.Path

	if port == "" {
		if parsed.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	if host == "" {
		return "", "", "", fmt.Errorf("empty host in redfish address %q", address)
	}
	if path == "" {
		return "", "", "", fmt.Errorf("empty path in redfish address %q", address)
	}

	return host, port, path, nil
}

type redfishAccountCollection struct {
	Members []struct {
		OdataID string `json:"@odata.id"`
	} `json:"Members"`
}

type redfishAccount struct {
	ID       string `json:"Id"`
	UserName string `json:"UserName"`
}

// IsSushyEmulator returns true if the Redfish system path contains a UUID,
// which indicates sushy-tools (virtual BMC) rather than real hardware.
// sushy-tools uses paths like /redfish/v1/Systems/<uuid> and does not
// implement AccountService, so credential rotation is not possible.
func IsSushyEmulator(redfishPath string) bool {
	parts := strings.Split(redfishPath, "/")
	if len(parts) == 0 {
		return false
	}
	systemID := parts[len(parts)-1]
	// UUID v4 pattern: 8-4-4-4-12 hex digits
	return len(systemID) == 36 && strings.Count(systemID, "-") == 4
}

// ChangeBMCPasswordViaRedfish changes the BMC password using the Redfish AccountService API.
// It discovers the account matching the given username, then PATCHes the password.
func ChangeBMCPasswordViaRedfish(oc *exutil.CLI, nodeName, redfishHost, redfishPort, username, currentPassword, newPassword string) error {
	authority := net.JoinHostPort(redfishHost, redfishPort)
	baseURL := fmt.Sprintf("https://%s", authority)

	accountURL, err := findRedfishAccountByUsername(oc, nodeName, baseURL, username, currentPassword)
	if err != nil {
		return fmt.Errorf("find redfish account for user %q: %w", username, err)
	}

	framework.Logf("Changing BMC password for account %s on %s", accountURL, authority)

	patchScript := `body=$(jq -n --arg pw "$3" '{"Password":$pw}' | curl -k -s -w "\n%{http_code}" -X PATCH \
		-H 'Content-Type: application/json' \
		-u "$1:$2" \
		-d @- \
		"$4") && printf '%s' "$body"`

	patchURL := baseURL + accountURL
	output, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-etcd",
		"bash", "-c", patchScript, "redfish-patch", username, currentPassword, newPassword, patchURL)
	if err != nil {
		return fmt.Errorf("PATCH %s failed: %w", patchURL, err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	statusCode := lines[len(lines)-1]
	responseBody := strings.Join(lines[:len(lines)-1], "\n")

	if statusCode != "200" && statusCode != "204" {
		const maxBody = 512
		truncated := responseBody
		if len(truncated) > maxBody {
			truncated = truncated[:maxBody] + "..."
		}
		return fmt.Errorf("PATCH %s returned HTTP %s (expected 200 or 204): %s", patchURL, statusCode, truncated)
	}

	framework.Logf("Successfully changed BMC password via Redfish API (HTTP %s)", statusCode)
	return nil
}

func findRedfishAccountByUsername(oc *exutil.CLI, nodeName, baseURL, username, password string) (string, error) {
	accountsURL := baseURL + "/redfish/v1/AccountService/Accounts"
	curlGet := `curl -k -s -u "$1:$2" "$3"`

	output, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-etcd",
		"bash", "-c", curlGet, "redfish-list", username, password, accountsURL)
	if err != nil {
		return "", fmt.Errorf("GET %s failed: %w", accountsURL, err)
	}

	var collection redfishAccountCollection
	if err := json.Unmarshal([]byte(output), &collection); err != nil {
		return "", fmt.Errorf("parse account collection: %w (body: %s)", err, output)
	}

	for _, member := range collection.Members {
		memberURL := baseURL + member.OdataID
		acctOutput, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-etcd",
			"bash", "-c", curlGet, "redfish-get", username, password, memberURL)
		if err != nil {
			framework.Logf("Warning: failed to GET %s: %v", memberURL, err)
			continue
		}

		var account redfishAccount
		if err := json.Unmarshal([]byte(acctOutput), &account); err != nil {
			framework.Logf("Warning: failed to parse account at %s: %v", memberURL, err)
			continue
		}

		if account.UserName == username {
			return member.OdataID, nil
		}
	}

	return "", fmt.Errorf("no Redfish account found with username %q", username)
}

// ChangeSushyToolsPassword changes the BMC password on a sushy-tools virtual BMC
// by updating its htpasswd file and restarting the container via SSH to the hypervisor.
func ChangeSushyToolsPassword(username, newPassword string, sshConfig *core.SSHConfig, knownHostsPath string) error {
	cmd := fmt.Sprintf(
		`htpasswd -nbB %q %q | sudo podman exec -i sushy-tools tee /root/sushy/htpasswd > /dev/null && sudo podman restart sushy-tools`,
		username, newPassword)

	stdout, stderr, err := core.ExecuteSSHCommand(cmd, sshConfig, knownHostsPath)
	if err != nil {
		return fmt.Errorf("change sushy-tools password: %w (stdout: %s, stderr: %s)", err, stdout, stderr)
	}

	framework.Logf("Successfully changed sushy-tools password for user %s", username)
	return nil
}

// ValidateBMCCredentials validates credentials against the BMC using fence_redfish --action status.
func ValidateBMCCredentials(oc *exutil.CLI, nodeName, redfishHost, redfishPort, redfishPath, username, password string, sslInsecure bool) error {
	fenceScript := `/usr/sbin/fence_redfish --username "$1" --password "$2" --ip "$3" --ipport "$4" --systems-uri "$5" --action status`
	if sslInsecure {
		fenceScript += " --ssl-insecure"
	}

	ipForFence := redfishHost
	if strings.Contains(redfishHost, ":") {
		ipForFence = "[" + redfishHost + "]"
	}

	output, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-etcd",
		"bash", "-c", fenceScript, "fence-validate",
		username, password, ipForFence, redfishPort, redfishPath)
	if err != nil {
		return fmt.Errorf("fence_redfish validation failed: %w (output: %s)", err, output)
	}

	framework.Logf("BMC credential validation passed: %s", strings.TrimSpace(output))
	return nil
}
