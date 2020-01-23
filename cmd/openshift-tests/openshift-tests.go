package main

import (
	"os"
	"syscall"

	"k8s.io/klog"
)

func main() {
	if err := syscall.Exec("/usr/bin/openshift-tests-kubernetes", os.Args, os.Environ()); err != nil {
		klog.Fatal(err)
	}
}
