package eventrules

//go:generate go run ../../gen/rules.go ../../gen/gen.go -import github.com/mesos/mesos-go/api/v1/lib/scheduler -type E:*scheduler.Event:&scheduler.Event{}

//go:generate go run ../../gen/rule_handlers.go ../../gen/gen.go -import github.com/mesos/mesos-go/api/v1/lib/scheduler -import github.com/mesos/mesos-go/api/v1/lib/scheduler/events -type E:*scheduler.Event -type H:events.Handler -type HF:events.HandlerFunc -output handlers_generated.go

//go:generate go run ../../gen/rule_metrics.go ../../gen/gen.go -import github.com/mesos/mesos-go/api/v1/lib/scheduler -type E:*scheduler.Event:&scheduler.Event{} -type ET:scheduler.Event_Type -output metrics_generated.go
