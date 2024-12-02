/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"

	"sigs.k8s.io/cloud-provider-azure/pkg/azclient"
	"sigs.k8s.io/cloud-provider-azure/pkg/azureclients/armauth"
	"sigs.k8s.io/cloud-provider-azure/pkg/consts"
)

var (
	// ErrorNoAuth indicates that no credentials are provided.
	ErrorNoAuth = fmt.Errorf("no credentials provided for Azure cloud provider")
)

const (
	maxReadLength = 10 * 1 << 20 // 10MB
)

// AzureAuthConfig holds auth related part of cloud config
type AzureAuthConfig struct {
	azclient.ARMClientConfig `json:",inline" yaml:",inline"`
	azclient.AzureAuthConfig `json:",inline" yaml:",inline"`

	// The ID of the Azure Subscription that the cluster is deployed in
	SubscriptionID string `json:"subscriptionId,omitempty" yaml:"subscriptionId,omitempty"`
	// IdentitySystem indicates the identity provider. Relevant only to hybrid clouds (Azure Stack).
	// Allowed values are 'azure_ad' (default), 'adfs'.
	IdentitySystem string `json:"identitySystem,omitempty" yaml:"identitySystem,omitempty"`

	// The ID of the Azure Subscription that the network resources are deployed in
	NetworkResourceSubscriptionID string `json:"networkResourceSubscriptionID,omitempty" yaml:"networkResourceSubscriptionID,omitempty"`
}

// GetServicePrincipalToken creates a new service principal token based on the configuration.
//
// By default, the cluster and its network resources are deployed in the same AAD Tenant and Subscription,
// and all azure clients use this method to fetch Service Principal Token.
//
// If NetworkResourceTenantID and NetworkResourceSubscriptionID are specified to have different values than TenantID and SubscriptionID, network resources are deployed in different AAD Tenant and Subscription than those for the cluster,
// than only azure clients except VM/VMSS and network resource ones use this method to fetch Token.
// For tokens for VM/VMSS and network resource ones, please check GetMultiTenantServicePrincipalToken and GetNetworkResourceServicePrincipalToken.
func GetServicePrincipalToken(config *AzureAuthConfig, env *azure.Environment, resource string) (*adal.ServicePrincipalToken, error) {
	logger := klog.Background().WithName("GetServicePrincipalToken")
	var tenantID string
	if strings.EqualFold(config.IdentitySystem, consts.ADFSIdentitySystem) {
		tenantID = consts.ADFSIdentitySystem
	} else {
		tenantID = config.TenantID
	}

	if resource == "" {
		resource = env.ServiceManagementEndpoint
	}

	if config.UseFederatedWorkloadIdentityExtension {
		logger.V(2).Info("Setup ARM general resource token provider", "method", "workload_identity")
		oauthConfig, err := adal.NewOAuthConfigWithAPIVersion(env.ActiveDirectoryEndpoint, config.TenantID, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create the OAuth config: %w", err)
		}

		jwtCallback := func() (string, error) {
			jwt, err := os.ReadFile(config.AADFederatedTokenFile)
			if err != nil {
				return "", fmt.Errorf("failed to read a file with a federated token: %w", err)
			}
			return string(jwt), nil
		}

		token, err := adal.NewServicePrincipalTokenFromFederatedTokenCallback(*oauthConfig, config.AADClientID, jwtCallback, env.ResourceManagerEndpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to create a workload identity token: %w", err)
		}
		return token, nil
	}

	if config.UseManagedIdentityExtension {
		logger.V(2).Info("Setup ARM general resource token provider", "method", "msi")
		msiEndpoint, err := adal.GetMSIVMEndpoint()
		if err != nil {
			return nil, fmt.Errorf("error getting the managed service identity endpoint: %w", err)
		}
		if len(config.UserAssignedIdentityID) > 0 {
			logger.V(2).Info("Parsing user assigned managed identity")
			resourceID, err := azure.ParseResourceID(config.UserAssignedIdentityID)
			if err == nil &&
				strings.EqualFold(resourceID.Provider, "Microsoft.ManagedIdentity") &&
				strings.EqualFold(resourceID.ResourceType, "userAssignedIdentities") {
				logger.V(2).Info("Setup with user assigned managed identity", "id-type", "resource_id")
				return adal.NewServicePrincipalTokenFromMSIWithIdentityResourceID(msiEndpoint,
					resource,
					config.UserAssignedIdentityID)
			}

			logger.V(2).Info("Setup with user assigned managed identity", "id-type", "client_id")
			return adal.NewServicePrincipalTokenFromMSIWithUserAssignedID(msiEndpoint,
				resource,
				config.UserAssignedIdentityID)
		}
		logger.V(2).Info("Setup with system assigned managed identity")
		return adal.NewServicePrincipalTokenFromMSI(
			msiEndpoint,
			resource)
	}

	oauthConfig, err := adal.NewOAuthConfigWithAPIVersion(env.ActiveDirectoryEndpoint, tenantID, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating the OAuth config: %w", err)
	}

	if len(config.AADClientSecret) > 0 {
		logger.V(2).Info("Setup ARM general resource token provider", "method", "sp_with_password")
		return adal.NewServicePrincipalToken(
			*oauthConfig,
			config.AADClientID,
			config.AADClientSecret,
			resource)
	}

	if len(config.AADClientCertPath) > 0 {
		logger.V(2).Info("Setup ARM general resource token provider", "method", "sp_with_certificate")
		certData, err := os.ReadFile(config.AADClientCertPath)
		if err != nil {
			return nil, fmt.Errorf("reading the client certificate from file %s: %w", config.AADClientCertPath, err)
		}
		certificate, privateKey, err := parseCertificate(certData, config.AADClientCertPassword)
		if err != nil {
			return nil, fmt.Errorf("decoding the client certificate: %w", err)
		}
		return adal.NewServicePrincipalTokenFromCertificate(
			*oauthConfig,
			config.AADClientID,
			certificate,
			privateKey,
			resource)
	}

	logger.V(2).Info("No valid auth method found")

	return nil, ErrorNoAuth
}

