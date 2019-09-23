package scheduler

// A CallOpt is a functional option type for Calls.
type CallOpt func(*Call)

// With applies the given CallOpts to the receiving Call, returning it.
func (c *Call) With(opts ...CallOpt) *Call {
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c
}

// ReconcileOpt is a functional option type for Call_Reconcile
type ReconcileOpt func(*Call_Reconcile)

// With applies the given ReconcileOpt's to the receiving Call_Reconcile, returning it.
func (r *Call_Reconcile) With(opts ...ReconcileOpt) *Call_Reconcile {
	for _, opt := range opts {
		if opt != nil {
			opt(r)
		}
	}
	return r
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
