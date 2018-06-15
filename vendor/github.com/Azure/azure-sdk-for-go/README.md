# Azure SDK for Go

[![godoc](https://godoc.org/github.com/Azure/azure-sdk-for-go?status.svg)](https://godoc.org/github.com/Azure/azure-sdk-for-go) 
[![Build Status](https://travis-ci.org/Azure/azure-sdk-for-go.svg?branch=master)](https://travis-ci.org/Azure/azure-sdk-for-go) 
[![Go Report Card](https://goreportcard.com/badge/github.com/Azure/azure-sdk-for-go)](https://goreportcard.com/report/github.com/Azure/azure-sdk-for-go)

azure-sdk-for-go provides Go packages for using Azure services. It has been
tested with Go 1.8 and 1.9. To be notified about updates and changes, subscribe
to the [Azure update feed][].

:exclamation: **NOTE:** This project is in preview and breaking changes are 
introduced frequently. Therefore, vendoring dependencies is even more
important than usual. We use [dep](https://github.com/golang/dep).

### Install:

```sh
$ go get -u github.com/Azure/azure-sdk-for-go/...
```

or if you use dep (recommended), within your project run:

```sh
$ dep ensure -add github.com/Azure/azure-sdk-for-go
```

If you need to install Go, follow [the official instructions][].

[the official instructions]: https://golang.org/dl/

### Use:

For complete examples see [Azure-Samples/azure-sdk-for-go-samples][samples_repo].

1. Import a package from the [services][services_dir] directory.
1. Create and authenticate a client with a `New*Client` func, e.g.
   `c := compute.NewVirtualMachinesClient(...)`.
1. Invoke API methods using the client, e.g. `c.CreateOrUpdate(...)`.
1. Handle responses.

For example, to create a new virtual network (substitute your own values for
strings in angle brackets):

```go
import (
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2017-09-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
)

vnetClient := network.NewVirtualNetworksClient("<subscriptionID>")
vnetClient.Authorizer = autorest.NewBearerAuthorizer("<OAuthTokenProvider>")

vnetClient.CreateOrUpdate(
  "<resourceGroupName>",
  "<vnetName>",
  network.VirtualNetwork{
    Location: to.StringPtr("<azureRegion>"),
    VirtualNetworkPropertiesFormat: &network.VirtualNetworkPropertiesFormat{
      AddressSpace: &network.AddressSpace{
        AddressPrefixes: &[]string{"10.0.0.0/8"},
      },
      Subnets: &[]network.Subnet{
        {
          Name: to.StringPtr("<subnet1Name>"),
          SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
            AddressPrefix: to.StringPtr("10.0.0.0/16"),
          },
        },
        {
          Name: to.StringPtr("<subnet2Name>"),
          SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
            AddressPrefix: to.StringPtr("10.1.0.0/16"),
          },
        },
      },
    },
  },
  nil)
```

## Authentication

Most operations require an OAuth token for authentication and authorization.
You can get one from Azure AD using the
[adal](https://github.com/Azure/go-autorest/tree/master/autorest/adal)
package and a service principal as shown in the following example.

If you need to create a service principal (aka Client, App), run
`az ad sp create-for-rbac -n "<app_name>"` 
in the [azure-cli](https://github.com/Azure/azure-cli), see these
[docs](https://docs.microsoft.com/cli/azure/create-an-azure-service-principal-azure-cli?view=azure-cli-latest)
for more info.

Copy the new principal's ID, secret, and tenant ID for use in your app, e.g. as follows:

```go
import (
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
)

var (
	clientID =        "<service_principal_ID>"
	subscriptionID =  "<subscription_ID>"
	tenantID =        "<tenant_ID>"
	clientSecret =    "<service_principal_secret>"
)

func getServicePrincipalToken() (adal.OAuthTokenProvider, error) {
	config, err := adal.NewOAuthConfig(azure.PublicCloud.ActiveDirectoryEndpoint, tenantID)
	return adal.NewServicePrincipalToken(
		*oauthConfig,
		clientID,
		clientSecret,
		azure.PublicCloud.ResourceManagerEndpoint)
}
```

## Background

Most packages in the SDK are generated from [Azure API specs][] with
[Azure/autorest][] and [Azure/autorest.go][].

[Azure API Specs]: https://github.com/Azure/azure-rest-api-specs
[Azure/autorest]: https://github.com/Azure/autorest
[Azure/autorest.go]: https://github.com/Azure/autorest.go

A list of available services and package import paths is in [SERVICES.md][].

## Versioning

*More info in [the wiki](https://github.com/Azure/azure-sdk-for-go/wiki/Versioning)*

You'll need to consider both SDK and API versions when selecting your
targets. The following will describe them one by one.

**SDK versions** are set with repository-wide [tags][]. We try to adhere to
[semver][], so although we've long moved past version zero, we continue to
add `-beta` to versions because **this package is still in preview.**

[tags]: https://github.com/Azure/azure-sdk-for-go/tags
[semver]: https://semver.org

Azure **API versions** are typically a date string of form
`yyyy-mm-dd[-preview|-beta]`. Whatever SDK version you use, you must also
specify an API version in your import path, for example:

```go
import "github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2016-03-01/compute"
import "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2017-09-01/network"
```

For a list of all available services and versions see [godoc][services_godoc]
or start with `find ./services -type d -mindepth 3` within this repo. 

Azure **API profiles** specify a subset of APIs and versions offered in
specific Azure regions and Azure Stack. The 2017-03-09 profile is intended for
use with Azure Stack and includes compatible Compute, Network, Storage and
Group management APIs. You can use it as follows:

```go
import "github.com/Azure/azure-sdk-for-go/profiles/2017-03-09/compute/mgmt/compute"
import "github.com/Azure/azure-sdk-for-go/profiles/2017-03-09/network/mgmt/network"
import "github.com/Azure/azure-sdk-for-go/profiles/2017-03-09/resources/mgmt/..."
import "github.com/Azure/azure-sdk-for-go/profiles/2017-03-09/storage/mgmt/storage"
```

We also provide two special profiles: `latest` and `preview`. These will always
refer to the most recent stable or preview API versions for each service. For
example, to automatically use the most recent Compute APIs, use one of the following
imports:

```go
import "github.com/Azure/azure-sdk-for-go/profiles/latest/compute/mgmt/compute"
import "github.com/Azure/azure-sdk-for-go/profiles/preview/compute/mgmt/compute"
```

## Resources

- SDK docs are at [godoc.org](https://godoc.org/github.com/Azure/azure-sdk-for-go/).
- SDK samples are at [Azure-Samples/azure-sdk-for-go-samples](https://github.com/Azure-Samples/azure-sdk-for-go-samples).
- SDK notifications are published via the [Azure update feed][].
- Azure API docs are at [docs.microsoft.com/rest/api](https://docs.microsoft.com/rest/api/).
- General Azure docs are at [docs.microsoft.com/azure](https://docs.microsoft.com/azure).

## License

Apache 2.0, see [LICENSE][].

## Contribute

See [CONTRIBUTING.md][]. 

[services_dir]: https://github.com/Azure/azure-sdk-for-go/tree/master/services
[samples_repo]: https://github.com/Azure-Samples/azure-sdk-for-go-samples
[Azure update feed]: https://azure.microsoft.com/updates/
[LICENSE]: ./LICENSE
[CONTRIBUTING.md]: ./CONTRIBUTING.md
[services_godoc]: https://godoc.org/github.com/Azure/azure-sdk-for-go/services
