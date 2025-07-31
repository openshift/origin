package compat_otp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	nutanix "github.com/tecbiz-ch/nutanix-go-sdk"
	"github.com/tecbiz-ch/nutanix-go-sdk/schema"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// NutanixSession is an object representing an nutanix session
type NutanixSession struct {
	nutanixClient *nutanix.Client
	Username      string
	Password      string
	Endpoint      string
}

// Secret is an object representing secrets
type Secret struct {
	Data struct {
		Credentials string `json:"credentials"`
	} `json:"data"`
}

// Credential is an object representing  credentials
type Credential struct {
	Type string `json:"type"`
	Data struct {
		PrismCentral struct {
			Username string `json:"username"`
			Password string `json:"password"`
		} `json:"prismCentral"`
	} `json:"data"`
}

// NewNutanixSession creates a new nutanix session from environment credentials
func NewNutanixSession(username, password, endpoint string) (*NutanixSession, error) {
	configCreds := nutanix.Credentials{
		Username: username,
		Password: password,
	}

	opts := []nutanix.ClientOption{
		nutanix.WithCredentials(&configCreds),
		nutanix.WithEndpoint(endpoint),
	}

	client := nutanix.NewClient(opts...)

	nutanixSess := &NutanixSession{
		nutanixClient: client,
		Username:      username,
		Password:      password,
		Endpoint:      endpoint,
	}
	return nutanixSess, nil
}

// GetNutanixCredentialFromCluster gets credentials like username, password, and endpoint URL from the cluster
func GetNutanixCredentialFromCluster(oc *exutil.CLI) (string, string, string, error) {
	credentialJSON, getSecErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/nutanix-credentials", "-n", "openshift-machine-api", "-o", "json").Output()
	if getSecErr != nil || credentialJSON == "" {
		g.Skip("Failed to get credential to access Nutanix, skip the testing.")
	}

	var secret Secret
	errSecret := json.Unmarshal([]byte(credentialJSON), &secret)
	o.Expect(errSecret).NotTo(o.HaveOccurred())

	credentials := secret.Data.Credentials
	decodedCred, decodeCredErr := base64.StdEncoding.DecodeString(credentials)
	o.Expect(decodeCredErr).NotTo(o.HaveOccurred())

	var creds []Credential
	credErr := json.Unmarshal([]byte(decodedCred), &creds)
	o.Expect(credErr).NotTo(o.HaveOccurred())

	if len(creds) == 0 {
		return "", "", "", fmt.Errorf("No nutanix credentials found")
	}

	nutanixEndpointURL, nutanixEndpointURLErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("Infrastructure", "cluster", `-o=jsonpath={.spec.platformSpec.nutanix.prismCentral.address}`).Output()
	o.Expect(nutanixEndpointURLErr).NotTo(o.HaveOccurred())

	nutanixPort, nutanixPortLErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("Infrastructure", "cluster", `-o=jsonpath={.spec.platformSpec.nutanix.prismCentral.port}`).Output()
	o.Expect(nutanixPortLErr).NotTo(o.HaveOccurred())

	return creds[0].Data.PrismCentral.Username, creds[0].Data.PrismCentral.Password, nutanixEndpointURL + ":" + nutanixPort, nil
}

// GetNutanixInstanceID get nutanix instance id
func (nutanixSess *NutanixSession) GetNutanixInstanceID(instanceName string) (string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// First, retrieve the VM details using the List method
	vms, vmsErr := nutanixSess.nutanixClient.VM.List(ctx, &schema.DSMetadata{Filter: fmt.Sprintf("vm_name==%s", instanceName)})
	o.Expect(vmsErr).NotTo(o.HaveOccurred())

	if len(vms.Entities) > 0 {
		instanceID := vms.Entities[0].Metadata.UUID
		return instanceID, nil
	}

	return "", fmt.Errorf("InstanceID not found: %s", instanceName)
}

// GetNutanixInstanceState get nutanix powerstate for e.g. ON or OFF
func (nutanixSess *NutanixSession) GetNutanixInstanceState(instanceID string) (string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	vms, vmsErr := nutanixSess.nutanixClient.VM.List(ctx, &schema.DSMetadata{Filter: fmt.Sprintf("vm_name==%s", instanceID)})
	o.Expect(vmsErr).NotTo(o.HaveOccurred())

	if len(vms.Entities) > 0 {
		instanceStatus := vms.Entities[0].Status
		powerState := *instanceStatus.Resources.PowerState
		e2e.Logf("Power State: %s", powerState)

		// Check the power state
		switch powerState {
		case "ON":
			// Instance is running
			return "running", nil
		case "OFF":
			// Instance is stopped
			return "stopped", nil
		default:
			return "", fmt.Errorf("Invalid power state: %s", powerState)
		}
	}
	return "", nil
}

// SetNutanixInstanceState changes the Nutanix power state, e.g., ON or OFF
func (nutanixSess *NutanixSession) SetNutanixInstanceState(targetState string, instanceUUID string) error {
	// Create the request URL
	url := fmt.Sprintf("https://%s/api/nutanix/v3/vms/%s", nutanixSess.Endpoint, instanceUUID)

	// Retry logic parameters
	maxRetries := 5
	retryDelay := 30 * time.Second

	// Create HTTP client
	client := &http.Client{}

	// Helper function to set common headers and authentication
	setHeadersAndAuth := func(req *http.Request) {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.SetBasicAuth(nutanixSess.Username, nutanixSess.Password)
	}

	// Fetch the VM data
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}
	setHeadersAndAuth(req)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request failed with status code %d", resp.StatusCode)
	}

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %v", err)
	}

	// Update the VM power state in the JSON payload
	var vmData map[string]interface{}
	err = json.Unmarshal(body, &vmData)
	if err != nil {
		return fmt.Errorf("error parsing response JSON: %v", err)
	}
	delete(vmData, "status")
	vmData["spec"].(map[string]interface{})["resources"].(map[string]interface{})["power_state"] = targetState

	// Convert the modified data back to JSON
	payload, err := json.Marshal(vmData)
	if err != nil {
		return fmt.Errorf("error creating request body: %v", err)
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Update the VM state
		reqPut, err := http.NewRequest("PUT", url, bytes.NewBuffer(payload))
		if err != nil {
			return fmt.Errorf("error creating request: %v", err)
		}
		setHeadersAndAuth(reqPut)

		respPut, err := client.Do(reqPut)
		if err != nil {
			return fmt.Errorf("error sending request: %v", err)
		}
		defer respPut.Body.Close()

		if respPut.StatusCode == http.StatusOK || respPut.StatusCode == http.StatusAccepted {
			return nil
		} else if respPut.StatusCode == http.StatusConflict && attempt < maxRetries-1 {
			fmt.Printf("Conflict detected, retrying in %v seconds... (attempt %d/%d)\n", retryDelay.Seconds(), attempt+1, maxRetries)
			time.Sleep(retryDelay)
			continue
		}

		// Read the response body for debugging purposes
		respBody, err := ioutil.ReadAll(respPut.Body)
		if err != nil {
			return fmt.Errorf("error reading response body: %v", err)
		}
		return fmt.Errorf("PUT request failed with status code %d: %s", respPut.StatusCode, string(respBody))
	}

	return fmt.Errorf("request failed after %d attempts", maxRetries)
}
