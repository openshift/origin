package eventrules

//go:generate go run ../../gen/rules.go ../../gen/gen.go -import github.com/mesos/mesos-go/api/v1/lib/executor -type E:*executor.Event:&executor.Event{}

//go:generate go run ../../gen/rule_handlers.go ../../gen/gen.go -import github.com/mesos/mesos-go/api/v1/lib/executor -import github.com/mesos/mesos-go/api/v1/lib/executor/events -type E:*executor.Event -type H:events.Handler -type HF:events.HandlerFunc -output handlers_generated.go

//go:generate go run ../../gen/rule_metrics.go ../../gen/gen.go -import github.com/mesos/mesos-go/api/v1/lib/executor -type E:*executor.Event:&executor.Event{} -type ET:executor.Event_Type -output metrics_generated.go
