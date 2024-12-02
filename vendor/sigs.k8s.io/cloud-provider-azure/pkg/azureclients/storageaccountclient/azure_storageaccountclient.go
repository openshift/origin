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

package storageaccountclient

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2021-09-01/storage"
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

const saResourceType = "Microsoft.Storage/storageAccounts"

// Client implements StorageAccount client Interface.
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

// New creates a new StorageAccount client with ratelimiting.
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
		klog.V(2).Infof("Azure StorageAccountClient (read ops) using rate limit config: QPS=%g, bucket=%d",
			config.RateLimitConfig.CloudProviderRateLimitQPS,
			config.RateLimitConfig.CloudProviderRateLimitBucket)
		klog.V(2).Infof("Azure StorageAccountClient (write ops) using rate limit config: QPS=%g, bucket=%d",
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

// GetProperties gets properties of the StorageAccount.
func (c *Client) GetProperties(ctx context.Context, subsID, resourceGroupName, accountName string) (storage.Account, *retry.Error) {
	if subsID == "" {
		subsID = c.subscriptionID
	}
	mc := metrics.NewMetricContext("storage_account", "get", resourceGroupName, subsID, "")

	// Report errors if the client is rate limited.
	if !c.rateLimiterReader.TryAccept() {
		mc.RateLimitedCount()
		return storage.Account{}, retry.GetRateLimitError(false, "StorageAccountGet")
	}

	// Report errors if the client is throttled.
	if c.RetryAfterReader.After(time.Now()) {
		mc.ThrottledCount()
		rerr := retry.GetThrottlingError("StorageAccountGet", "client throttled", c.RetryAfterReader)
		return storage.Account{}, rerr
	}

	result, rerr := c.getStorageAccount(ctx, subsID, resourceGroupName, accountName)
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

// getStorageAccount gets properties of the StorageAccount.
func (c *Client) getStorageAccount(ctx context.Context, subsID, resourceGroupName string, accountName string) (storage.Account, *retry.Error) {
	resourceID := armclient.GetResourceID(
		subsID,
		resourceGroupName,
		saResourceType,
		accountName,
	)
	result := storage.Account{}

	response, rerr := c.armClient.GetResource(ctx, resourceID)
	defer c.armClient.CloseResponse(ctx, response)
	if rerr != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "storageaccount.get.request", resourceID, rerr.Error())
		return result, rerr
	}

	err := autorest.Respond(
		response,
		azure.WithErrorUnlessStatusCode(http.StatusOK),
		autorest.ByUnmarshallingJSON(&result))
	if err != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "storageaccount.get.respond", resourceID, err)
		return result, retry.GetError(response, err)
	}

	result.Response = autorest.Response{Response: response}
	return result, nil
}

// ListKeys get a list of storage account keys.
func (c *Client) ListKeys(ctx context.Context, subsID, resourceGroupName, accountName string) (storage.AccountListKeysResult, *retry.Error) {
	if subsID == "" {
		subsID = c.subscriptionID
	}
	mc := metrics.NewMetricContext("storage_account", "list_keys", resourceGroupName, subsID, "")

	// Report errors if the client is rate limited.
	if !c.rateLimiterReader.TryAccept() {
		mc.RateLimitedCount()
		return storage.AccountListKeysResult{}, retry.GetRateLimitError(false, "StorageAccountListKeys")
	}

	// Report errors if the client is throttled.
	if c.RetryAfterReader.After(time.Now()) {
		mc.ThrottledCount()
		rerr := retry.GetThrottlingError("StorageAccountListKeys", "client throttled", c.RetryAfterReader)
		return storage.AccountListKeysResult{}, rerr
	}

	result, rerr := c.listStorageAccountKeys(ctx, subsID, resourceGroupName, accountName)
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

// listStorageAccountKeys get a list of storage account keys.
func (c *Client) listStorageAccountKeys(ctx context.Context, subsID, resourceGroupName, accountName string) (storage.AccountListKeysResult, *retry.Error) {
	resourceID := armclient.GetResourceID(
		subsID,
		resourceGroupName,
		saResourceType,
		accountName,
	)

	result := storage.AccountListKeysResult{}
	response, rerr := c.armClient.PostResource(ctx, resourceID, "listKeys", struct{}{}, map[string]interface{}{})
	defer c.armClient.CloseResponse(ctx, response)
	if rerr != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "storageaccount.listkeys.request", resourceID, rerr.Error())
		return result, rerr
	}

	err := autorest.Respond(
		response,
		azure.WithErrorUnlessStatusCode(http.StatusOK),
		autorest.ByUnmarshallingJSON(&result))
	if err != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "storageaccount.listkeys.respond", resourceID, err)
		return result, retry.GetError(response, err)
	}

	result.Response = autorest.Response{Response: response}
	return result, nil
}

