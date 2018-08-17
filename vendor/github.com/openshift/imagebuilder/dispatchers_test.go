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
			Chown:    "",
		},
	}
	if !reflect.DeepEqual(mybuilder.PendingCopies, expectedPendingCopies) {
		t.Errorf("Expected %v, got %v\n", expectedPendingCopies, mybuilder.PendingCopies)
	}
}

func TestDispatchCopyChown(t *testing.T) {
	mybuilder := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "busybox",
		},
	}

	mybuilder2 := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "alpine",
		},
	}

	// Test Bad chown values
	args := []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager", "."}
	flagArgs := []string{"--chown=1376:1376"}
	original := "COPY --chown=1376:1376 /go/src/github.com/kubernetes-incubator/service-catalog/controller-manager ."
	if err := dispatchCopy(&mybuilder, args, nil, flagArgs, original); err != nil {
		t.Errorf("dispatchCopy error: %v", err)
	}
	expectedPendingCopies := []Copy{
		{
			From:     "",
			Src:      []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager"},
			Dest:     "/root/", // destination must contain a trailing slash
			Download: false,
			Chown:    "6731:6731",
		},
	}
	if reflect.DeepEqual(mybuilder.PendingCopies, expectedPendingCopies) {
		t.Errorf("Expected %v, to not match %v\n", expectedPendingCopies, mybuilder.PendingCopies)
	}

	// Test Good chown values
	flagArgs = []string{"--chown=6731:6731"}
	original = "COPY --chown=6731:6731 /go/src/github.com/kubernetes-incubator/service-catalog/controller-manager ."
	if err := dispatchCopy(&mybuilder2, args, nil, flagArgs, original); err != nil {
		t.Errorf("dispatchCopy error: %v", err)
	}
	expectedPendingCopies = []Copy{
		{
			From:     "",
			Src:      []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager"},
			Dest:     "/root/", // destination must contain a trailing slash
			Download: false,
			Chown:    "6731:6731",
		},
	}
	if !reflect.DeepEqual(mybuilder2.PendingCopies, expectedPendingCopies) {
		t.Errorf("Expected %v, to match %v\n", expectedPendingCopies, mybuilder2.PendingCopies)
	}
}
