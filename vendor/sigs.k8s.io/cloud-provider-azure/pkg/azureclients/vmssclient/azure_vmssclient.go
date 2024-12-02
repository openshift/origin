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

package vmssclient

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2022-08-01/compute"
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

const vmssResourceType = "Microsoft.Compute/virtualMachineScaleSets"

// Client implements VMSS client Interface.
type Client struct {
	armClient      armclient.Interface
	subscriptionID string
	cloudName      string

	// Rate limiting configures.
	rateLimiterReader flowcontrol.RateLimiter
	rateLimiterWriter flowcontrol.RateLimiter

	// ARM throttling configures.
	RetryAfterReader time.Time
	RetryAfterWriter time.Time
}

// New creates a new VMSS client with ratelimiting.
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
		klog.V(2).Infof("Azure VirtualMachineScaleSetClient (read ops) using rate limit config: QPS=%g, bucket=%d",
			config.RateLimitConfig.CloudProviderRateLimitQPS,
			config.RateLimitConfig.CloudProviderRateLimitBucket)
		klog.V(2).Infof("Azure VirtualMachineScaleSetClient (write ops) using rate limit config: QPS=%g, bucket=%d",
			config.RateLimitConfig.CloudProviderRateLimitQPSWrite,
			config.RateLimitConfig.CloudProviderRateLimitBucketWrite)
	}

	client := &Client{
		armClient:         armClient,
		rateLimiterReader: rateLimiterReader,
		rateLimiterWriter: rateLimiterWriter,
		subscriptionID:    config.SubscriptionID,
		cloudName:         config.CloudName,
	}

	return client
}

// Get gets a VirtualMachineScaleSet.
func (c *Client) Get(ctx context.Context, resourceGroupName string, VMScaleSetName string) (compute.VirtualMachineScaleSet, *retry.Error) {
	mc := metrics.NewMetricContext("vmss", "get", resourceGroupName, c.subscriptionID, "")

	// Report errors if the client is rate limited.
	if !c.rateLimiterReader.TryAccept() {
		mc.RateLimitedCount()
		return compute.VirtualMachineScaleSet{}, retry.GetRateLimitError(false, "VMSSGet")
	}

	// Report errors if the client is throttled.
	if c.RetryAfterReader.After(time.Now()) {
		mc.ThrottledCount()
		rerr := retry.GetThrottlingError("VMSSGet", "client throttled", c.RetryAfterReader)
		return compute.VirtualMachineScaleSet{}, rerr
	}

	result, rerr := c.getVMSS(ctx, resourceGroupName, VMScaleSetName)
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

// getVMSS gets a VirtualMachineScaleSet.
func (c *Client) getVMSS(ctx context.Context, resourceGroupName string, VMScaleSetName string) (compute.VirtualMachineScaleSet, *retry.Error) {
	resourceID := armclient.GetResourceID(
		c.subscriptionID,
		resourceGroupName,
		vmssResourceType,
		VMScaleSetName,
	)
	result := compute.VirtualMachineScaleSet{}

	response, rerr := c.armClient.GetResource(ctx, resourceID)
	defer c.armClient.CloseResponse(ctx, response)
	if rerr != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vmss.get.request", resourceID, rerr.Error())
		return result, rerr
	}

	err := autorest.Respond(
		response,
		azure.WithErrorUnlessStatusCode(http.StatusOK),
		autorest.ByUnmarshallingJSON(&result))
	if err != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vmss.get.respond", resourceID, err)
		return result, retry.GetError(response, err)
	}

	result.Response = autorest.Response{Response: response}
	return result, nil
}