// GetMultiTenantServicePrincipalToken is used when (and only when) NetworkResourceTenantID and NetworkResourceSubscriptionID are specified to have different values than TenantID and SubscriptionID.
//
// In that scenario, network resources are deployed in different AAD Tenant and Subscription than those for the cluster,
// and this method creates a new multi-tenant service principal token based on the configuration.
//
// PrimaryToken of the returned multi-tenant token is for the AAD Tenant specified by TenantID, and AuxiliaryToken of the returned multi-tenant token is for the AAD Tenant specified by NetworkResourceTenantID.
//
// Azure VM/VMSS clients use this multi-tenant token, in order to operate those VM/VMSS in AAD Tenant specified by TenantID, and meanwhile in their payload they are referencing network resources (e.g. Load Balancer, Network Security Group, etc.) in AAD Tenant specified by NetworkResourceTenantID.
func GetMultiTenantServicePrincipalToken(config *AzureAuthConfig, env *azure.Environment, authProvider *azclient.AuthProvider) (adal.MultitenantOAuthTokenProvider, error) {
	logger := klog.Background().WithName("GetMultiTenantServicePrincipalToken")

	err := config.ValidateForMultiTenant()
	if err != nil {
		return nil, fmt.Errorf("got error getting multi-tenant service principal token: %w", err)
	}

	multiTenantOAuthConfig, err := adal.NewMultiTenantOAuthConfig(
		env.ActiveDirectoryEndpoint, config.TenantID, []string{config.NetworkResourceTenantID}, adal.OAuthOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating the multi-tenant OAuth config: %w", err)
	}

	if len(config.AADClientSecret) > 0 && !strings.EqualFold(config.AADClientSecret, "msi") {
		logger.V(2).Info("Setup ARM multi-tenant token provider", "method", "sp_with_password")
		return adal.NewMultiTenantServicePrincipalToken(
			multiTenantOAuthConfig,
			config.AADClientID,
			config.AADClientSecret,
			env.ServiceManagementEndpoint)
	}

	if len(config.AADClientCertPath) > 0 {
		logger.V(2).Info("Setup ARM multi-tenant token provider", "method", "sp_with_certificate")
		certData, err := os.ReadFile(config.AADClientCertPath)
		if err != nil {
			return nil, fmt.Errorf("reading the client certificate from file %s: %w", config.AADClientCertPath, err)
		}
		certificate, privateKey, err := parseCertificate(certData, config.AADClientCertPassword)
		if err != nil {
			return nil, fmt.Errorf("decoding the client certificate: %w", err)
		}
		return adal.NewMultiTenantServicePrincipalTokenFromCertificate(
			multiTenantOAuthConfig,
			config.AADClientID,
			certificate,
			privateKey,
			env.ServiceManagementEndpoint)
	}

	if authProvider.ComputeCredential != nil && authProvider.NetworkCredential != nil {
		logger.V(2).Info("Setup ARM multi-tenant token provider", "method", "msi_with_auxiliary_token")
		return armauth.NewMultiTenantTokenProvider(
			klog.Background().WithName("multi-tenant-resource-token-provider"),
			authProvider.ComputeCredential,
			[]azcore.TokenCredential{authProvider.NetworkCredential},
			authProvider.DefaultTokenScope(),
		)
	}

	logger.V(2).Info("No valid auth method found")

	return nil, ErrorNoAuth
}

