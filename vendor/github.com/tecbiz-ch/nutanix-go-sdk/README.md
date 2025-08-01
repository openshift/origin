# nutanix-go-sdk: A Go library for the Nutanix Prism API

[![GitHub Actions status](https://github.com/tecbiz-ch/nutanix-go-sdk/workflows/Continuous%20Integration/badge.svg)](https://github.com/tecbiz-ch/nutanix-go-sdk/actions)
[![GoDoc](https://godoc.org/github.com/tecbiz-ch/nutanix-go-sdk?status.svg)](https://pkg.go.dev/github.com/tecbiz-ch/nutanix-go-sdk)

Package nutanix is a library for the Nutanix Prism API

The library’s documentation is available at [GoDoc](https://pkg.go.dev/github.com/tecbiz-ch/nutanix-go-sdk),
the public API documentation is available at [www.nutanix.dev](https://www.nutanix.dev/reference/prism_central/v3/).

## Example

```go
package main

import (
    "context"
    "fmt"
    "log"

    nutanix "github.com/tecbiz-ch/nutanix-go-sdk"
)

func main() {

	configCreds := nutanix.Credentials{
		Username: "admin",
		Password: "password",
	}

	opts := []nutanix.ClientOption{
		nutanix.WithCredentials(&configCreds),
		nutanix.WithEndpoint("https://PC"),
		nutanix.WithInsecure(), // Allow insecure
	}

	client := nutanix.NewClient(opts...)

        ctx := context.Background()
	mycluster, err = client.Cluster.Get(ctx, "mycluster")

        list, err = client.VM.All(ctx)

}
```

## License

MIT license
