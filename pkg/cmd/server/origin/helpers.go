package origin

import (
	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api/rest"
)

// restInPeace returns the given storage if the error is nil, or fatals
func restInPeace(s rest.StandardStorage, err error) rest.StandardStorage {
	if err != nil {
		glog.Fatal(err)
	}
	return s
}

func updateInPeace(s rest.Updater, err error) rest.Updater {
	if err != nil {
		glog.Fatal(err)
	}
	return s
}
