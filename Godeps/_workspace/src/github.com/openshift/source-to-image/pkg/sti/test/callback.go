package test

type FakeCallbackInvoker struct {
	CallbackUrl string
	Success     bool
	Messages    []string
	Result      []string
}

func (f *FakeCallbackInvoker) ExecuteCallback(callbackUrl string, success bool, messages []string) []string {
	f.CallbackUrl = callbackUrl
	f.Success = success
	f.Messages = messages
	return f.Result
}
