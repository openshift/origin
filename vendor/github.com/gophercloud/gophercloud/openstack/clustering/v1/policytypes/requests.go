package policytypes

import (
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/pagination"
)

// List makes a request against the API to list policy types.
func List(client *gophercloud.ServiceClient) pagination.Pager {
	url := policyTypeListURL(client)
	return pagination.NewPager(client, url, func(r pagination.PageResult) pagination.Page {
		return PolicyTypePage{pagination.SinglePageBase(r)}
	})
}
