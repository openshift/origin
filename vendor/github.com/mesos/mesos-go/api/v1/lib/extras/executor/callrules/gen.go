package callrules

//go:generate go run ../../gen/rules.go ../../gen/gen.go -import github.com/mesos/mesos-go/api/v1/lib -import github.com/mesos/mesos-go/api/v1/lib/executor -type E:*executor.Call:&executor.Call{} -type Z:mesos.Response:&mesos.ResponseWrapper{}

//go:generate go run ../../gen/rule_callers.go ../../gen/gen.go -import github.com/mesos/mesos-go/api/v1/lib/executor -import github.com/mesos/mesos-go/api/v1/lib/executor/calls -type E:*executor.Call -type C:calls.Caller -type CF:calls.CallerFunc -output callers_generated.go

//go:generate go run ../../gen/rule_metrics.go ../../gen/gen.go -import github.com/mesos/mesos-go/api/v1/lib -import github.com/mesos/mesos-go/api/v1/lib/executor -type E:*executor.Call:&executor.Call{} -type ET:executor.Call_Type -type Z:mesos.Response:&mesos.ResponseWrapper{} -output metrics_generated.go
