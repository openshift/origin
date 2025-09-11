package ginkgo

type retryStrategyFlag struct {
	strategy *RetryStrategy
}

func newRetryStrategyFlag(strategy *RetryStrategy) *retryStrategyFlag {
	return &retryStrategyFlag{strategy: strategy}
}

func (r *retryStrategyFlag) String() string {
	if r.strategy == nil || *r.strategy == nil {
		return defaultRetryStrategy
	}
	return (*r.strategy).Name()
}

func (r *retryStrategyFlag) Set(value string) error {
	strategy, err := createRetryStrategy(value)
	if err != nil {
		return err
	}
	*r.strategy = strategy
	return nil
}

func (r *retryStrategyFlag) Type() string {
	return "string"
}
