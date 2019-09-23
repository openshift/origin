package app

import (
	"log"
	"time"

	xmetrics "github.com/mesos/mesos-go/api/v1/lib/extras/metrics"
)

func forever(name string, jobRestartDelay time.Duration, counter xmetrics.Counter, f func() error) {
	for {
		counter(name)
		err := f()
		if err != nil {
			log.Printf("job %q exited with error %+v", name, err)
		} else {
			log.Printf("job %q exited", name)
		}
		time.Sleep(jobRestartDelay)
	}
}
