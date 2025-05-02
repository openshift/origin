# Azure Monitor Query Logs client module for Go

* Query logs (this module): execute read-only queries against [Azure Monitor Logs][logs_overview]
* Query metrics ([query/azmetrics][azmetrics]): execute read-only queries against [Azure Monitor Metrics][metrics_overview]
* Upload logs ([ingestion/azlogs][ingestion_azlogs]): send custom logs to [Azure Monitor][azure_monitor_overview] using the [Logs Ingestion API][ingestion_overview]

[Source code][source_code] | [Package (pkg.go.dev)][pkg_go] | [REST API documentation][monitor_rest_docs] | [Product documentation][monitor_docs] | [Samples][examples]

## Getting started

### Prerequisites

* Go, version 1.18 or higher - [Install Go](https://go.dev/doc/install)
* Azure subscription - [Create a free account][azure_sub]
* To query logs, you need one of the following things:
  * An [Azure Log Analytics workspace][log_analytics_workspace_create]
  * The resource ID of an Azure resource (Storage Account, Key Vault, Cosmos DB, etc.)

### Install the packages

Install the `azlogs` and `azidentity` modules with `go get`:

```bash
go get github.com/Azure/azure-sdk-for-go/sdk/monitor/query/azlogs
go get github.com/Azure/azure-sdk-for-go/sdk/azidentity
```

The [azidentity][azure_identity] module is used for Azure Active Directory authentication during client construction.

### Authentication

An authenticated client object is required to execute a query. The examples demonstrate using [azidentity.NewDefaultAzureCredential][default_cred_ref] to authenticate; however, the client accepts any [azidentity][azure_identity] credential. See the [azidentity][azure_identity] documentation for more information about other credential types.

The clients default to the Azure public cloud. For other cloud configurations, see the [cloud][cloud_documentation] package documentation.

#### Create a client

Example [client][example_client].

## Key concepts

[Azure Monitor Logs][logs_overview] collects and organizes log and performance data from monitored resources. Data from different sources such as platform logs from Azure services, log and performance data from virtual machines agents, and usage and performance data from apps can be consolidated into a single [Azure Log Analytics workspace][log_analytics_workspace]. The various data types can be analyzed together using the [Kusto Query Language][kusto_query_language]. See the [Kusto to SQL cheat sheet][kusto_to_sql] for more information.

### Timespan

It's best practice to always query with a timespan (type `TimeInterval`) to prevent excessive queries of the entire logs data set. Log queries use the ISO8601 Time Interval Standard. All time should be represented in UTC. If the timespan is included in both the Kusto query string and `Timespan` field, the timespan is the intersection of the two values.

Use the `NewTimeInterval()` method for easy creation.

### Logs query rate limits and throttling

The Log Analytics service applies throttling when the request rate is too high. Limits, such as the maximum number of rows returned, are also applied on the Kusto queries. For more information, see [Query API][service_limits].

If you're executing a batch logs query, a throttled request will return a `ErrorInfo` object. That object's `code` value will be `ThrottledError`.

### Advanced logs queries

#### Query multiple workspaces

To run the same query against multiple Log Analytics workspaces, add the additional workspace ID strings to the `AdditionalWorkspaces` slice in the `QueryBody` struct. 

When multiple workspaces are included in the query, the logs in the result table are not grouped according to the workspace from which they were retrieved.

#### Increase wait time, include statistics, include visualization

The `QueryOptions` type is used for advanced logs options.

* By default, your query will run for up to three minutes. To increase the default timeout, set `QueryOptions.Wait` to the desired number of seconds. The maximum wait time is 10 minutes (600 seconds).

* To get logs query execution statistics, such as CPU and memory consumption, set `QueryOptions.Statistics` to `true`.

* To get visualization data for logs queries, set `QueryOptions.Visualization` to `true`.

```go
azlogs.QueryWorkspaceOptions{
			Options: &azlogs.QueryOptions{
				Statistics:    to.Ptr(true),
				Visualization: to.Ptr(true),
				Wait:          to.Ptr(600),
			},
		}
```

## Examples

Get started with our [examples][examples].

## Contributing

This project welcomes contributions and suggestions. Most contributions require you to agree to a [Contributor License Agreement (CLA)][cla] declaring that you have the right to, and actually do, grant us the rights to use your contribution.

When you submit a pull request, a CLA-bot will automatically determine whether you need to provide a CLA and decorate
the PR appropriately (e.g., label, comment). Simply follow the instructions provided by the bot. You will only need to
do this once across all repos using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct][coc]. For more information, see
the [Code of Conduct FAQ][coc_faq] or contact [opencode@microsoft.com][coc_contact] with any additional questions or
comments.

<!-- LINKS -->
[azmetrics]: https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/monitor/query/azmetrics
[azure_identity]: https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azidentity
[azure_sub]: https://azure.microsoft.com/free/
[azure_monitor_overview]: https://learn.microsoft.com/azure/azure-monitor/overview
[cloud_documentation]: https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud
[default_cred_ref]: https://github.com/Azure/azure-sdk-for-go/tree/main/sdk/azidentity#defaultazurecredential
[example_client]: https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/monitor/query/azlogs#example-NewClient
[examples]: https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/monitor/query/azlogs#pkg-examples
[ingestion_azlogs]: https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/monitor/ingestion/azlogs
[ingestion_overview]: https://learn.microsoft.com/azure/azure-monitor/logs/logs-ingestion-api-overview
[kusto_query_language]: https://learn.microsoft.com/azure/data-explorer/kusto/query/
[kusto_to_sql]: https://learn.microsoft.com/azure/data-explorer/kusto/query/sqlcheatsheet
[log_analytics_workspace]: https://learn.microsoft.com/azure/azure-monitor/logs/log-analytics-workspace-overview
[log_analytics_workspace_create]: https://learn.microsoft.com/azure/azure-monitor/logs/quick-create-workspace
[logs_overview]: https://learn.microsoft.com/azure/azure-monitor/logs/data-platform-logs
[metrics_overview]: https://learn.microsoft.com/azure/azure-monitor/essentials/data-platform-metrics
[monitor_docs]: https://learn.microsoft.com/azure/azure-monitor/
[monitor_rest_docs]: https://learn.microsoft.com/rest/api/monitor/
[pkg_go]: https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/monitor/query/azlogs
[service_limits]: https://learn.microsoft.com/azure/azure-monitor/service-limits#la-query-api
[source_code]: https://github.com/Azure/azure-sdk-for-go/tree/main/sdk/monitor/query/azlogs
[cla]: https://cla.microsoft.com
[coc]: https://opensource.microsoft.com/codeofconduct/
[coc_faq]: https://opensource.microsoft.com/codeofconduct/faq/
[coc_contact]: mailto:opencode@microsoft.com