package cmd

import (
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
)

func usageError(cmd *cobra.Command, format string, args ...interface{}) {
	glog.Errorf(format, args...)
	glog.Errorf("See '%s -h' for help.", cmd.CommandPath())
	os.Exit(1)
}

func checkErr(err error) {
	if err != nil {
		glog.FatalDepth(1, err)
	}
}
