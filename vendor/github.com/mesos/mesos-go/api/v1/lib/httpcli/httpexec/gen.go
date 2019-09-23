package httpexec

//go:generate go run ../../extras/gen/httpsender.go ../../extras/gen/gen.go -import github.com/mesos/mesos-go/api/v1/lib/executor -import github.com/mesos/mesos-go/api/v1/lib/executor/calls -type C:executor.Call:executor.Call{Type:executor.Call_MESSAGE}
