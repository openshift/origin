package test

// FakeCallbackInvoker provides the fake callback invoker
type FakeCallbackInvoker struct {
	CallbackURL string
	Success     bool
	Messages    []string
	Result      []string
}

// ExecuteCallback executes the fake callback
func (f *FakeCallbackInvoker) ExecuteCallback(callbackURL string, success bool, messages []string) []string {
	f.CallbackURL = callbackURL
	f.Success = success
	f.Messages = messages
	return f.Result
}
