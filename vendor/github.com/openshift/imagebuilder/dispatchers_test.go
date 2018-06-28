package imagebuilder

import (
	"reflect"
	"testing"

	docker "github.com/fsouza/go-dockerclient"
)

func TestDispatchCopy(t *testing.T) {
	mybuilder := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "alpine",
		},
	}
	args := []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager", "."}
	flagArgs := []string{"--from=builder"}
	original := "COPY --from=builder /go/src/github.com/kubernetes-incubator/service-catalog/controller-manager ."
	if err := dispatchCopy(&mybuilder, args, nil, flagArgs, original); err != nil {
		t.Errorf("dispatchCopy error: %v", err)
	}
	expectedPendingCopies := []Copy{
		{
			From:     "builder",
			Src:      []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager"},
			Dest:     "/root/", // destination must contain a trailing slash
			Download: false,
		},
	}
	if !reflect.DeepEqual(mybuilder.PendingCopies, expectedPendingCopies) {
		t.Errorf("Expected %v, got %v\n", expectedPendingCopies, mybuilder.PendingCopies)
	}
}
