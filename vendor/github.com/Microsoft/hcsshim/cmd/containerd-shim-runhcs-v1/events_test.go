package main

var _ = (publisher)(fakePublisher)

func fakePublisher(topic string, event interface{}) {
	// Do nothing
}
