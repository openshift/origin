package main

import (
	"math/rand"
	"os"
	"time"

	"k8s.io/apiserver/pkg/util/logs"

	"github.com/openshift/origin/pkg/cmd/flagtypes"
	"github.com/openshift/origin/pkg/cmd/server/start/network"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	rand.Seed(time.Now().UTC().UnixNano())

	cmd, _ := network.NewCommandStartNetwork("openshift-sdn", os.Stdout, os.Stderr)
	flagtypes.GLog(cmd.PersistentFlags())

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
