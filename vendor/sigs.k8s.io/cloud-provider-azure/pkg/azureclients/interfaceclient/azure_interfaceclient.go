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

package interfaceclient

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2022-07-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"

	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	azclients "sigs.k8s.io/cloud-provider-azure/pkg/azureclients"
	"sigs.k8s.io/cloud-provider-azure/pkg/azureclients/armclient"
	"sigs.k8s.io/cloud-provider-azure/pkg/metrics"
	"sigs.k8s.io/cloud-provider-azure/pkg/retry"
)

var _ Interface = &Client{}

const netInterfaceResourceType = "Microsoft.Network/networkInterfaces"

// Client implements network interface client.
type Client struct {
	armClient              armclient.Interface
	subscriptionID         string
	cloudName              string
	disableAzureStackCloud bool

	// Rate limiting configures.
	rateLimiterReader flowcontrol.RateLimiter
	rateLimiterWriter flowcontrol.RateLimiter

	// ARM throttling configures.
	RetryAfterReader time.Time
	RetryAfterWriter time.Time

	computeAPIVersion string
}

// New creates a new network interface client with ratelimiting.
func New(config *azclients.ClientConfig) *Client {
	baseURI := config.ResourceManagerEndpoint
	authorizer := config.Authorizer
	apiVersion := APIVersion
	if strings.EqualFold(config.CloudName, AzureStackCloudName) && !config.DisableAzureStackCloud {
		apiVersion = AzureStackCloudAPIVersion
	}
	armClient := armclient.New(authorizer, *config, baseURI, apiVersion)
	rateLimiterReader, rateLimiterWriter := azclients.NewRateLimiter(config.RateLimitConfig)

	if azclients.RateLimitEnabled(config.RateLimitConfig) {
		klog.V(2).Infof("Azure InterfacesClient (read ops) using rate limit config: QPS=%g, bucket=%d",
			config.RateLimitConfig.CloudProviderRateLimitQPS,
			config.RateLimitConfig.CloudProviderRateLimitBucket)
		klog.V(2).Infof("Azure InterfacesClient (write ops) using rate limit config: QPS=%g, bucket=%d",
			config.RateLimitConfig.CloudProviderRateLimitQPSWrite,
			config.RateLimitConfig.CloudProviderRateLimitBucketWrite)
	}

	computeAPIVersion := ComputeAPIVersion
	if strings.EqualFold(config.CloudName, AzureStackCloudName) && !config.DisableAzureStackCloud {
		computeAPIVersion = AzureStackComputeAPIVersion
	}

	client := &Client{
		armClient:              armClient,
		rateLimiterReader:      rateLimiterReader,
		rateLimiterWriter:      rateLimiterWriter,
		subscriptionID:         config.SubscriptionID,
		cloudName:              config.CloudName,
		disableAzureStackCloud: config.DisableAzureStackCloud,
		computeAPIVersion:      computeAPIVersion,
	}

	return client
}

// Get gets a network.Interface.
func (c *Client) Get(ctx context.Context, resourceGroupName string, networkInterfaceName string, expand string) (network.Interface, *retry.Error) {
	mc := metrics.NewMetricContext("interfaces", "get", resourceGroupName, c.subscriptionID, "")

	// Report errors if the client is rate limited.
	if !c.rateLimiterReader.TryAccept() {
		mc.RateLimitedCount()
		return network.Interface{}, retry.GetRateLimitError(false, "NicGet")
	}

	// Report errors if the client is throttled.
	if c.RetryAfterReader.After(time.Now()) {
		mc.ThrottledCount()
		rerr := retry.GetThrottlingError("NicGet", "client throttled", c.RetryAfterReader)
		return network.Interface{}, rerr
	}

	result, rerr := c.getNetworkInterface(ctx, resourceGroupName, networkInterfaceName, expand)
	mc.Observe(rerr)
	if rerr != nil {
		if rerr.IsThrottled() {
			// Update RetryAfterReader so that no more requests would be sent until RetryAfter expires.
			c.RetryAfterReader = rerr.RetryAfter
		}

		return result, rerr
	}

	return result, nil
}

