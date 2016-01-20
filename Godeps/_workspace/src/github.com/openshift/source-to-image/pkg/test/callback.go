package test

// FakeCallbackInvoker provides the fake callback invoker
type FakeCallbackInvoker struct {
	CallbackURL string
	Success     bool
	Messages    []string
	Labels      map[string]string
	Result      []string
}

// ExecuteCallback executes the fake callback
func (f *FakeCallbackInvoker) ExecuteCallback(callbackURL string, success bool, labels map[string]string, messages []string) []string {
	f.CallbackURL = callbackURL
	f.Success = success
	f.Labels = labels
	f.Messages = messages
	return f.Result
}