// List gets a list of VirtualMachineScaleSets in the resource group.
func (c *Client) List(ctx context.Context, resourceGroupName string) ([]compute.VirtualMachineScaleSet, *retry.Error) {
	mc := metrics.NewMetricContext("vmss", "list", resourceGroupName, c.subscriptionID, "")

	// Report errors if the client is rate limited.
	if !c.rateLimiterReader.TryAccept() {
		mc.RateLimitedCount()
		return nil, retry.GetRateLimitError(false, "VMSSList")
	}

	// Report errors if the client is throttled.
	if c.RetryAfterReader.After(time.Now()) {
		mc.ThrottledCount()
		rerr := retry.GetThrottlingError("VMSSList", "client throttled", c.RetryAfterReader)
		return nil, rerr
	}

	result, rerr := c.listVMSS(ctx, resourceGroupName)
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

// listVMSS gets a list of VirtualMachineScaleSets in the resource group.
func (c *Client) listVMSS(ctx context.Context, resourceGroupName string) ([]compute.VirtualMachineScaleSet, *retry.Error) {
	resourceID := armclient.GetResourceListID(
		c.subscriptionID,
		resourceGroupName,
		vmssResourceType,
	)
	result := make([]compute.VirtualMachineScaleSet, 0)
	page := &VirtualMachineScaleSetListResultPage{}
	page.fn = c.listNextResults

	resp, rerr := c.armClient.GetResource(ctx, resourceID)
	defer c.armClient.CloseResponse(ctx, resp)
	if rerr != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vmss.list.request", resourceID, rerr.Error())
		return result, rerr
	}

	var err error
	page.vmsslr, err = c.listResponder(resp)
	if err != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vmss.list.respond", resourceID, err)
		return result, retry.GetError(resp, err)
	}

	for {
		result = append(result, page.Values()...)

		// Abort the loop when there's no nextLink in the response.
		if ptr.Deref(page.Response().NextLink, "") == "" {
			break
		}

		if err = page.NextWithContext(ctx); err != nil {
			klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vmss.list.next", resourceID, err)
			return result, retry.GetError(page.Response().Response.Response, err)
		}
	}

	return result, nil
}

