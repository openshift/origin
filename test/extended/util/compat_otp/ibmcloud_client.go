package compat_otp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode"

	exutil "github.com/openshift/origin/test/extended/util"

	v "github.com/IBM-Cloud/power-go-client/clients/instance"
	ps "github.com/IBM-Cloud/power-go-client/ibmpisession"
	ac "github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// IBMSession is an object representing an IBM session
type IBMSession struct {
	vpcv1 *vpcv1.VpcV1
}

type IBMPowerVsSession struct {
	powerVsSession *v.IBMPIInstanceClient
}

// NewIBMSessionFromEnv creates a new IBM session from environment credentials
func NewIBMSessionFromEnv(ibmApiKey string) (*IBMSession, error) {
	// Create an IAM authenticator
	authenticator := &core.IamAuthenticator{
		ApiKey: ibmApiKey,
	}

	// Create a VPC service client
	vpcService, err := vpcv1.NewVpcV1(&vpcv1.VpcV1Options{
		Authenticator: authenticator,
	})
	if err != nil {
		return nil, fmt.Errorf("Error creating VPC service client: %v", err)
	}

	session := &IBMSession{
		vpcv1: vpcService,
	}

	return session, nil
}

// IsBase64Encoded checks if the input string is likely base64-encoded.
func IsBase64Encoded(s string) bool {
	// Check if the length is a multiple of 4
	if len(s)%4 != 0 {
		return false
	}

	// Check if the string contains only valid base64 characters
	for _, c := range s {
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '+' && c != '/' && c != '=' {
			return false
		}
	}

	// Attempt to decode the string
	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil
}

// GetIBMCredentialFromCluster gets IBM credentials like ibmapikey, ibmvpc, and ibmregion from the cluster
func GetIBMCredentialFromCluster(oc *exutil.CLI) (string, string, string, error) {
	var (
		credential       string
		credentialAPIKey []byte
		credErr          error
	)

	platform := CheckPlatform(oc)
	if platform == "powervs" {
		credential, credErr = oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/ibm-cloud-credentials", "-n", "openshift-cloud-controller-manager", "-o=jsonpath={.data.ibmcloud_api_key}").Output()
	} else {
		credential, credErr = oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/qe-ibmcloud-creds", "-n", "kube-system", "-o=jsonpath={.data.apiKey}").Output()
	}

	if credErr != nil || len(credential) == 0 {
		// Fallback to reading from the CLUSTER_PROFILE_DIR
		clusterProfileDir := os.Getenv("CLUSTER_PROFILE_DIR")
		if clusterProfileDir == "" {
			return "", "", "", fmt.Errorf("error getting environment variable CLUSTER_PROFILE_DIR")
		}
		credentialAPIKey, credErr = os.ReadFile(clusterProfileDir + "/ibmcloud-api-key")
		if credErr != nil || len(credentialAPIKey) == 0 {
			g.Skip("Failed to get credential to access IBM, skip the testing.")
		}
		credential = string(credentialAPIKey)
	}

	credential = strings.TrimSpace(credential)

	if IsBase64Encoded(credential) {
		credDecode, err := base64.StdEncoding.DecodeString(credential)
		if err != nil || string(credDecode) == "" {
			return "", "", "", fmt.Errorf("Error decoding IBM credentials: %s", err)
		}
		credential = string(credDecode)
		credential = strings.TrimSpace(credential)
	}

	ibmRegion, regionErr := GetIBMRegion(oc)
	if regionErr != nil {
		return "", "", "", regionErr
	}

	ibmResourceGrpName, ibmResourceGrpNameErr := GetIBMResourceGrpName(oc)
	if ibmResourceGrpNameErr != nil {
		return "", "", "", ibmResourceGrpNameErr
	}

	if platform == "powervs" {
		return credential, ibmRegion, ibmResourceGrpName, nil
	}
	return credential, ibmRegion, ibmResourceGrpName + "-vpc", nil
}

// StopIBMInstance stop the IBM instance
func StopIBMInstance(session *IBMSession, instanceID string) error {
	stopInstanceOptions := session.vpcv1.NewCreateInstanceActionOptions(instanceID, "stop")
	_, _, err := session.vpcv1.CreateInstanceAction(stopInstanceOptions)
	if err != nil {
		return fmt.Errorf("Unable to stop IBM instance: %v", err)
	}
	return nil
}