// GetNetworkResourceServicePrincipalToken is used when (and only when) NetworkResourceTenantID and NetworkResourceSubscriptionID are specified to have different values than TenantID and SubscriptionID.
//
// In that scenario, network resources are deployed in different AAD Tenant and Subscription than those for the cluster,
// and this method creates a new service principal token for network resources tenant based on the configuration.
//
// Azure network resource (Load Balancer, Public IP, Route Table, Network Security Group and their sub level resources) clients use this multi-tenant token, in order to operate resources in AAD Tenant specified by NetworkResourceTenantID.
func GetNetworkResourceServicePrincipalToken(config *AzureAuthConfig, env *azure.Environment, authProvider *azclient.AuthProvider) (adal.OAuthTokenProvider, error) {
	logger := klog.Background().WithName("GetNetworkResourceServicePrincipalToken")

	err := config.ValidateForMultiTenant()
	if err != nil {
		return nil, fmt.Errorf("got error(%w) in getting network resources service principal token", err)
	}

	oauthConfig, err := adal.NewOAuthConfigWithAPIVersion(env.ActiveDirectoryEndpoint, config.NetworkResourceTenantID, nil)
	if err != nil {
		return nil, fmt.Errorf("creating the OAuth config for network resources tenant: %w", err)
	}

	if len(config.AADClientSecret) > 0 && !strings.EqualFold(config.AADClientSecret, "msi") {
		logger.V(2).Info("Setup ARM network resource token provider", "method", "sp_with_password")
		return adal.NewServicePrincipalToken(
			*oauthConfig,
			config.AADClientID,
			config.AADClientSecret,
			env.ServiceManagementEndpoint)
	}

	if len(config.AADClientCertPath) > 0 {
		logger.V(2).Info("Setup ARM network resource token provider", "method", "sp_with_certificate")
		certData, err := os.ReadFile(config.AADClientCertPath)
		if err != nil {
			return nil, fmt.Errorf("reading the client certificate from file %s: %w", config.AADClientCertPath, err)
		}
		certificate, privateKey, err := parseCertificate(certData, config.AADClientCertPassword)
		if err != nil {
			return nil, fmt.Errorf("decoding the client certificate: %w", err)
		}
		return adal.NewServicePrincipalTokenFromCertificate(
			*oauthConfig,
			config.AADClientID,
			certificate,
			privateKey,
			env.ServiceManagementEndpoint)
	}

	if authProvider.ComputeCredential != nil && authProvider.NetworkCredential != nil {
		logger.V(2).Info("Setup ARM network resource token provider", "method", "msi_with_auxiliary_token")

		return armauth.NewTokenProvider(
			klog.Background().WithName("network-resource-token-provider"),
			authProvider.NetworkCredential,
			authProvider.DefaultTokenScope(),
		)
	}

	logger.V(2).Info("No valid auth method found")

	return nil, ErrorNoAuth
}

// ParseAzureEnvironment returns the azure environment.
// If 'resourceManagerEndpoint' is set, the environment is computed by querying the cloud's resource manager endpoint.
// Otherwise, a pre-defined Environment is looked up by name.
func ParseAzureEnvironment(cloudName, resourceManagerEndpoint, identitySystem string) (*azure.Environment, error) {
	var env azure.Environment
	var err error
	if resourceManagerEndpoint != "" {
		klog.V(4).Infof("Loading environment from resource manager endpoint: %s", resourceManagerEndpoint)
		nameOverride := azure.OverrideProperty{Key: azure.EnvironmentName, Value: cloudName}
		env, err = azure.EnvironmentFromURL(resourceManagerEndpoint, nameOverride)
		if err == nil {
			azureStackOverrides(&env, resourceManagerEndpoint, identitySystem)
		}
	} else if cloudName == "" {
		klog.V(4).Info("Using public cloud environment")
		env = azure.PublicCloud
	} else {
		klog.V(4).Infof("Using %s environment", cloudName)
		env, err = azure.EnvironmentFromName(cloudName)
	}
	return &env, err
}

