package admission

var (
	// FactoryFilterFn allows the injection of a global filter on all admission factory function.  This allows
	// us to inject a filtering function for things like config rewriting just before construction.
	FactoryFilterFn func(Factory) Factory = func(delegate Factory) Factory {
		return delegate
	}
)