// StartIBMInstance start the IBM instance
func StartIBMInstance(session *IBMSession, instanceID string) error {
	startInstanceOptions := session.vpcv1.NewCreateInstanceActionOptions(instanceID, "start")
	_, _, err := session.vpcv1.CreateInstanceAction(startInstanceOptions)
	if err != nil {
		return fmt.Errorf("Unable to start IBM instance: %v", err)
	}
	return nil
}

// GetIBMInstanceID get IBM instance id
func GetIBMInstanceID(session *IBMSession, oc *exutil.CLI, region string, vpcName string, instanceID string, baseDomain string) (string, error) {
	err := SetVPCServiceURLForRegion(session, region)
	if err != nil {
		return "", fmt.Errorf("Failed to set vpc api service url :: %v", err)
	}

	// Retrieve the VPC ID based on the VPC name
	listVpcsOptions := session.vpcv1.NewListVpcsOptions()
	vpcs, _, err := session.vpcv1.ListVpcs(listVpcsOptions)
	if err != nil {
		return "", fmt.Errorf("Error listing VPCs: %v", err)
	}

	var vpcID string
	for _, vpc := range vpcs.Vpcs {
		if *vpc.Name == vpcName {
			vpcID = *vpc.ID
			e2e.Logf("VpcID found of VpcName %s :: %s", vpcName, vpcID)
			break
		}
	}

	if vpcID == "" {
		// Attempt to extract VPC ID using the DNS base domain
		vpcID, err = ExtractVPCIDFromBaseDomain(oc, vpcs.Vpcs, baseDomain)
		if err != nil {
			return "", fmt.Errorf("VPC not found: %s", vpcName)
		}
	}

	// Set the VPC ID in the listInstancesOptions
	listInstancesOptions := session.vpcv1.NewListInstancesOptions()
	listInstancesOptions.SetVPCID(vpcID)

	// Retrieve the list of instances in the specified VPC
	instances, _, err := session.vpcv1.ListInstances(listInstancesOptions)
	if err != nil {
		return "", fmt.Errorf("Error listing instances: %v", err)
	}

	// Search for the instance by name
	for _, instance := range instances.Instances {
		if *instance.Name == instanceID {
			return *instance.ID, nil
		}
	}

	return "", fmt.Errorf("Instance not found for name: %s", instanceID)
}

// GetIBMInstanceStatus check IBM instance running status
func GetIBMInstanceStatus(session *IBMSession, instanceID string) (string, error) {
	getInstanceOptions := session.vpcv1.NewGetInstanceOptions(instanceID)
	instance, _, err := session.vpcv1.GetInstance(getInstanceOptions)
	if err != nil {
		return "", err
	}
	return *instance.Status, nil
}

// SetVPCServiceURLForRegion will set the VPC Service URL to a specific IBM Cloud Region, in order to access Region scoped resources
func SetVPCServiceURLForRegion(session *IBMSession, region string) error {
	regionOptions := session.vpcv1.NewGetRegionOptions(region)
	vpcRegion, _, err := session.vpcv1.GetRegion(regionOptions)
	if err != nil {
		return err
	}
	err = session.vpcv1.SetServiceURL(fmt.Sprintf("%s/v1", *vpcRegion.Endpoint))
	if err != nil {
		return err
	}
	return nil
}

// GetIBMRegion gets IBM cluster region
func GetIBMRegion(oc *exutil.CLI) (string, error) {
	platformType := CheckPlatform(oc)
	var ibmRegion string
	var regionErr error

	switch platformType {
	case "ibmcloud":
		ibmRegion, regionErr = oc.AsAdmin().WithoutNamespace().Run("get").Args("Infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.ibmcloud.location}").Output()
	case "powervs":
		ibmRegion, regionErr = oc.AsAdmin().WithoutNamespace().Run("get").Args("Infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.powervs.zone}").Output()
	default:
		return "", fmt.Errorf("Unsupported platform type: %s", platformType)
	}

	if regionErr != nil || ibmRegion == "" {
		return "", regionErr
	}

	return ibmRegion, nil
}