// Create creates a StorageAccount.
func (c *Client) Create(ctx context.Context, subsID, resourceGroupName, accountName string, parameters storage.AccountCreateParameters) *retry.Error {
	if subsID == "" {
		subsID = c.subscriptionID
	}
	mc := metrics.NewMetricContext("storage_account", "create", resourceGroupName, subsID, "")

	// Report errors if the client is rate limited.
	if !c.rateLimiterWriter.TryAccept() {
		mc.RateLimitedCount()
		return retry.GetRateLimitError(true, "StorageAccountCreate")
	}

	// Report errors if the client is throttled.
	if c.RetryAfterWriter.After(time.Now()) {
		mc.ThrottledCount()
		rerr := retry.GetThrottlingError("StorageAccountCreate", "client throttled", c.RetryAfterWriter)
		return rerr
	}

	rerr := c.createStorageAccount(ctx, subsID, resourceGroupName, accountName, parameters)
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

// createStorageAccount creates or updates a StorageAccount.
func (c *Client) createStorageAccount(ctx context.Context, subsID, resourceGroupName, accountName string, parameters storage.AccountCreateParameters) *retry.Error {
	resourceID := armclient.GetResourceID(
		subsID,
		resourceGroupName,
		saResourceType,
		accountName,
	)

	response, rerr := c.armClient.PutResource(ctx, resourceID, parameters)
	defer c.armClient.CloseResponse(ctx, response)
	if rerr != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "storageAccount.put.request", resourceID, rerr.Error())
		return rerr
	}

	if response != nil && response.StatusCode != http.StatusNoContent {
		_, rerr = c.createResponder(response)
		if rerr != nil {
			klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "storageAccount.put.respond", resourceID, rerr.Error())
			return rerr
		}
	}

	return nil
}

func (c *Client) createResponder(resp *http.Response) (*storage.Account, *retry.Error) {
	result := &storage.Account{}
	err := autorest.Respond(
		resp,
		azure.WithErrorUnlessStatusCode(http.StatusOK, http.StatusAccepted),
		autorest.ByUnmarshallingJSON(&result))
	result.Response = autorest.Response{Response: resp}
	return result, retry.GetError(resp, err)
}

// Update updates a storage account.
func (c *Client) Update(ctx context.Context, subsID, resourceGroupName, accountName string, parameters storage.AccountUpdateParameters) *retry.Error {
	if subsID == "" {
		subsID = c.subscriptionID
	}
	mc := metrics.NewMetricContext("storage_account", "update", resourceGroupName, subsID, "")

	// Report errors if the client is rate limited.
	if !c.rateLimiterWriter.TryAccept() {
		mc.RateLimitedCount()
		return retry.GetRateLimitError(true, "StorageAccountUpdate")
	}

	// Report errors if the client is throttled.
	if c.RetryAfterWriter.After(time.Now()) {
		mc.ThrottledCount()
		rerr := retry.GetThrottlingError("StorageAccountUpdate", "client throttled", c.RetryAfterWriter)
		return rerr
	}

	rerr := c.updateStorageAccount(ctx, subsID, resourceGroupName, accountName, parameters)
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

// updateStorageAccount updates a StorageAccount.
func (c *Client) updateStorageAccount(ctx context.Context, subsID, resourceGroupName, accountName string, parameters storage.AccountUpdateParameters) *retry.Error {
	resourceID := armclient.GetResourceID(
		subsID,
		resourceGroupName,
		saResourceType,
		accountName,
	)

	response, rerr := c.armClient.PatchResource(ctx, resourceID, parameters)
	defer c.armClient.CloseResponse(ctx, response)
	if rerr != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "storageAccount.patch.request", resourceID, rerr.Error())
		return rerr
	}

	if response != nil && response.StatusCode != http.StatusNoContent {
		_, rerr = c.updateResponder(response)
		if rerr != nil {
			klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "storageAccount.patch.respond", resourceID, rerr.Error())
			return rerr
		}
	}

	return nil
}