// getNetworkInterface gets a network.Interface.
func (c *Client) getNetworkInterface(ctx context.Context, resourceGroupName string, networkInterfaceName string, expand string) (network.Interface, *retry.Error) {
	resourceID := armclient.GetResourceID(
		c.subscriptionID,
		resourceGroupName,
		netInterfaceResourceType,
		networkInterfaceName,
	)
	result := network.Interface{}

	response, rerr := c.armClient.GetResourceWithExpandQuery(ctx, resourceID, expand)
	defer c.armClient.CloseResponse(ctx, response)
	if rerr != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "nic.get.request", resourceID, rerr.Error())
		return result, rerr
	}

	err := autorest.Respond(
		response,
		azure.WithErrorUnlessStatusCode(http.StatusOK),
		autorest.ByUnmarshallingJSON(&result))
	if err != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "nic.get.respond", resourceID, err)
		return result, retry.GetError(response, err)
	}

	result.Response = autorest.Response{Response: response}
	return result, nil
}

// GetVirtualMachineScaleSetNetworkInterface gets a network.Interface of VMSS VM.
func (c *Client) GetVirtualMachineScaleSetNetworkInterface(ctx context.Context, resourceGroupName string, virtualMachineScaleSetName string, virtualmachineIndex string, networkInterfaceName string, expand string) (network.Interface, *retry.Error) {
	mc := metrics.NewMetricContext("interfaces", "get_vmss_nic", resourceGroupName, c.subscriptionID, "")

	// Report errors if the client is rate limited.
	if !c.rateLimiterReader.TryAccept() {
		mc.RateLimitedCount()
		return network.Interface{}, retry.GetRateLimitError(false, "NicGetVirtualMachineScaleSetNetworkInterface")
	}

	// Report errors if the client is throttled.
	if c.RetryAfterReader.After(time.Now()) {
		mc.ThrottledCount()
		rerr := retry.GetThrottlingError("NicGetVirtualMachineScaleSetNetworkInterface", "client throttled", c.RetryAfterReader)
		return network.Interface{}, rerr
	}

	result, rerr := c.getVMSSNetworkInterface(ctx, resourceGroupName, virtualMachineScaleSetName, virtualmachineIndex, networkInterfaceName, expand)
	mc.Observe(rerr)
	if rerr != nil {
		if rerr.IsThrottled() {
			// Update RetryAfterReader so that no more requests would be sent until RetryAfter expires.
			c.RetryAfterReader = rerr.RetryAfter
		}

		return result, rerr
	}

	return result, nil
}

// getVMSSNetworkInterface gets a network.Interface of VMSS VM.
func (c *Client) getVMSSNetworkInterface(ctx context.Context, resourceGroupName string, virtualMachineScaleSetName string, virtualmachineIndex string, networkInterfaceName string, expand string) (network.Interface, *retry.Error) {
	resourceID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/virtualMachineScaleSets/%s/virtualMachines/%s/networkInterfaces/%s",
		autorest.Encode("path", c.subscriptionID),
		autorest.Encode("path", resourceGroupName),
		autorest.Encode("path", virtualMachineScaleSetName),
		autorest.Encode("path", virtualmachineIndex),
		autorest.Encode("path", networkInterfaceName),
	)

	result := network.Interface{}

	response, rerr := c.armClient.GetResourceWithExpandAPIVersionQuery(ctx, resourceID, expand, c.computeAPIVersion)
	defer c.armClient.CloseResponse(ctx, response)
	if rerr != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vmssnic.get.request", resourceID, rerr.Error())
		return result, rerr
	}

	err := autorest.Respond(
		response,
		azure.WithErrorUnlessStatusCode(http.StatusOK),
		autorest.ByUnmarshallingJSON(&result))
	if err != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vmssnic.get.respond", resourceID, err)
		return result, retry.GetError(response, err)
	}

	result.Response = autorest.Response{Response: response}
	return result, nil
}

