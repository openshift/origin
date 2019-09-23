package calls

// callers.go APIs are deprecated in favor of sender.go APIs
//go:generate go run ../../extras/gen/callers.go ../../extras/gen/gen.go -import github.com/mesos/mesos-go/api/v1/lib/executor -type C:*executor.Call -output calls_caller_generated.go

//go:generate go run ../../extras/gen/sender.go ../../extras/gen/gen.go -import github.com/mesos/mesos-go/api/v1/lib/executor -type C:executor.Call -type O:executor.CallOpt -output calls_sender_generated.go
