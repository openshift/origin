package httpagent

//go:generate go run ../../extras/gen/httpsender.go ../../extras/gen/gen.go -import github.com/mesos/mesos-go/api/v1/lib/agent -import github.com/mesos/mesos-go/api/v1/lib/agent/calls -type C:agent.Call:agent.Call{Type:agent.Call_GET_METRICS}