// CreateOrUpdate creates or updates a VirtualMachineScaleSet.
func (c *Client) CreateOrUpdate(ctx context.Context, resourceGroupName string, VMScaleSetName string, parameters compute.VirtualMachineScaleSet) *retry.Error {
	mc := metrics.NewMetricContext("vmss", "create_or_update", resourceGroupName, c.subscriptionID, "")

	// Report errors if the client is rate limited.
	if !c.rateLimiterWriter.TryAccept() {
		mc.RateLimitedCount()
		return retry.GetRateLimitError(true, "VMSSCreateOrUpdate")
	}

	// Report errors if the client is throttled.
	if c.RetryAfterWriter.After(time.Now()) {
		mc.ThrottledCount()
		rerr := retry.GetThrottlingError("VMSSCreateOrUpdate", "client throttled", c.RetryAfterWriter)
		return rerr
	}

	rerr := c.createOrUpdateVMSS(ctx, resourceGroupName, VMScaleSetName, parameters)
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

// CreateOrUpdateAsync sends the request to arm client and DO NOT wait for the response
func (c *Client) CreateOrUpdateAsync(ctx context.Context, resourceGroupName string, VMScaleSetName string, parameters compute.VirtualMachineScaleSet) (*azure.Future, *retry.Error) {
	mc := metrics.NewMetricContext("vmss", "create_or_update_async", resourceGroupName, c.subscriptionID, "")

	// Report errors if the client is rate limited.
	if !c.rateLimiterWriter.TryAccept() {
		mc.RateLimitedCount()
		return nil, retry.GetRateLimitError(true, "VMSSCreateOrUpdateAsync")
	}

	// Report errors if the client is throttled.
	if c.RetryAfterWriter.After(time.Now()) {
		mc.ThrottledCount()
		rerr := retry.GetThrottlingError("VMSSCreateOrUpdateAsync", "client throttled", c.RetryAfterWriter)
		return nil, rerr
	}

	resourceID := armclient.GetResourceID(
		c.subscriptionID,
		resourceGroupName,
		vmssResourceType,
		VMScaleSetName,
	)

	future, rerr := c.armClient.PutResourceAsync(ctx, resourceID, parameters)
	mc.Observe(rerr)
	if rerr != nil {
		if rerr.IsThrottled() {
			// Update RetryAfterReader so that no more requests would be sent until RetryAfter expires.
			c.RetryAfterWriter = rerr.RetryAfter
		}

		return nil, rerr
	}

	return future, nil
}

// WaitForCreateOrUpdateResult waits for the response of the create or update request
func (c *Client) WaitForCreateOrUpdateResult(ctx context.Context, future *azure.Future, resourceGroupName string) (*http.Response, error) {
	return c.WaitForAsyncOperationResult(ctx, future, resourceGroupName, "wait_for_create_or_update_result", "VMSSWaitForCreateOrUpdateResult")
}

// WaitForDeleteInstancesResult waits for the response of the delete instance request
func (c *Client) WaitForDeleteInstancesResult(ctx context.Context, future *azure.Future, resourceGroupName string) (*http.Response, error) {
	return c.WaitForAsyncOperationResult(ctx, future, resourceGroupName, "wait_for_delete_instances_result", "VMSSWaitForDeleteInstancesResult")
}

// WaitForDeallocateInstancesResult waits for the response of the delete instance request
func (c *Client) WaitForDeallocateInstancesResult(ctx context.Context, future *azure.Future, resourceGroupName string) (*http.Response, error) {
	return c.WaitForAsyncOperationResult(ctx, future, resourceGroupName, "wait_for_deallocate_instances_result", "VMSSWaitForDeallocateInstancesResult")
}

// WaitForStartInstancesResult waits for the response of the delete instance request
func (c *Client) WaitForStartInstancesResult(ctx context.Context, future *azure.Future, resourceGroupName string) (*http.Response, error) {
	return c.WaitForAsyncOperationResult(ctx, future, resourceGroupName, "wait_for_start_instances_result", "VMSSWaitForStartInstancesResult")
}

// WaitForAsyncOperationResult waits for the response of the request
func (c *Client) WaitForAsyncOperationResult(ctx context.Context, future *azure.Future, resourceGroupName, request, asycOpName string) (*http.Response, error) {
	mc := metrics.NewMetricContext("vmss", request, resourceGroupName, c.subscriptionID, "")
	res, err := c.armClient.WaitForAsyncOperationResult(ctx, future, asycOpName)
	mc.Observe(retry.NewErrorOrNil(false, err))
	return res, err
}

// createOrUpdateVMSS creates or updates a VirtualMachineScaleSet.
func (c *Client) createOrUpdateVMSS(ctx context.Context, resourceGroupName string, VMScaleSetName string, parameters compute.VirtualMachineScaleSet) *retry.Error {
	resourceID := armclient.GetResourceID(
		c.subscriptionID,
		resourceGroupName,
		vmssResourceType,
		VMScaleSetName,
	)
	response, rerr := c.armClient.PutResource(ctx, resourceID, parameters)
	defer c.armClient.CloseResponse(ctx, response)
	if rerr != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vmss.put.request", resourceID, rerr.Error())
		return rerr
	}

	if response != nil && response.StatusCode != http.StatusNoContent {
		_, rerr = c.createOrUpdateResponder(response)
		if rerr != nil {
			klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vmss.put.respond", resourceID, rerr.Error())
			return rerr
		}
	}

	return nil
}

func (c *Client) createOrUpdateResponder(resp *http.Response) (*compute.VirtualMachineScaleSet, *retry.Error) {
	result := &compute.VirtualMachineScaleSet{}
	err := autorest.Respond(
		resp,
		azure.WithErrorUnlessStatusCode(http.StatusOK, http.StatusCreated),
		autorest.ByUnmarshallingJSON(&result))
	result.Response = autorest.Response{Response: resp}
	return result, retry.GetError(resp, err)
}

func (c *Client) listResponder(resp *http.Response) (result compute.VirtualMachineScaleSetListResult, err error) {
	err = autorest.Respond(
		resp,
		autorest.ByIgnoring(),
		azure.WithErrorUnlessStatusCode(http.StatusOK),
		autorest.ByUnmarshallingJSON(&result))
	result.Response = autorest.Response{Response: resp}
	return
}

