package httpmaster

//go:generate go run ../../extras/gen/httpsender.go ../../extras/gen/gen.go -import github.com/mesos/mesos-go/api/v1/lib/master -import github.com/mesos/mesos-go/api/v1/lib/master/calls -type C:master.Call:master.Call{Type:master.Call_GET_METRICS}
