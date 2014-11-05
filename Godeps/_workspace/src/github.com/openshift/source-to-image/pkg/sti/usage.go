package sti

// UsageHandler handles a request to display usage
type usageHandler interface {
	cleanup()
	setup(required []string, optional []string) error
	execute(command string) error
}

// Usage display usage information about a particular build image
type Usage struct {
	handler usageHandler
}

// NewUsage creates a new instance of the default Usage implementation
func NewUsage(req *STIRequest) (*Usage, error) {
	if h, err := newRequestHandler(req); err != nil {
		return nil, err
	} else {
		return &Usage{handler: h}, nil
	}
}

// Show starts the builder container and invokes the usage script on it
// to print usage information for the script.
func (u *Usage) Show() error {
	h := u.handler
	defer h.cleanup()

	if err := h.setup([]string{"usage"}, []string{}); err != nil {
		return err
	}

	return h.execute("usage")
}