// virtualMachineScaleSetListResultPreparer prepares a request to retrieve the next set of results.
// It returns nil if no more results exist.
func (c *Client) virtualMachineScaleSetListResultPreparer(ctx context.Context, vmsslr compute.VirtualMachineScaleSetListResult) (*http.Request, error) {
	if vmsslr.NextLink == nil || len(ptr.Deref(vmsslr.NextLink, "")) < 1 {
		return nil, nil
	}

	decorators := []autorest.PrepareDecorator{
		autorest.WithBaseURL(ptr.Deref(vmsslr.NextLink, "")),
	}
	return c.armClient.PrepareGetRequest(ctx, decorators...)
}

// listNextResults retrieves the next set of results, if any.
func (c *Client) listNextResults(ctx context.Context, lastResults compute.VirtualMachineScaleSetListResult) (result compute.VirtualMachineScaleSetListResult, err error) {
	req, err := c.virtualMachineScaleSetListResultPreparer(ctx, lastResults)
	if err != nil {
		return result, autorest.NewErrorWithError(err, "vmssclient", "listNextResults", nil, "Failure preparing next results request")
	}
	if req == nil {
		return
	}

	resp, rerr := c.armClient.Send(ctx, req)
	defer c.armClient.CloseResponse(ctx, resp)
	if rerr != nil {
		result.Response = autorest.Response{Response: resp}
		return result, autorest.NewErrorWithError(rerr.Error(), "vmssclient", "listNextResults", resp, "Failure sending next results request")
	}

	result, err = c.listResponder(resp)
	if err != nil {
		err = autorest.NewErrorWithError(err, "vmssclient", "listNextResults", resp, "Failure responding to next results request")
	}

	return
}

// VirtualMachineScaleSetListResultPage contains a page of VirtualMachineScaleSet values.
type VirtualMachineScaleSetListResultPage struct {
	fn     func(context.Context, compute.VirtualMachineScaleSetListResult) (compute.VirtualMachineScaleSetListResult, error)
	vmsslr compute.VirtualMachineScaleSetListResult
}

// NextWithContext advances to the next page of values.  If there was an error making
// the request the page does not advance and the error is returned.
func (page *VirtualMachineScaleSetListResultPage) NextWithContext(ctx context.Context) (err error) {
	next, err := page.fn(ctx, page.vmsslr)
	if err != nil {
		return err
	}
	page.vmsslr = next
	return nil
}

// Next advances to the next page of values.  If there was an error making
// the request the page does not advance and the error is returned.
// Deprecated: Use NextWithContext() instead.
func (page *VirtualMachineScaleSetListResultPage) Next() error {
	return page.NextWithContext(context.Background())
}

// NotDone returns true if the page enumeration should be started or is not yet complete.
func (page VirtualMachineScaleSetListResultPage) NotDone() bool {
	return !page.vmsslr.IsEmpty()
}

// Response returns the raw server response from the last page request.
func (page VirtualMachineScaleSetListResultPage) Response() compute.VirtualMachineScaleSetListResult {
	return page.vmsslr
}

// Values returns the slice of values for the current page or nil if there are no values.
func (page VirtualMachineScaleSetListResultPage) Values() []compute.VirtualMachineScaleSet {
	if page.vmsslr.IsEmpty() {
		return nil
	}
	return *page.vmsslr.Value
}

