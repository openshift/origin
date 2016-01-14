package partitioningbucketer

import (
	"fmt"
	"testing"
)

func TestSimpleCase(t *testing.T) {
	stop := make(chan struct{})

	pb := NewPartitioningBucketer(10, NamespaceKeyFunc, stop)

	numIterations := 10

	responseChannel := make(chan interface{}, 50)
	go func() {
		for i := 0; i < numIterations; i++ {
			fmt.Printf("HERE!\n")
			pb.AddWork(fmt.Sprintf("foo-%d", i), responseChannel)
		}
	}()

	loggingWorker := func(b *Bucket) {
		for {
			if len(b.IncomingWorkChannel) == 0 {
				break
			}
			workItem := <-b.IncomingWorkChannel
			fmt.Printf("got %#v\n", workItem)
			workItem.ResponseChannel <- fmt.Sprintf("done with %v!", workItem.Work)
		}
	}
	go pb.DoWork(loggingWorker)

	for i := 0; i < numIterations; i++ {
		select {
		case value := <-responseChannel:
			fmt.Printf("received %v\n", value)
		}
	}
}