// CreateOrUpdate creates or updates a network.Interface.
func (c *Client) CreateOrUpdate(ctx context.Context, resourceGroupName string, networkInterfaceName string, parameters network.Interface) *retry.Error {
	mc := metrics.NewMetricContext("interfaces", "create_or_update", resourceGroupName, c.subscriptionID, "")

	// Report errors if the client is rate limited.
	if !c.rateLimiterWriter.TryAccept() {
		mc.RateLimitedCount()
		return retry.GetRateLimitError(true, "NicCreateOrUpdate")
	}

	// Report errors if the client is throttled.
	if c.RetryAfterWriter.After(time.Now()) {
		mc.ThrottledCount()
		rerr := retry.GetThrottlingError("NicCreateOrUpdate", "client throttled", c.RetryAfterWriter)
		return rerr
	}

	rerr := c.createOrUpdateInterface(ctx, resourceGroupName, networkInterfaceName, parameters)
	mc.Observe(rerr)
	if rerr != nil {
		if rerr.IsThrottled() {
			// Update RetryAfterReader so that no more requests would be sent until RetryAfter expires.
			c.RetryAfterWriter = rerr.RetryAfter
		}

		return rerr
	}

	return nil
}

// createOrUpdateInterface creates or updates a network.Interface.
func (c *Client) createOrUpdateInterface(ctx context.Context, resourceGroupName string, networkInterfaceName string, parameters network.Interface) *retry.Error {
	resourceID := armclient.GetResourceID(
		c.subscriptionID,
		resourceGroupName,
		netInterfaceResourceType,
		networkInterfaceName,
	)
	decorators := []autorest.PrepareDecorator{}
	if ptr.Deref(parameters.Etag, "") != "" {
		decorators = append(decorators, autorest.WithHeader("If-Match", autorest.String(ptr.Deref(parameters.Etag, ""))))
	}

	response, rerr := c.armClient.PutResource(ctx, resourceID, parameters, decorators...)
	defer c.armClient.CloseResponse(ctx, response)
	if rerr != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "nic.put.request", resourceID, rerr.Error())
		return rerr
	}

	if response != nil && response.StatusCode != http.StatusNoContent {
		_, rerr = c.createOrUpdateResponder(response)
		if rerr != nil {
			klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "nic.put.respond", resourceID, rerr.Error())
			return rerr
		}
	}

	return nil
}

func (c *Client) createOrUpdateResponder(resp *http.Response) (*network.Interface, *retry.Error) {
	result := &network.Interface{}
	err := autorest.Respond(
		resp,
		azure.WithErrorUnlessStatusCode(http.StatusOK, http.StatusCreated),
		autorest.ByUnmarshallingJSON(&result))
	result.Response = autorest.Response{Response: resp}
	return result, retry.GetError(resp, err)
}

// Delete deletes a network interface by name.
func (c *Client) Delete(ctx context.Context, resourceGroupName string, networkInterfaceName string) *retry.Error {
	mc := metrics.NewMetricContext("interfaces", "delete", resourceGroupName, c.subscriptionID, "")

	// Report errors if the client is rate limited.
	if !c.rateLimiterWriter.TryAccept() {
		mc.RateLimitedCount()
		return retry.GetRateLimitError(true, "NicDelete")
	}

	// Report errors if the client is throttled.
	if c.RetryAfterWriter.After(time.Now()) {
		mc.ThrottledCount()
		rerr := retry.GetThrottlingError("NicDelete", "client throttled", c.RetryAfterWriter)
		return rerr
	}

	rerr := c.deleteInterface(ctx, resourceGroupName, networkInterfaceName)
	mc.Observe(rerr)
	if rerr != nil {
		if rerr.IsThrottled() {
			// Update RetryAfterReader so that no more requests would be sent until RetryAfter expires.
			c.RetryAfterWriter = rerr.RetryAfter
		}

		return rerr
	}

	return nil
}

// deleteInterface deletes a network interface by name.
func (c *Client) deleteInterface(ctx context.Context, resourceGroupName string, networkInterfaceName string) *retry.Error {
	resourceID := armclient.GetResourceID(
		c.subscriptionID,
		resourceGroupName,
		netInterfaceResourceType,
		networkInterfaceName,
	)

	return c.armClient.DeleteResource(ctx, resourceID)
}