// DeleteInstances deletes the instances for a VirtualMachineScaleSet.
func (c *Client) DeleteInstances(ctx context.Context, resourceGroupName string, vmScaleSetName string, vmInstanceIDs compute.VirtualMachineScaleSetVMInstanceRequiredIDs) *retry.Error {
	mc := metrics.NewMetricContext("vmss", "delete_instances", resourceGroupName, c.subscriptionID, "")

	// Report errors if the client is rate limited.
	if !c.rateLimiterWriter.TryAccept() {
		mc.RateLimitedCount()
		return retry.GetRateLimitError(true, "VMSSDeleteInstances")
	}

	// Report errors if the client is throttled.
	if c.RetryAfterWriter.After(time.Now()) {
		mc.ThrottledCount()
		rerr := retry.GetThrottlingError("VMSSDeleteInstances", "client throttled", c.RetryAfterWriter)
		return rerr
	}

	rerr := c.deleteVMSSInstances(ctx, resourceGroupName, vmScaleSetName, vmInstanceIDs)
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

// DeleteInstancesAsync sends the delete request to ARM client and DOES NOT wait on the future
func (c *Client) DeleteInstancesAsync(ctx context.Context, resourceGroupName string, vmScaleSetName string, vmInstanceIDs compute.VirtualMachineScaleSetVMInstanceRequiredIDs, forceDelete bool) (*azure.Future, *retry.Error) {
	mc := metrics.NewMetricContext("vmss", "delete_instances_async", resourceGroupName, c.subscriptionID, "")

	// Report errors if the client is rate limited.
	if !c.rateLimiterWriter.TryAccept() {
		mc.RateLimitedCount()
		return nil, retry.GetRateLimitError(true, "VMSSDeleteInstancesAsync")
	}

	// Report errors if the client is throttled.
	if c.RetryAfterWriter.After(time.Now()) {
		mc.ThrottledCount()
		rerr := retry.GetThrottlingError("VMSSDeleteInstancesAsync", "client throttled", c.RetryAfterWriter)
		return nil, rerr
	}

	resourceID := armclient.GetResourceID(
		c.subscriptionID,
		resourceGroupName,
		vmssResourceType,
		vmScaleSetName,
	)

	var queryParameters map[string]interface{}
	if forceDelete {
		queryParameters = map[string]interface{}{
			"forceDeletion": true,
		}
	}
	response, rerr := c.armClient.PostResource(ctx, resourceID, "delete", vmInstanceIDs, queryParameters)
	defer c.armClient.CloseResponse(ctx, response)

	if rerr != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vmss.deletevms.request", resourceID, rerr.Error())
		return nil, rerr
	}

	err := autorest.Respond(response, azure.WithErrorUnlessStatusCode(http.StatusOK, http.StatusAccepted))
	if err != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vmss.deletevms.respond", resourceID, err)
		return nil, retry.GetError(response, err)
	}

	future, err := azure.NewFutureFromResponse(response)
	rerr = retry.NewErrorOrNil(false, err)
	mc.Observe(rerr)
	if err != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vmss.deletevms.future", resourceID, err)
		return nil, rerr
	}

	return &future, nil
}

// DeallocateInstancesAsync sends the deallocate request to ARM client and DOES NOT wait on the future
func (c *Client) DeallocateInstancesAsync(ctx context.Context, resourceGroupName string, vmScaleSetName string, vmInstanceIDs compute.VirtualMachineScaleSetVMInstanceRequiredIDs) (*azure.Future, *retry.Error) {
	mc := metrics.NewMetricContext("vmss", "deallocate_instances_async", resourceGroupName, c.subscriptionID, "")

	// Report errors if the client is rate limited.
	if !c.rateLimiterWriter.TryAccept() {
		mc.RateLimitedCount()
		return nil, retry.GetRateLimitError(true, "VMSSDeallocateInstancesAsync")
	}

	// Report errors if the client is throttled.
	if c.RetryAfterWriter.After(time.Now()) {
		mc.ThrottledCount()
		rerr := retry.GetThrottlingError("VMSSDeallocateInstancesAsync", "client throttled", c.RetryAfterWriter)
		return nil, rerr
	}

	resourceID := armclient.GetResourceID(
		c.subscriptionID,
		resourceGroupName,
		vmssResourceType,
		vmScaleSetName,
	)

	response, rerr := c.armClient.PostResource(ctx, resourceID, "deallocate", vmInstanceIDs, map[string]interface{}{})
	defer c.armClient.CloseResponse(ctx, response)

	if rerr != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vmss.deallocatevms.request", resourceID, rerr.Error())
		return nil, rerr
	}

	err := autorest.Respond(response, azure.WithErrorUnlessStatusCode(http.StatusOK, http.StatusAccepted))
	if err != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vmss.deallocatevms.respond", resourceID, err)
		return nil, retry.GetError(response, err)
	}

	future, err := azure.NewFutureFromResponse(response)
	rerr = retry.NewErrorOrNil(false, err)
	mc.Observe(rerr)
	if err != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vmss.deallocatevms.future", resourceID, err)
		return nil, rerr
	}

	return &future, nil
}