func (c *Client) updateResponder(resp *http.Response) (*storage.Account, *retry.Error) {
	result := &storage.Account{}
	err := autorest.Respond(
		resp,
		azure.WithErrorUnlessStatusCode(http.StatusOK, http.StatusAccepted),
		autorest.ByUnmarshallingJSON(&result))
	result.Response = autorest.Response{Response: resp}
	return result, retry.GetError(resp, err)
}

// Delete deletes a StorageAccount by name.
func (c *Client) Delete(ctx context.Context, subsID, resourceGroupName, accountName string) *retry.Error {
	if subsID == "" {
		subsID = c.subscriptionID
	}
	mc := metrics.NewMetricContext("storage_account", "delete", resourceGroupName, subsID, "")

	// Report errors if the client is rate limited.
	if !c.rateLimiterWriter.TryAccept() {
		mc.RateLimitedCount()
		return retry.GetRateLimitError(true, "StorageAccountDelete")
	}

	// Report errors if the client is throttled.
	if c.RetryAfterWriter.After(time.Now()) {
		mc.ThrottledCount()
		rerr := retry.GetThrottlingError("StorageAccountDelete", "client throttled", c.RetryAfterWriter)
		return rerr
	}

	rerr := c.deleteStorageAccount(ctx, subsID, resourceGroupName, accountName)
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

// deleteStorageAccount deletes a PublicIPAddress by name.
func (c *Client) deleteStorageAccount(ctx context.Context, subsID, resourceGroupName, accountName string) *retry.Error {
	resourceID := armclient.GetResourceID(
		subsID,
		resourceGroupName,
		saResourceType,
		accountName,
	)

	return c.armClient.DeleteResource(ctx, resourceID)
}

// ListByResourceGroup get a list storage accounts by resourceGroup.
func (c *Client) ListByResourceGroup(ctx context.Context, subsID, resourceGroupName string) ([]storage.Account, *retry.Error) {
	if subsID == "" {
		subsID = c.subscriptionID
	}
	mc := metrics.NewMetricContext("storage_account", "list_by_resource_group", resourceGroupName, subsID, "")

	// Report errors if the client is rate limited.
	if !c.rateLimiterReader.TryAccept() {
		mc.RateLimitedCount()
		return nil, retry.GetRateLimitError(false, "StorageAccountListByResourceGroup")
	}

	// Report errors if the client is throttled.
	if c.RetryAfterReader.After(time.Now()) {
		mc.ThrottledCount()
		rerr := retry.GetThrottlingError("StorageAccountListByResourceGroup", "client throttled", c.RetryAfterReader)
		return nil, rerr
	}

	result, rerr := c.ListStorageAccountByResourceGroup(ctx, subsID, resourceGroupName)
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

// ListStorageAccountByResourceGroup get a list storage accounts by resourceGroup.
func (c *Client) ListStorageAccountByResourceGroup(ctx context.Context, subsID, resourceGroupName string) ([]storage.Account, *retry.Error) {
	if subsID == "" {
		subsID = c.subscriptionID
	}
	resourceID := armclient.GetResourceListID(subsID, resourceGroupName, saResourceType)
	result := make([]storage.Account, 0)
	page := &AccountListResultPage{}
	page.fn = c.listNextResults

	resp, rerr := c.armClient.GetResource(ctx, resourceID)
	defer c.armClient.CloseResponse(ctx, resp)
	if rerr != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "storageAccount.list.request", resourceID, rerr.Error())
		return result, rerr
	}

	var err error
	page.alr, err = c.listResponder(resp)
	if err != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "storageAccount.list.respond", resourceID, err)
		return result, retry.GetError(resp, err)
	}

	for {
		result = append(result, page.Values()...)

		// Abort the loop when there's no nextLink in the response.
		if ptr.Deref(page.Response().NextLink, "") == "" {
			break
		}

		if err = page.NextWithContext(ctx); err != nil {
			klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "storageAccount.list.next", resourceID, err)
			return result, retry.GetError(page.Response().Response.Response, err)
		}
	}

	return result, nil
}

