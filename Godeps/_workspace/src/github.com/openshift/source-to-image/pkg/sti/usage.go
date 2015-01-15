package sti

import (
	"github.com/openshift/source-to-image/pkg/sti/api"
)

// UsageHandler handles a request to display usage
type usageHandler interface {
	cleanup()
	setup(required []api.Script, optional []api.Script) error
	execute(command api.Script) error
}

// Usage display usage information about a particular build image
type Usage struct {
	handler usageHandler
}

// NewUsage creates a new instance of the default Usage implementation
func NewUsage(req *api.Request) (*Usage, error) {
	h, err := newRequestHandler(req)
	if err != nil {
		return nil, err
	}
	return &Usage{handler: h}, nil
}

// Show starts the builder container and invokes the usage script on it
// to print usage information for the script.
func (u *Usage) Show() error {
	h := u.handler
	defer h.cleanup()

	if err := h.setup([]api.Script{api.Usage}, []api.Script{}); err != nil {
		return err
	}

	return h.execute(api.Usage)
}
