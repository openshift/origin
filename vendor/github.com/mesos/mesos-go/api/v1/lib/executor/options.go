package executor

// A CallOpt is a functional option type for Calls.
type CallOpt func(*Call)

// With applies the given CallOpts to the receiving Call, returning it.
func (c *Call) With(opts ...CallOpt) *Call {
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type CallOptions []CallOpt

// Copy returns a cloned copy of CallOptions
func (co CallOptions) Copy() CallOptions {
	if len(co) == 0 {
		return nil
	}
	x := make(CallOptions, len(co))
	copy(x, co)
	return x
}
