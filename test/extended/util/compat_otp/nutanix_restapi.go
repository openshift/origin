package compat_otp

import (
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"

	"github.com/tidwall/gjson"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type NutanixClient struct {
	nutanixToken string
	nutanixHost  string
}

// GetNutanixCredentialFromCluster gets nutanix credentials from cluster
func GetNutanixCredFromCluster(oc *exutil.CLI) (string, error) {
	credential, getSecErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/nutanix-credentials", "-n", "openshift-machine-api", "-o=jsonpath={.data.credentials}").Output()
	if getSecErr != nil {
		return "", fmt.Errorf("Get Nutanix credential Error")
	}

	creJson, err := base64.StdEncoding.DecodeString(credential)
	if err != nil {
		return "", err
	}

	result := gjson.Get(string(creJson), "0.data.prismCentral")
	if !result.Exists() {
		return "", fmt.Errorf("No Nutanix prismCentral credential data found")
	}

	username := result.Get("username").String()
	password := result.Get("password").String()

	if username != "" && password != "" {
		return base64.StdEncoding.EncodeToString([]byte(username + ":" + password)), nil
	}
	return "", fmt.Errorf("No Nutanix credential string found")
}

// GetNutanixHostromCluster Gets nutanix [Host]:port  from cluster
func GetNutanixHostromCluster(oc *exutil.CLI) (string, error) {
	host, getHostErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.spec.platformSpec.nutanix.prismCentral.address}").Output()
	if getHostErr != nil {
		return "", fmt.Errorf("Failed to get Nutanix prismCentral address")
	}

	port, getPortErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.spec.platformSpec.nutanix.prismCentral.port}").Output()
	if getPortErr != nil {
		return "", fmt.Errorf("Failed to get Nutanix prismCentral port")
	}

	if host != "" && port != "" {
		return net.JoinHostPort(host, port), nil
	}

	return "", fmt.Errorf("Failed to get Nutanix whole prismCentral host:port info")
}

// Initial nutanix client by configuring coresponding parameters
func InitNutanixClient(oc *exutil.CLI) (*NutanixClient, error) {
	encodedCre, err := GetNutanixCredFromCluster(oc)
	if err != nil {
		return nil, err
	}
	host, err := GetNutanixHostromCluster(oc)
	if err != nil {
		return nil, err
	}
	nutanixClient := NutanixClient{
		nutanixToken: encodedCre,
		nutanixHost:  host,
	}

	return &nutanixClient, nil

}

// Get Nutanix VM UUID
func (nt *NutanixClient) GetNutanixVMUUID(nodeName string) (string, error) {
	cmdCurl := `curl -s -X POST --header "Content-Type: application/json" \
	--header "Accept: application/json" \
	--header "Authorization: Basic %v" \
	"https://%v/api/nutanix/v3/vms/list" \
	-d '{ "kind": "vm","filter": "","length": 60,"offset": 0}' |
	jq -r '.entities[] | select(.spec.name == "'"%v"'") | .metadata.uuid'
	`
	formattedCmd := fmt.Sprintf(cmdCurl, nt.nutanixToken, nt.nutanixHost, nodeName)
	uuid, cmdErr := exec.Command("bash", "-c", formattedCmd).Output()
	if cmdErr != nil || string(uuid) == "" {
		return "", cmdErr
	}
	return strings.TrimRight(string(uuid), "\n"), nil
}

// Get Nutanix VM state, general value would be "ON" or "OFF"
func (nt *NutanixClient) GetNutanixVMState(vmUUID string) (string, error) {
	cmdCurl := `curl -s --header "Content-Type: application/json"\
	--header "Authorization: Basic %v" \
	"https://%v/api/nutanix/v3/vms/%v" \
	| jq -r '.spec.resources.power_state'
	`
	formattedCmd := fmt.Sprintf(cmdCurl, nt.nutanixToken, nt.nutanixHost, vmUUID)
	state, cmdErr := exec.Command("bash", "-c", formattedCmd).Output()
	if cmdErr != nil || string(state) == "" {
		return "", cmdErr
	}
	return strings.TrimRight(string(state), "\n"), nil
}

// Change NutanixVMstate, target state should be "ON" or "OFF"
func (nt *NutanixClient) ChangeNutanixVMState(vmUUID string, targeState string) error {
	cmdCurl := `curl -s --header "Content-Type: application/json" \
	--header "Accept: application/json" \
	--header "Authorization: Basic %v" \
	"https://%v/api/nutanix/v3/vms/%v"  \
	| jq 'del(.status) | .spec.resources.power_state |= "%v"' > %v
	`
	currentTime := time.Now()
	dateTimeString := currentTime.Format("20060102")
	randStr := GetRandomString()
	filePath := "/tmp/" + randStr + dateTimeString + ".json"
	formattedCmd := fmt.Sprintf(cmdCurl, nt.nutanixToken, nt.nutanixHost, vmUUID, targeState, filePath)
	_, cmdErr := exec.Command("bash", "-c", formattedCmd).Output()
	defer func() {
		if err := os.RemoveAll(filePath); err != nil {
			e2e.Logf("Error removing file %v: %v", filePath, err.Error())
		}
	}()
	if cmdErr != nil {
		return cmdErr
	}

	// Submit the payload to change the VM state
	updateAPI := `curl -s -X 'PUT' --header "Content-Type: application/json" --header "Accept: application/json" --header "Authorization: Basic %v" "https://%v/api/nutanix/v3/vms/%v" -d @%v`
	formattedUpdateCmd := fmt.Sprintf(updateAPI, nt.nutanixToken, nt.nutanixHost, vmUUID, filePath)
	_, cmdErr = exec.Command("bash", "-c", formattedUpdateCmd).Output()
	if cmdErr != nil {
		return cmdErr
	}
	return nil
}
