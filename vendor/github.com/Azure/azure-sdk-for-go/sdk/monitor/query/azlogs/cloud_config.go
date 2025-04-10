//go:build go1.18
// +build go1.18

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License. See License.txt in the project root for license information.

package azlogs

import "github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"

// Cloud Service Names for Monitor Query Logs, used to identify the respective cloud.ServiceConfiguration
const ServiceName cloud.ServiceName = "query/azlogs"

func init() {
	cloud.AzureChina.Services[ServiceName] = cloud.ServiceConfiguration{
		Audience: "https://api.loganalytics.azure.cn",
		Endpoint: "https://api.loganalytics.azure.cn/v1",
	}
	cloud.AzureGovernment.Services[ServiceName] = cloud.ServiceConfiguration{
		Audience: "https://api.loganalytics.us",
		Endpoint: "https://api.loganalytics.us/v1",
	}
	cloud.AzurePublic.Services[ServiceName] = cloud.ServiceConfiguration{
		Audience: "https://api.loganalytics.io",
		Endpoint: "https://api.loganalytics.io/v1",
	}
}
