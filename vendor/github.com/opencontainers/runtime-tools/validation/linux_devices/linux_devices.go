package main

import (
	"os"

	rspecs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/validation/util"
)

func main() {
	g, err := util.GetDefaultGenerator()
	if err != nil {
		util.Fatal(err)
	}

	// add char device
	cdev := rspecs.LinuxDevice{}
	cdev.Path = "/dev/test1"
	cdev.Type = "c"
	cdev.Major = 10
	cdev.Minor = 666
	cmode := os.FileMode(int32(432))
	cdev.FileMode = &cmode
	cuid := uint32(0)
	cdev.UID = &cuid
	cgid := uint32(0)
	cdev.GID = &cgid
	g.AddDevice(cdev)

	// add block device
	bdev := rspecs.LinuxDevice{}
	bdev.Path = "/dev/test2"
	bdev.Type = "b"
	bdev.Major = 8
	bdev.Minor = 666
	bmode := os.FileMode(int32(432))
	bdev.FileMode = &bmode
	uid := uint32(0)
	bdev.UID = &uid
	gid := uint32(0)
	bdev.GID = &gid
	g.AddDevice(bdev)

	// add fifo device
	pdev := rspecs.LinuxDevice{}
	pdev.Path = "/dev/test3"
	pdev.Type = "p"
	pdev.Major = 8
	pdev.Minor = 666
	pmode := os.FileMode(int32(432))
	pdev.FileMode = &pmode
	g.AddDevice(pdev)

	err = util.RuntimeInsideValidate(g, nil, nil)
	if err != nil {
		util.Fatal(err)
	}
}