// StartInstancesAsync sends the start request to ARM client and DOES NOT wait on the future
func (c *Client) StartInstancesAsync(ctx context.Context, resourceGroupName string, vmScaleSetName string, vmInstanceIDs compute.VirtualMachineScaleSetVMInstanceRequiredIDs) (*azure.Future, *retry.Error) {
	mc := metrics.NewMetricContext("vmss", "start_instances_async", resourceGroupName, c.subscriptionID, "")

	// Report errors if the client is rate limited.
	if !c.rateLimiterWriter.TryAccept() {
		mc.RateLimitedCount()
		return nil, retry.GetRateLimitError(true, "VMSSStartInstancesAsync")
	}

	// Report errors if the client is throttled.
	if c.RetryAfterWriter.After(time.Now()) {
		mc.ThrottledCount()
		rerr := retry.GetThrottlingError("VMSSStartInstancesAsync", "client throttled", c.RetryAfterWriter)
		return nil, rerr
	}

	resourceID := armclient.GetResourceID(
		c.subscriptionID,
		resourceGroupName,
		vmssResourceType,
		vmScaleSetName,
	)

	response, rerr := c.armClient.PostResource(ctx, resourceID, "start", vmInstanceIDs, map[string]interface{}{})
	defer c.armClient.CloseResponse(ctx, response)

	if rerr != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vmss.startvms.request", resourceID, rerr.Error())
		return nil, rerr
	}

	err := autorest.Respond(response, azure.WithErrorUnlessStatusCode(http.StatusOK, http.StatusAccepted))
	if err != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vmss.startvms.respond", resourceID, err)
		return nil, retry.GetError(response, err)
	}

	future, err := azure.NewFutureFromResponse(response)
	rerr = retry.NewErrorOrNil(false, err)
	mc.Observe(rerr)
	if err != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vmss.startvms.future", resourceID, err)
		return nil, rerr
	}

	return &future, nil
}

// deleteVMSSInstances deletes the instances for a VirtualMachineScaleSet.
func (c *Client) deleteVMSSInstances(ctx context.Context, resourceGroupName string, vmScaleSetName string, vmInstanceIDs compute.VirtualMachineScaleSetVMInstanceRequiredIDs) *retry.Error {
	resourceID := armclient.GetResourceID(
		c.subscriptionID,
		resourceGroupName,
		vmssResourceType,
		vmScaleSetName,
	)
	response, rerr := c.armClient.PostResource(ctx, resourceID, "delete", vmInstanceIDs, map[string]interface{}{})
	defer c.armClient.CloseResponse(ctx, response)
	if rerr != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vmss.deletevms.request", resourceID, rerr.Error())
		return rerr
	}

	err := autorest.Respond(response, azure.WithErrorUnlessStatusCode(http.StatusOK, http.StatusAccepted))
	if err != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vmss.deletevms.respond", resourceID, rerr.Error())
		return retry.GetError(response, err)
	}

	future, err := azure.NewFutureFromResponse(response)
	if err != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vmss.deletevms.future", resourceID, rerr.Error())
		return retry.NewError(false, err)
	}

	if err := c.armClient.WaitForAsyncOperationCompletion(ctx, &future, "vmssclient.DeleteInstances"); err != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vmss.deletevms.wait", resourceID, rerr.Error())
		return retry.NewError(false, err)
	}

	return nil
}