func (c *Client) listResponder(resp *http.Response) (result storage.AccountListResult, err error) {
	err = autorest.Respond(
		resp,
		autorest.ByIgnoring(),
		azure.WithErrorUnlessStatusCode(http.StatusOK),
		autorest.ByUnmarshallingJSON(&result))
	result.Response = autorest.Response{Response: resp}
	return
}

// StorageAccountResultPreparer prepares a request to retrieve the next set of results.
// It returns nil if no more results exist.
func (c *Client) StorageAccountResultPreparer(ctx context.Context, lr storage.AccountListResult) (*http.Request, error) {
	if lr.NextLink == nil || len(ptr.Deref(lr.NextLink, "")) < 1 {
		return nil, nil
	}

	decorators := []autorest.PrepareDecorator{
		autorest.WithBaseURL(ptr.Deref(lr.NextLink, "")),
	}
	return c.armClient.PrepareGetRequest(ctx, decorators...)
}

// listNextResults retrieves the next set of results, if any.
func (c *Client) listNextResults(ctx context.Context, lastResults storage.AccountListResult) (result storage.AccountListResult, err error) {
	req, err := c.StorageAccountResultPreparer(ctx, lastResults)
	if err != nil {
		return result, autorest.NewErrorWithError(err, "storageaccount", "listNextResults", nil, "Failure preparing next results request")
	}
	if req == nil {
		return
	}

	resp, rerr := c.armClient.Send(ctx, req)
	defer c.armClient.CloseResponse(ctx, resp)
	if rerr != nil {
		result.Response = autorest.Response{Response: resp}
		return result, autorest.NewErrorWithError(rerr.Error(), "storageaccount", "listNextResults", resp, "Failure sending next results request")
	}

	result, err = c.listResponder(resp)
	if err != nil {
		err = autorest.NewErrorWithError(err, "storageaccount", "listNextResults", resp, "Failure responding to next results request")
	}

	return
}

// AccountListResultPage contains a page of Account values.
type AccountListResultPage struct {
	fn  func(context.Context, storage.AccountListResult) (storage.AccountListResult, error)
	alr storage.AccountListResult
}

// NextWithContext advances to the next page of values.  If there was an error making
// the request the page does not advance and the error is returned.
func (page *AccountListResultPage) NextWithContext(ctx context.Context) (err error) {
	next, err := page.fn(ctx, page.alr)
	if err != nil {
		return err
	}
	page.alr = next
	return nil
}

// Next advances to the next page of values.  If there was an error making
// the request the page does not advance and the error is returned.
// Deprecated: Use NextWithContext() instead.
func (page *AccountListResultPage) Next() error {
	return page.NextWithContext(context.Background())
}

// NotDone returns true if the page enumeration should be started or is not yet complete.
func (page AccountListResultPage) NotDone() bool {
	return !page.alr.IsEmpty()
}

// Response returns the raw server response from the last page request.
func (page AccountListResultPage) Response() storage.AccountListResult {
	return page.alr
}

// Values returns the slice of values for the current page or nil if there are no values.
func (page AccountListResultPage) Values() []storage.Account {
	if page.alr.IsEmpty() {
		return nil
	}
	return *page.alr.Value
}