// ParseAzureAuthConfig returns a parsed configuration for an Azure cloudprovider config file
func ParseAzureAuthConfig(configReader io.Reader) (*AzureAuthConfig, *azure.Environment, error) {
	var config AzureAuthConfig

	if configReader == nil {
		return nil, nil, errors.New("nil config is provided")
	}

	limitedReader := &io.LimitedReader{R: configReader, N: maxReadLength}
	configContents, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, nil, err
	}
	if limitedReader.N <= 0 {
		return nil, nil, errors.New("the read limit is reached")
	}
	err = yaml.Unmarshal(configContents, &config)
	if err != nil {
		return nil, nil, err
	}

	environment, err := ParseAzureEnvironment(config.Cloud, config.ResourceManagerEndpoint, config.IdentitySystem)
	if err != nil {
		return nil, nil, err
	}

	return &config, environment, nil
}

// UsesNetworkResourceInDifferentTenant determines whether the AzureAuthConfig indicates to use network resources in
// different AAD Tenant than those for the cluster. Return true when NetworkResourceTenantID is specified  and not equal
// to one defined in global configs
func (config *AzureAuthConfig) UsesNetworkResourceInDifferentTenant() bool {
	return len(config.NetworkResourceTenantID) > 0 && !strings.EqualFold(config.NetworkResourceTenantID, config.TenantID)
}

// UsesNetworkResourceInDifferentSubscription determines whether the AzureAuthConfig indicates to use network resources
// in different Subscription than those for the cluster. Return true when NetworkResourceSubscriptionID is specified
// and not equal to one defined in global configs
func (config *AzureAuthConfig) UsesNetworkResourceInDifferentSubscription() bool {
	return len(config.NetworkResourceSubscriptionID) > 0 && !strings.EqualFold(config.NetworkResourceSubscriptionID, config.SubscriptionID)
}

// azureStackOverrides ensures that the Environment matches what AKSe currently generates for Azure Stack
func azureStackOverrides(env *azure.Environment, resourceManagerEndpoint, identitySystem string) {
	env.ManagementPortalURL = strings.Replace(resourceManagerEndpoint, "https://management.", "https://portal.", -1)
	env.ServiceManagementEndpoint = env.TokenAudience
	env.ResourceManagerVMDNSSuffix = strings.Replace(resourceManagerEndpoint, "https://management.", "cloudapp.", -1)
	env.ResourceManagerVMDNSSuffix = strings.TrimSuffix(env.ResourceManagerVMDNSSuffix, "/")
	if strings.EqualFold(identitySystem, consts.ADFSIdentitySystem) {
		env.ActiveDirectoryEndpoint = strings.TrimSuffix(env.ActiveDirectoryEndpoint, "/")
		env.ActiveDirectoryEndpoint = strings.TrimSuffix(env.ActiveDirectoryEndpoint, "adfs")
	}
}

// ValidateForMultiTenant checks configuration for the scenario of using network resource in different tenant
func (config *AzureAuthConfig) ValidateForMultiTenant() error {
	if !config.UsesNetworkResourceInDifferentTenant() {
		return fmt.Errorf("NetworkResourceTenantID must be configured")
	}

	if strings.EqualFold(config.IdentitySystem, consts.ADFSIdentitySystem) {
		return fmt.Errorf("ADFS identity system is not supported")
	}

	return nil
}

// parseCertificate extracts the x509 certificate and RSA private key from the provided PFX or PEM data.
// The cert data must contain a private key along with a certificate whose public key matches that of the
// private key or an error is returned.
func parseCertificate(certData []byte, password string) (*x509.Certificate, *rsa.PrivateKey, error) {
	certificates, privateKey, err := azidentity.ParseCertificates(certData, []byte(password))
	if err != nil {
		return nil, nil, err
	}

	rsaPrivateKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, nil, fmt.Errorf("failed to parse certificate: private key is not RSA")
	}

	// find the certificate with the matching public key of private key
	for _, cert := range certificates {
		certKey, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			continue
		}
		if rsaPrivateKey.E == certKey.E && rsaPrivateKey.N.Cmp(certKey.N) == 0 {
			// found a match
			return cert, rsaPrivateKey, nil
		}
	}

	return nil, nil, fmt.Errorf("failed to parse certificate: cannot find public key for private key")
}
