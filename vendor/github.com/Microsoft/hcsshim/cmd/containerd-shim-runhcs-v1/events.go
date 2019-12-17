package main

import (
	"bytes"
	"os/exec"
	"sync"

	"github.com/containerd/typeurl"
	"github.com/sirupsen/logrus"
)

type publisher func(topic string, event interface{})

var _ = (publisher)(publishEvent)

var publishLock sync.Mutex

func publishEvent(topic string, event interface{}) {
	publishLock.Lock()
	defer publishLock.Unlock()

	encoded, err := typeurl.MarshalAny(event)
	if err != nil {
		logrus.WithError(err).Error("publishEvent - Failed to encode event")
		return
	}
	data, err := encoded.Marshal()
	if err != nil {
		logrus.WithError(err).Error("publishEvent - Failed to marshal event")
		return
	}
	cmd := exec.Command(containerdBinaryFlag, "--address", addressFlag, "publish", "--topic", topic, "--namespace", namespaceFlag)
	cmd.Stdin = bytes.NewReader(data)
	err = cmd.Run()
	if err != nil {
		logrus.WithError(err).Error("publishEvent - Failed to publish event")
	}
}