// GetIBMResourceGrpName get IBM cluster resource group name
func GetIBMResourceGrpName(oc *exutil.CLI) (string, error) {
	platformType := CheckPlatform(oc)
	var ibmResourceGrpName string
	var ibmResourceGrpNameErr error
	switch platformType {
	case "ibmcloud":
		ibmResourceGrpName, ibmResourceGrpNameErr = oc.AsAdmin().WithoutNamespace().Run("get").Args("Infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.ibmcloud.resourceGroupName}").Output()
	case "powervs":
		ibmResourceGrpName, ibmResourceGrpNameErr = oc.AsAdmin().WithoutNamespace().Run("get").Args("Infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.powervs}").Output()
		if ibmResourceGrpNameErr == nil && ibmResourceGrpName != "" {
			re := regexp.MustCompile(`a/([a-f0-9]+):`)
			match := re.FindStringSubmatch(ibmResourceGrpName)
			if len(match) < 2 {
				e2e.Failf("Not able to get ResourceGrp name")
			}
			ibmResourceGrpName = match[1]
		}
	default:
		return "", fmt.Errorf("Unsupported platform type: %s", platformType)
	}

	if ibmResourceGrpNameErr != nil || ibmResourceGrpName == "" {
		return "", ibmResourceGrpNameErr
	}

	return ibmResourceGrpName, nil
}

// LoginIBMPowerVsCloud authenticates and returns a session for Powervs cloud
func LoginIBMPowerVsCloud(apiKey, zone, userAccount string, cloudId string) (*IBMPowerVsSession, error) {
	// Authenticator
	authenticator := &core.IamAuthenticator{
		ApiKey: apiKey,
	}

	// Create the session
	options := &ps.IBMPIOptions{
		Authenticator: authenticator,
		Zone:          zone,
		UserAccount:   userAccount,
	}
	session, err := ps.NewIBMPISession(options)
	if err != nil {
		return nil, err
	}
	// Create the instance client
	powerClient := v.NewIBMPIInstanceClient(context.Background(), session, cloudId)
	return &IBMPowerVsSession{powerVsSession: powerClient}, nil
}

// PerformInstanceActionOnPowerVs performs start or stop action on the instance
func PerformInstanceActionOnPowerVs(powerClient *IBMPowerVsSession, instanceID, action string) error {
	powerAction := &ac.PVMInstanceAction{
		Action: core.StringPtr(action),
	}
	return powerClient.powerVsSession.Action(instanceID, powerAction)
}

// GetIBMPowerVsCloudID get powervsCloud Id
func GetIBMPowerVsCloudID(oc *exutil.CLI, nodeName string) string {
	jsonString, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", nodeName, "-o=jsonpath={.spec}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	type Data struct {
		ProviderID string `json:"providerID"`
	}

	// Parse the JSON string into the defined struct
	var data Data
	err = json.Unmarshal([]byte(jsonString), &data)
	o.Expect(err).NotTo(o.HaveOccurred())

	// Extract the ID part from the providerID field
	parts := strings.Split(data.ProviderID, "/")
	if len(parts) < 4 {
		e2e.Failf("Invalid providerID format")

	}
	instanceID := parts[4]
	return instanceID
}

// GetInstanceInfo retrieves information for the specified instance name
func GetIBMPowerVsInstanceInfo(powerClient *IBMPowerVsSession, instanceName string) (string, string, error) {
	// Get all instances
	getAllResp, err := powerClient.powerVsSession.GetAll()
	if err != nil {
		return "", "", err
	}

	// Print instance information
	for _, inst := range getAllResp.PvmInstances {
		if *inst.ServerName == instanceName {
			e2e.Logf("ID: %s, Name: %s, Status: %s\n", *inst.PvmInstanceID, *inst.ServerName, *inst.Status)
			return *inst.PvmInstanceID, strings.ToLower(*inst.Status), nil
		}
	}

	return "", "", nil
}

// ExtractVPCIDFromBaseDomain extracts the VPC ID based on the DNS base domain.
func ExtractVPCIDFromBaseDomain(oc *exutil.CLI, vpcs []vpcv1.VPC, baseDomain string) (string, error) {
	baseDomain = strings.TrimSpace(baseDomain)
	parts := strings.Split(baseDomain, ".")
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid base domain format")
	}
	resourceGroupName := parts[0]
	e2e.Logf("Extracted resource group name from DNS base domain: %s", resourceGroupName)
	expectedVpcName := resourceGroupName + "-vpc"
	// Find the VPC with the matching name
	for _, vpc := range vpcs {
		if *vpc.Name == expectedVpcName {
			vpcID := *vpc.ID
			e2e.Logf("VpcID found for VpcName %s: %s", *vpc.Name, vpcID)
			return vpcID, nil
		}
	}
	return "", fmt.Errorf("VPC not found: %s", expectedVpcName)
}
