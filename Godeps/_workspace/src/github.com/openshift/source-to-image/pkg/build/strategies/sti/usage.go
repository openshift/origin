package sti

import (
	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/build"
)

// UsageHandler handles a request to display usage
type usageHandler interface {
	build.ScriptsHandler
	build.Preparer
	SetScripts([]string, []string)
}

// Usage display usage information about a particular build image
type Usage struct {
	handler usageHandler
	garbage build.Cleaner
	request *api.Request
}

// NewUsage creates a new instance of the default Usage implementation
func NewUsage(req *api.Request) (*Usage, error) {
	b, err := New(req)
	if err != nil {
		return nil, err
	}
	usage := Usage{
		handler: b,
		request: req,
		garbage: b.garbage,
	}
	return &usage, nil
}

// Show starts the builder container and invokes the usage script on it
// to print usage information for the script.
func (u *Usage) Show() error {
	b := u.handler
	defer u.garbage.Cleanup(u.request)

	b.SetScripts([]string{api.Usage}, []string{})

	if err := b.Prepare(u.request); err != nil {
		return err
	}

	return b.Execute(api.Usage, u.request)
}
